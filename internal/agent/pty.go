package agent

// The bytes moving between the process and the emulator: the read pump, the
// reply pump that answers terminal queries, writes, resizes, and teardown.

import (
	"errors"
	"time"

	"github.com/creack/pty"
)

// pump copies PTY output into the emulator until the process exits.
func (r *Runner) pump() {
	buf := make([]byte, 32*1024)
	for {
		n, err := r.ptmx.Read(buf)
		if n > 0 {
			r.termMu.Lock()
			_, _ = r.term.Write(buf[:n])
			r.termMu.Unlock()
			r.mu.Lock()
			r.lastWrite = time.Now()
			r.mu.Unlock()
		}
		if err != nil {
			// A closed PTY reports EIO on Linux and EOF on macOS once the
			// child is gone. Neither is an error worth showing; Wait gives
			// us the real exit status.
			waitErr := r.cmd.Wait()
			r.mu.Lock()
			r.exited = true
			r.exitErr = waitErr
			r.mu.Unlock()
			r.closePTY()
			// The emulator is left open and intact: the pane still shows the
			// agent's final screen under an "exited" banner. See release for
			// why it is never Closed.
			return
		}
	}
}

// respond copies the emulator's replies back to the agent.
//
// This half of the loop is mandatory, not a nicety. A terminal is
// bidirectional: programs ask it questions — cursor position (DSR), device
// attributes (DA), background colour (OSC 11) — and block on the answer. The
// emulator queues its replies on an io.Pipe, and an io.Pipe write blocks until
// something reads it. That write happens inside the parser, which runs under
// the emulator lock, so with no reader the first query an agent sends would
// deadlock the session outright.
//
// It also fixes a slow path worth knowing about: termenv waits five seconds
// for an OSC 11 reply before falling back to $COLORFGBG. Unanswered, every
// lipgloss-based agent would stall that long before drawing its first frame.
// It keeps draining after the PTY is gone rather than returning. The
// emulator's parser writes replies to that pipe while holding the emulator
// lock, so a pump that stopped reading would not merely leak — it would block
// the parser mid-write and freeze every Render with it.
func (r *Runner) respond() {
	buf := make([]byte, 4096)
	for {
		n, err := r.term.Read(buf)
		if n > 0 {
			// A failed write means the PTY is closed. Discard and keep the
			// pipe drained.
			_, _ = r.ptmx.Write(buf[:n])
		}
		if err != nil {
			return
		}
	}
}

// Write forwards keystrokes to the agent.
func (r *Runner) Write(p []byte) error {
	if len(p) == 0 {
		return nil
	}
	if r.Status() == Exited {
		return errors.New("agent: process has exited")
	}
	_, err := r.ptmx.Write(p)
	return err
}

// Resize changes both the emulator screen and the kernel's idea of the
// terminal size. Both are required: the emulator so our render is the right
// shape, the ioctl so the agent redraws to fit and receives SIGWINCH.
func (r *Runner) Resize(w, h int) {
	if w < 20 {
		w = 20
	}
	if h < 5 {
		h = 5
	}
	r.termMu.Lock()
	if r.term.Width() == w && r.term.Height() == h {
		r.termMu.Unlock()
		return
	}
	r.term.Resize(w, h)
	r.termMu.Unlock()

	// Skipped once the PTY is gone: there is nothing to inform, and taking its
	// descriptor would race the close. See ptyMu.
	r.ptyMu.Lock()
	defer r.ptyMu.Unlock()
	if r.ptyGone {
		return
	}
	_ = pty.Setsize(r.ptmx, &pty.Winsize{Rows: uint16(h), Cols: uint16(w)})
}

// closePTY closes the pseudo-terminal exactly once, and marks it gone so no
// later Resize reaches for its descriptor.
func (r *Runner) closePTY() {
	r.ptyMu.Lock()
	defer r.ptyMu.Unlock()
	if r.ptyGone {
		return
	}
	r.ptyGone = true
	_ = r.ptmx.Close()
}

// release frees the emulator's memory once a session is finished with.
//
// It shrinks the screen and drops the scrollback instead of calling
// Emulator.Close, which would be the obvious move and is not safe: Close
// writes the emulator's closed flag, and unlike every other mutating method
// it is promoted straight from the embedded Emulator rather than wrapped by
// SafeEmulator's mutex. Calling it while the parser is running is a data race
// the detector catches. Until that is guarded upstream, the reply pump stays
// parked on the emulator's pipe for the life of the process — one idle
// goroutine per stopped session, against a scrollback that was the actual
// weight.
func (r *Runner) release() {
	r.termMu.Lock()
	defer r.termMu.Unlock()
	r.term.ClearScrollback()
	r.term.Resize(20, 5)
}
