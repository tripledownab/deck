package ui

// Small shared helpers for drawing: ANSI-aware truncation, scroll windowing,
// and duration formatting.

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"
)

// truncate shortens s to width display cells, appending an ellipsis when it
// had to cut. It is ANSI-aware, so it is safe on already-styled text.
func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if ansi.StringWidth(s) <= width {
		return s
	}
	return ansi.Truncate(s, width, "…")
}

// truncateStyled is truncate without the ellipsis, for text that is already a
// composed row and would look wrong with one.
func truncateStyled(s string, width int) string {
	if width <= 0 {
		return ""
	}
	return ansi.Truncate(s, width, "")
}

// pad right-pads s to width cells.
func pad(s string, width int) string {
	gap := width - ansi.StringWidth(s)
	if gap <= 0 {
		return s
	}
	return s + strings.Repeat(" ", gap)
}

// clip forces a slice to exactly height lines, padding with blanks or cutting
// the tail. Panes have a fixed shape; a list that is one line off shifts the
// whole frame.
func clip(lines []string, height int) []string {
	if height <= 0 {
		return nil
	}
	if len(lines) > height {
		return lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return lines
}

// moreGlyph marks a column edge with content past it.
//
// The same glyph truncate appends, and deliberately so: it already means "cut
// off, there is more" everywhere else in the frame, and a second symbol for the
// same fact is a second thing to learn.
//
// One character, so it fits the narrowest column Deck will draw, and it carries
// no count: the project list's lines are projects, while the sidebar's are
// thirds of a session card, so any number would be right in one column and
// wrong in the other. Which edge it sits on is the direction.
const moreGlyph = "…"

// window returns the height-line slice of lines that keeps focus visible,
// scrolling only when it has to.
//
// A clipped edge becomes more, so a column that continues off screen says so.
// Without it a list that fits and one that is cut look identical, and arrowing
// past the edge is the only way to find out which you are looking at. Pass ""
// to mark nothing.
//
// The marker arrives already styled. This file draws and does not know the
// palette.
//
// Nothing is marked below three lines. The markers cost the first and last row,
// and a two-line column would be all marker and no content.
func window(lines []string, focus, height int, more string) []string {
	if height <= 0 {
		return nil
	}
	if len(lines) <= height {
		return clip(lines, height)
	}
	start := focus - height/2
	if start < 0 {
		start = 0
	}
	if start > len(lines)-height {
		start = len(lines) - height
	}
	out := lines[start : start+height]
	if more == "" || height < 3 {
		return out
	}
	// Copied before overwriting: the slice above aliases the caller's lines,
	// and marking in place would edit the list itself rather than this view of
	// it. Centring keeps focus off both edges whenever a marker goes there, so
	// no marker can land on the row the cursor is meant to be showing.
	marked := make([]string, height)
	copy(marked, out)
	if start > 0 {
		marked[0] = more
	}
	if start+height < len(lines) {
		marked[height-1] = more
	}
	return marked
}

// firstLine returns the first line of primary, or fallback when primary is
// empty. Session titles are free text and a pasted prompt can carry newlines.
func firstLine(primary, fallback string) string {
	primary = strings.TrimSpace(primary)
	if primary == "" {
		return fallback
	}
	if i := strings.IndexByte(primary, '\n'); i >= 0 {
		return primary[:i]
	}
	return primary
}

// ago formats a timestamp the way the reference dashboard does: one unit, no
// "ago" suffix — 6d, 33d, 12m.
func ago(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return shortDuration(time.Since(t))
}

// plural renders a count with its noun, adding an s for anything but one.
func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

func shortDuration(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}
