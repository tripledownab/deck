package ui

// Small shared helpers for drawing: ANSI-aware truncation, clipping,
// and duration formatting.

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"
)

// oneRowSpaces turns the characters that break a one-row slot into spaces.
//
// A newline breaks the row outright. A tab breaks it by width: the cell count
// that decides where to cut reads a tab as zero cells, and lipgloss then draws
// it as several, so a name that was cut to fit is drawn wider than its slot and
// wraps.
var oneRowSpaces = strings.NewReplacer("\n", " ", "\t", " ")

// truncate fits s to one row of width display cells, appending an ellipsis
// when it had to cut. It is ANSI-aware, so it is safe on already-styled text.
//
// Every caller fills a one-row slot, so a newline or a tab becomes a space (see
// oneRowSpaces). A line that wraps makes the frame taller than the terminal.
// Bubble Tea then drops rows from the top of the frame, so the header goes and
// every row under it is drawn a line higher than the code placed it, while a
// click still resolves against the placement.
func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	s = oneRowSpaces.Replace(s)
	if ansi.StringWidth(s) <= width {
		return s
	}
	return ansi.Truncate(s, width, "…")
}

// truncateStyled is truncate without the ellipsis, for text that is already a
// composed row and would look wrong with one. A newline or a tab becomes a
// space, for the reason truncate gives.
func truncateStyled(s string, width int) string {
	if width <= 0 {
		return ""
	}
	return ansi.Truncate(oneRowSpaces.Replace(s), width, "")
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
