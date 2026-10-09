package agent

// Mouse input to the agent: which mouse reports the agent has asked for, and
// the wheel notches Deck sends it in the form it asked for.

import (
	"slices"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/vt"
)

// mouseTracking lists the modes in which a terminal reports wheel notches as
// mouse events. They are the modes the emulator's own SendMouse acts on.
var mouseTracking = []ansi.Mode{
	ansi.ModeMouseX10,
	ansi.ModeMouseNormal,
	ansi.ModeMouseHighlight,
	ansi.ModeMouseButtonEvent,
	ansi.ModeMouseAnyEvent,
}

// x10Limit is the last 0-based cell an X10 report can carry: a coordinate is
// one byte, offset by 33, so 222 is the most that fits in 255.
const x10Limit = 255 - 33

// trackModes keeps r.mouse current from the emulator's mode changes: the
// tracking modes and SGR encoding (?1006). A full reset (RIS) sets every mode
// again through the same path, so the record cannot outlive a reset. It must
// be called before the read pump starts.
func (r *Runner) trackModes() {
	set := func(on bool) func(ansi.Mode) {
		return func(mode ansi.Mode) {
			if mode == ansi.ModeMouseExtSgr || slices.Contains(mouseTracking, mode) {
				r.mu.Lock()
				r.mouse[mode] = on
				r.mu.Unlock()
			}
		}
	}
	r.term.SetCallbacks(vt.Callbacks{EnableMode: set(true), DisableMode: set(false)})
}

// Wheel sends one wheel notch at cell (x, y) of the agent's screen.
//
// An agent that asked for mouse reports gets a wheel report, which it reads as
// a scroll. The report is SGR if the agent set ?1006 and X10 otherwise, even
// if it asked for another encoding (?1005, ?1015, ?1016) instead. Any other
// agent gets the ↑ or ↓ key, so a pane with no mouse tracking still moves
// under the wheel. Sending the key to an agent that tracks the mouse is wrong:
// cathode, with mouse capture on, reads ↑ as the previous prompt from its
// history, not as a scroll.
//
// The report is encoded here and written as a key is, not sent through the
// emulator's SendMouse. SendMouse writes to the reply pipe while holding the
// emulator lock, so an agent that stopped reading its input would block it,
// then the read pump, then every Render. Its X10 encoder also writes a
// coordinate of 95 or more as two UTF-8 bytes.
func (r *Runner) Wheel(up bool, x, y int) error {
	r.mu.Lock()
	tracking, sgr := false, r.mouse[ansi.ModeMouseExtSgr]
	for _, t := range mouseTracking {
		tracking = tracking || r.mouse[t]
	}
	r.mu.Unlock()

	button, key := ansi.MouseWheelDown, "\x1b[B"
	if up {
		button, key = ansi.MouseWheelUp, "\x1b[A"
	}
	b := ansi.EncodeMouseButton(button, false, false, false, false)
	switch {
	case !tracking:
		return r.Write([]byte(key))
	case sgr:
		return r.Write([]byte(ansi.MouseSgr(b, x, y, false)))
	default:
		x, y = min(x, x10Limit), min(y, x10Limit)
		return r.Write([]byte{0x1b, '[', 'M', b + 32, byte(x + 33), byte(y + 33)})
	}
}
