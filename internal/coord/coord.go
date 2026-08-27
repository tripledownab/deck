// Package coord lets the agents working on one project see and coordinate with
// each other.
//
// Sessions are deliberately isolated — separate git worktrees, separate claude
// transcripts — which is what makes running several of them safe. The cost is
// that they are blind to each other by construction, so two agents will
// happily refactor the same interface in parallel and only discover it at
// merge time. This package is the one channel through that isolation.
//
// It is exposed to agents as an MCP server (see mcp.go). Deck itself never
// speaks the agent's protocol; it just runs a server the agent can call.
package coord

import (
	"context"
	"fmt"
	"github.com/tripledownab/deck/internal/agent"
	"os"
	"sync"
)

// Session is one agent run, as the coordinator sees it.
type Session struct {
	ID        string
	ProjectID string
	Name      string // scheming-hawk-jhgk
	Title     string
	Branch    string
	Dir       string // worktree or project directory

	// Isolated and BaseRef are what the work tool needs. A session sharing the
	// project directory has no branch of its own, so its changes cannot be
	// told apart from anyone else's, and BaseRef is the commit its worktree
	// started at.
	Isolated bool
	BaseRef  string
}

// Coordinator holds the live picture and serves it over MCP.
type Coordinator struct {
	mu       sync.Mutex
	sessions map[string]Session   // by session id
	claims   map[string][]Claim   // by session id
	inbox    map[string][]Message // by recipient session id

	// noteLines counts what is on disk per project, so compaction does not
	// re-read the file on every append.
	noteLines map[string]int

	// status holds what each session's hooks last reported. Separate from the
	// sessions map because a report can arrive before or after a session is
	// registered, and losing one to a race would leave a stale dot.
	status *statusBoard

	// jobs are spawned analyses, and spend is what each session's have cost
	// it. Both are dropped with the session, like claims and the inbox: a
	// review belongs to the session that paid for it.
	jobs  map[string]*Job
	spend map[string]float64

	// spawn performs one headless turn. A field rather than a direct call
	// because otherwise `go test` invokes the paid CLI on every run, and CI —
	// where claude is not installed — reports green on a spawn that failed.
	spawn Reviewer

	// life bounds everything the coordinator started. Close cancels it, so a
	// spawned analysis cannot outlive the app that asked for it: without this
	// quitting Deck mid-review left a claude process running, and billing,
	// with no surface left to show it on.
	life      context.Context
	endOfLife context.CancelFunc

	notesDir string
	server   *server
}

// Reviewer performs one spawned analysis. Deck uses claude; a caller may
// substitute another, and the tests do so the suite neither spends money nor
// needs the CLI installed.
type Reviewer func(ctx context.Context, dir, prompt string) (agent.ClaudeRun, error)

// Option configures a Coordinator at startup.
type Option func(*Coordinator)

// WithReviewer replaces who performs a spawned analysis.
func WithReviewer(r Reviewer) Option {
	return func(c *Coordinator) { c.spawn = r }
}

// Start brings up the coordinator and its MCP endpoint on localhost.
func Start(notesDir string, opts ...Option) (*Coordinator, error) {
	if err := os.MkdirAll(notesDir, 0o755); err != nil {
		return nil, fmt.Errorf("create notes dir: %w", err)
	}
	life, endOfLife := context.WithCancel(context.Background())
	c := &Coordinator{
		life:      life,
		endOfLife: endOfLife,
		sessions:  map[string]Session{},
		claims:    map[string][]Claim{},
		inbox:     map[string][]Message{},
		noteLines: map[string]int{},
		jobs:      map[string]*Job{},
		spend:     map[string]float64{},
		spawn: func(ctx context.Context, dir, prompt string) (agent.ClaudeRun, error) {
			return agent.RunClaude(ctx, dir, prompt, "--permission-mode", "plan")
		},
		status:   newStatusBoard(),
		notesDir: notesDir,
	}
	// Options after the defaults, so a caller replaces rather than races them.
	for _, opt := range opts {
		opt(c)
	}
	srv, err := newServer(c)
	if err != nil {
		return nil, err
	}
	c.server = srv
	return c, nil
}

// Close stops the MCP endpoint.
// Close stops the endpoint and everything the coordinator started.
//
// Cancelling first: a spawned analysis holds no lock and needs none to stop,
// and closing the listener first would leave those runs alive for as long as
// the shutdown takes.
func (c *Coordinator) Close() error {
	if c.endOfLife != nil {
		c.endOfLife()
	}
	if c.server == nil {
		return nil
	}
	return c.server.close()
}

// Register adds or updates a live session.
func (c *Coordinator) Register(s Session) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sessions[s.ID] = s
}

// Registered lists the session ids the coordinator currently knows about, so
// the caller can reconcile them against the processes that are actually alive.
func (c *Coordinator) Registered() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	ids := make([]string, 0, len(c.sessions))
	for id := range c.sessions {
		ids = append(ids, id)
	}
	return ids
}

// Unregister drops a session and every claim it held.
//
// Releasing on exit is why claims are in memory and not on disk: a claim held
// by a process that is gone is worse than no claim at all, because the next
// agent believes someone is working there.
func (c *Coordinator) Unregister(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.sessions, id)
	delete(c.claims, id)
	delete(c.inbox, id)
	delete(c.spend, id)
	for jid, j := range c.jobs {
		if j.From == id {
			delete(c.jobs, jid)
		}
	}
	c.status.clear(id)
}

// MCPConfigJSON is the inline --mcp-config that points one session's agent at
// its own endpoint. The session id is in the URL path, so the server knows who
// is calling without the agent having to prove it.
func (c *Coordinator) MCPConfigJSON(sessionID string) string {
	return fmt.Sprintf(`{"mcpServers":{"%s":{"type":"http","url":"%s"}}}`,
		serverName, c.server.urlFor(sessionID))
}
