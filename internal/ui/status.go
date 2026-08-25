package ui

// What the sidebar dot says, and how much a reported state is trusted.

import (
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/tripledownab/deck/internal/agent"
	"github.com/tripledownab/deck/internal/coord"
	"github.com/tripledownab/deck/internal/store"
)

// staleWorkingReport is how long a "working" report survives a silent pane.
//
// Only "working" can get stuck, because it is the only state that needs a
// later event to clear it, and three separate events turned out to be missing:
// an API error fires StopFailure rather than Stop, granting a permission fires
// nothing, and esc fires nothing either. Two were fixable by registering the
// right hook. The interrupt is not — claude has no event for it — so the pane
// is the backstop.
//
// Ten seconds because claude's own spinner repaints several times a second
// while a turn runs, so a pane this quiet is a turn that ended without saying
// so. Measured rather than guessed: the longest silence inside a real turn was
// under a second, and TestLiveInteractiveClaudePaneNeverGoesQuietMidTurn fails
// if that ever reaches half the threshold.
//
// It is deliberately far longer than agent.activityWindow: this is not a
// second opinion on the heuristic, it is a staleness bound on the report. The
// heuristic's own window is 900ms, close enough to the observed silence that
// tightening this towards it would flicker.
//
// A var, not a const, only so a test can shrink it: the alternative is a unit
// test that sleeps for ten seconds.
var staleWorkingReport = 10 * time.Second

// statusOf maps a session to its sidebar dot.
//
// The order is deliberate. No runner at all means the session was never
// opened or has been closed, which is reported rather than guessed at. A dead
// process is ground truth and outranks anything an agent said before it died.
// A state reported by hooks outranks the activity heuristic, which infers
// "working" from bytes arriving and cannot tell "thinking" from "waiting for
// you" at all. The heuristic is the last resort, for agents that report
// nothing — cathode, or claude before its first turn.
//
// The one exception is a stale "working": see staleWorkingReport.
func (m Model) statusOf(sess *store.Session) (glyph, label string, style lipgloss.Style) {
	s := m.styles
	r, ok := m.runners[sess.ID]
	if !ok {
		return "○", "Closed", s.Faint
	}
	if r.Status() == agent.Exited {
		if err := r.Err(); err != nil {
			return "◍", "Exited: " + truncate(err.Error(), 20), s.Error
		}
		return "◍", "Exited", s.Error
	}
	if m.coord != nil {
		if state, reported := m.coord.StateOf(sess.ID); reported {
			switch state {
			case coord.StateWorking:
				if r.Quiet() > staleWorkingReport {
					break // the report outlived the turn; the PTY knows better
				}
				return "◉", "Working", s.Accent
			case coord.StateWaiting:
				return "◆", "Needs you", s.Accent
			case coord.StateIdle:
				return "◉", "Idle", s.Muted
			}
		}
	}
	if r.Status() == agent.Working {
		return "◉", "Working", s.Accent
	}
	return "◉", "Idle", s.Muted
}
