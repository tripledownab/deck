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
// later event to clear it, and three turn-ends turned out not to arrive under
// the original config. Two were fixable by widening the set of events
// registered. One is not observable at all, so the pane is the backstop.
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

// The status glyphs.
//
// Shape carries the state and colour only reinforces it. Idle and working were
// both ◉ and told apart by accent against muted, which is no distinction at all
// in a low-contrast theme or to a reader who cannot separate the two hues — and
// the sidebar card now shows the glyph without the word beside it, so the shape
// is all there is.
//
// Filled is busy and hollow is quiet, which is the reading people arrive at
// unprompted. Closed is a dot rather than a hollow circle so it does not
// compete with idle, and a dead process is a cross rather than a circle at all:
// it is not a degree of running.
const (
	glyphClosed  = "·"
	glyphIdle    = "○"
	glyphWorking = "●"
	glyphWaiting = "◆"
	glyphExited  = "✕"
)

// status is what a session's dot, and the words beside it, say.
//
// A struct rather than a fourth return value. form.update grew to four and the
// architecture doc records what that cost — every caller and every test had to
// be updated, and the next field would cost the same again.
type status struct {
	glyph string
	// label is the state in words, for the places with room to print it.
	label string
	// detail is what neither the glyph nor the label can carry: the error a
	// dead process left behind. Empty for every other state.
	detail string
	style  lipgloss.Style
}

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
func (m Model) statusOf(sess *store.Session) status {
	s := m.styles
	r, ok := m.runners[sess.ID]
	if !ok {
		return status{glyph: glyphClosed, label: "Closed", style: s.Faint}
	}
	if r.Status() == agent.Exited {
		st := status{glyph: glyphExited, label: "Exited", style: s.Error}
		if err := r.Err(); err != nil {
			st.detail = truncate(err.Error(), 20)
		}
		return st
	}
	if m.coord != nil {
		if state, reported := m.coord.StateOf(sess.ID); reported {
			switch state {
			case coord.StateWorking:
				if r.Quiet() > staleWorkingReport {
					break // the report outlived the turn; the PTY knows better
				}
				return status{glyph: glyphWorking, label: "Working", style: s.Accent}
			case coord.StateWaiting:
				return status{glyph: glyphWaiting, label: "Needs you", style: s.Accent}
			case coord.StateIdle:
				return status{glyph: glyphIdle, label: "Idle", style: s.Muted}
			}
		}
	}
	if r.Status() == agent.Working {
		return status{glyph: glyphWorking, label: "Working", style: s.Accent}
	}
	return status{glyph: glyphIdle, label: "Idle", style: s.Muted}
}
