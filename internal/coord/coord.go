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
	"fmt"
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

	notesDir string
	server   *server
}

// Start brings up the coordinator and its MCP endpoint on localhost.
func Start(notesDir string) (*Coordinator, error) {
	if err := os.MkdirAll(notesDir, 0o755); err != nil {
		return nil, fmt.Errorf("create notes dir: %w", err)
	}
	c := &Coordinator{
		sessions:  map[string]Session{},
		claims:    map[string][]Claim{},
		inbox:     map[string][]Message{},
		noteLines: map[string]int{},
		status:    newStatusBoard(),
		notesDir:  notesDir,
	}
	srv, err := newServer(c)
	if err != nil {
		return nil, err
	}
	c.server = srv
	return c, nil
}

// Close stops the MCP endpoint.
func (c *Coordinator) Close() error {
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
	c.status.clear(id)
}

// MCPConfigJSON is the inline --mcp-config that points one session's agent at
// its own endpoint. The session id is in the URL path, so the server knows who
// is calling without the agent having to prove it.
func (c *Coordinator) MCPConfigJSON(sessionID string) string {
	return fmt.Sprintf(`{"mcpServers":{"%s":{"type":"http","url":"%s"}}}`,
		serverName, c.server.urlFor(sessionID))
}
