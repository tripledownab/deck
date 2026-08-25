package agent

// What a runner reports about itself: whether it is working, how long it has
// been quiet, and how it ended.

import "time"

// Status is what the sidebar dot reports for a session.
type Status int

func (s Status) String() string {
	switch s {
	case Working:
		return "Working"
	case Exited:
		return "Exited"
	default:
		return "Idle"
	}
}

// activityWindow is how long after the last byte of output a session still
// counts as Working.
//
// This is an activity heuristic, not a protocol signal. A PTY gives us bytes
// and nothing else, so "the agent is thinking" is inferred from the fact that
// it keeps redrawing its spinner. It is wrong exactly when an agent thinks
// quietly.
//
// The fix is Claude Code's Stop and Notification hooks, which report real turn
// boundaries in an interactive session — see docs/backlog.md. Tuning this
// constant only moves where the guess is wrong.
const activityWindow = 900 * time.Millisecond

// Status reports what the sidebar should show. See activityWindow.
func (r *Runner) Status() Status {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.exited {
		return Exited
	}
	if time.Since(r.lastWrite) < activityWindow {
		return Working
	}
	return Idle
}

// Quiet is how long since the agent last printed anything.
//
// Status answers the same question against one fixed threshold. This exposes
// the raw duration because a caller weighing the PTY against something else
// needs its own threshold: the UI trusts a hook's "working" report over a
// short silence, and stops trusting it over a long one.
func (r *Runner) Quiet() time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	// lastWrite stays zero until the first byte arrives, and time since the
	// zero value is two thousand years. An agent that has printed nothing yet
	// has been quiet since it started, not since the year zero — Status is
	// unaffected because its comparison goes the other way, but a caller
	// asking "how long" would read a launching agent as long dead.
	since := r.lastWrite
	if since.IsZero() {
		since = r.started
	}
	return time.Since(since)
}

// Err returns the process exit error once it has exited, else nil.
func (r *Runner) Err() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.exitErr
}

// Uptime is how long the process has been running.
func (r *Runner) Uptime() time.Duration { return time.Since(r.started) }

// Dir is the directory the agent runs in.
func (r *Runner) Dir() string { return r.cfg.Dir }
