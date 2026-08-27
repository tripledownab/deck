package coord

// Reported session state, from Claude Code hooks rather than guessed from PTY
// output.

import (
	"fmt"
	"strings"
	"sync"
)

// State is what an agent last told us it was doing.
//
// This is the honest version of the sidebar dot. The PTY heuristic in
// internal/agent infers "working" from bytes arriving, which is wrong exactly
// when an agent thinks quietly — the moment you are most likely to be watching
// it. Hooks report turn boundaries instead of implying them.
type State int

const (
	// StateUnknown means no hook has reported yet, so the caller should fall
	// back to the activity heuristic.
	StateUnknown State = iota
	// StateWorking: a turn started and has not finished.
	StateWorking
	// StateWaiting: the agent wants a human — a permission prompt, a question,
	// or a turn that died on an API error.
	StateWaiting
	// StateIdle: the turn finished.
	StateIdle
)

func (s State) String() string {
	switch s {
	case StateWorking:
		return "Working"
	case StateWaiting:
		return "Waiting"
	case StateIdle:
		return "Idle"
	default:
		return "Unknown"
	}
}

// hookPaths map the last URL segment a hook posts to onto a state.
//
// The meaning is carried by the URL, not by the request body. Deck writes
// the hooks config, so it chooses a distinct endpoint per event and never has
// to parse the payload — which keeps this working whether or not the body's
// field names are what we expect, and means a Notification subtype is
// distinguished by its matcher rather than by reading an undocumented field.
var hookPaths = map[string]State{
	"working": StateWorking,
	"waiting": StateWaiting,
	"idle":    StateIdle,
}

// statusBoard holds the last reported state per session.
type statusBoard struct {
	mu      sync.Mutex
	reports map[string]State
}

func newStatusBoard() *statusBoard {
	return &statusBoard{reports: map[string]State{}}
}

func (b *statusBoard) set(sessionID string, s State) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.reports[sessionID] = s
}

func (b *statusBoard) get(sessionID string) (State, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	s, ok := b.reports[sessionID]
	return s, ok
}

func (b *statusBoard) clear(sessionID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.reports, sessionID)
}

// Report records a state as if a hook had posted it.
//
// It exists for tests in other packages: ui.TestStatusOfPrefersTruthOverGuess
// has to put a session into "waiting" without a real agent to raise a
// permission prompt, and the hook endpoint that would otherwise do it is
// unexported. This was briefly deleted for having no consumer — the consumer
// was a test that had not been written yet.
func (c *Coordinator) Report(sessionID string, s State) {
	c.status.set(sessionID, s)
}

// StateOf returns the last reported state, and whether anything was reported.
// A false second value means no hook has fired yet — the caller should fall
// back to whatever it used before.
func (c *Coordinator) StateOf(sessionID string) (State, bool) {
	return c.status.get(sessionID)
}

// hookTimeoutSeconds bounds how long claude waits for our reply.
//
// The documented default is 600. That is disastrous for a status ping: if
// Deck is gone or wedged, every turn boundary would stall the agent for
// ten minutes. Five seconds is far more than a local POST needs and caps the
// damage when the far end is not there.
const hookTimeoutSeconds = 5

// waitingNotifications are the notification_type values that mean a human is
// wanted. Notification's matcher filters on that field, so each needs its own
// entry.
//
// The criterion is "nobody can make progress until a person acts", not any
// fixed count: a permission dialog, the idle nudge, an agent asking for input,
// and either shape of MCP elicitation all block on an answer. The other
// documented types — auth_success, elicitation_complete, elicitation_response,
// agent_completed — report something that already happened, so they say
// nothing about whether the agent is stuck.
var waitingNotifications = []string{
	"permission_prompt",
	"idle_prompt",
	"agent_needs_input",
	"elicitation_dialog",
	"elicitation_url_dialog",
}

// HooksConfigJSON is the --settings payload that wires one session's agent to
// report its turn boundaries.
//
// Five events, because three do not cover a turn:
//
//   - UserPromptSubmit opens a turn.
//   - PostToolBatch reopens it. Without this the "needs you" dot sticks for
//     the rest of the turn, because the grant itself is not what we observe —
//     the batch resolving is. The agent would run on for minutes still
//     flagged as blocked, which is the false positive that teaches you to
//     ignore the dot. It closes late rather than fully: a long approved tool
//     call still reads "needs you" while it runs.
//   - Stop closes a turn that finished.
//   - StopFailure closes one that ended in failure rather than normally.
//     Without it such a session reads "Working" until the next prompt. It
//     reports "waiting" rather than "idle" because a turn cut short that way
//     does want a human.
//   - Notification carries the subtypes that mean a human is wanted, each as
//     its own matcher entry.
//
// Stop and StopFailure take no matcher.
//
// PreToolUse is the tempting sixth, for the residual gap above. Whether it
// helps depends on whether it fires before or after the permission dialog,
// which is unverified — before, and it changes nothing, because Notification
// sets "waiting" straight after it. See docs/backlog.md rather than adding it
// on the assumption.
func (c *Coordinator) HooksConfigJSON(sessionID string) string {
	handler := func(kind string) string {
		return fmt.Sprintf(`{"type":"http","url":%q,"timeout":%d}`,
			c.server.hookURL(sessionID, kind), hookTimeoutSeconds)
	}
	entry := func(kind string) string {
		return fmt.Sprintf(`{"hooks":[%s]}`, handler(kind))
	}
	matched := func(matcher, kind string) string {
		return fmt.Sprintf(`{"matcher":%q,"hooks":[%s]}`, matcher, handler(kind))
	}

	notifications := make([]string, 0, len(waitingNotifications))
	for _, n := range waitingNotifications {
		notifications = append(notifications, matched(n, "waiting"))
	}

	return fmt.Sprintf(
		`{"hooks":{"UserPromptSubmit":[%s],"PostToolBatch":[%s],"Stop":[%s],"StopFailure":[%s],"Notification":[%s]}}`,
		entry("working"),
		entry("working"),
		entry("idle"),
		entry("waiting"),
		strings.Join(notifications, ","),
	)
}
