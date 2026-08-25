package agent

// Painting the emulated screen as one styled string per row.

import (
	"image/color"
	"strings"

	uv "github.com/charmbracelet/ultraviolet"
)

// Render paints the emulated screen as one styled string per row.
//
// It walks cells rather than calling the emulator's own String, so that the
// caller can invert the cursor cell — a PTY pane inside another TUI has no
// real hardware cursor to place, and a prompt with no visible caret is
// unusable.
func (r *Runner) Render(cursor bool, cursorFg, cursorBg color.Color) []string {
	// Held across the whole walk. CellAt hands back a pointer into the live
	// buffer, so the read of Content and Style below is only safe while the
	// parser is held off. See termMu.
	r.termMu.Lock()
	defer r.termMu.Unlock()

	w, h := r.term.Width(), r.term.Height()
	pos := r.term.CursorPosition()

	lines := make([]string, 0, h)
	for y := range h {
		var line, run strings.Builder
		var runStyle uv.Style
		// flush emits the pending run under one escape sequence. Batching
		// runs of equal style matters: a full screen is tens of thousands of
		// cells per frame, and styling each one separately spends more time
		// building escape sequences than drawing.
		flush := func() {
			if run.Len() == 0 {
				return
			}
			line.WriteString(runStyle.Styled(run.String()))
			run.Reset()
		}

		for x := 0; x < w; {
			cell := r.term.CellAt(x, y)
			if cell == nil {
				if !runStyle.IsZero() {
					flush()
					runStyle = uv.Style{}
				}
				run.WriteByte(' ')
				x++
				continue
			}
			width := cell.Width
			if width <= 0 {
				// Continuation half of a wide grapheme; already emitted.
				x++
				continue
			}
			content := cell.Content
			if content == "" {
				content = strings.Repeat(" ", width)
			}
			style := cell.Style
			if cursor && y == pos.Y && x == pos.X {
				style.Fg, style.Bg = cursorFg, cursorBg
			}
			if !style.Equal(&runStyle) {
				flush()
				runStyle = style
			}
			run.WriteString(content)
			x += width
		}
		flush()
		lines = append(lines, line.String())
	}
	return lines
}
