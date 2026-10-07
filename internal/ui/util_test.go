package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
)

func TestTruncate(t *testing.T) {
	cases := []struct {
		in, want string
		width    int
	}{
		{"hello", "hello", 10},
		{"hello", "hello", 5}, // exactly fits: no ellipsis
		{"hello", "hel…", 4},  // the ellipsis is inside the budget
		{"hello", "", 0},
		{"", "", 5},
	}
	for _, c := range cases {
		got := truncate(c.in, c.width)
		if got != c.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", c.in, c.width, got, c.want)
		}
		// The result must never exceed the budget. Every pane row is a fixed
		// cell count, so one cell over shifts the whole frame.
		if w := lipgloss.Width(got); w > c.width {
			t.Errorf("truncate(%q, %d) is %d cells wide", c.in, c.width, w)
		}
	}
}

// TestTruncateCountsCellsNotBytes matters because every pane row must be an
// exact number of terminal cells; a byte-counting truncate would shift the
// frame on any styled or non-ASCII text.
func TestTruncateCountsCellsNotBytes(t *testing.T) {
	styled := lipgloss.NewStyle().Bold(true).Render("session/scheming-hawk")
	got := truncate(styled, 10)
	if w := lipgloss.Width(got); w > 10 {
		t.Errorf("truncated width = %d cells, want <= 10", w)
	}
}

func TestClipForcesExactHeight(t *testing.T) {
	if got := clip([]string{"a", "b", "c"}, 2); len(got) != 2 {
		t.Errorf("clip down gave %d lines, want 2", len(got))
	}
	got := clip([]string{"a"}, 4)
	if len(got) != 4 {
		t.Errorf("clip up gave %d lines, want 4", len(got))
	}
	if got[0] != "a" || got[3] != "" {
		t.Errorf("clip padded wrongly: %q", got)
	}
}

func TestWindowKeepsFocusVisible(t *testing.T) {
	lines := make([]string, 20)
	for i := range lines {
		lines[i] = strings.Repeat("x", i+1)
	}
	for _, focus := range []int{0, 10, 19} {
		got := window(lines, focus, 5, "")
		if len(got) != 5 {
			t.Fatalf("window height = %d, want 5", len(got))
		}
		want := lines[focus]
		found := false
		for _, l := range got {
			if l == want {
				found = true
			}
		}
		if !found {
			t.Errorf("focus %d not visible in window %q", focus, got)
		}
	}
}

func TestFirstLine(t *testing.T) {
	if got := firstLine("one\ntwo", "fallback"); got != "one" {
		t.Errorf("firstLine multi = %q", got)
	}
	if got := firstLine("  ", "fallback"); got != "fallback" {
		t.Errorf("firstLine blank = %q", got)
	}
}

func TestShortDuration(t *testing.T) {
	cases := map[time.Duration]string{
		30 * time.Second:    "30s",
		5 * time.Minute:     "5m",
		3 * time.Hour:       "3h",
		50 * time.Hour:      "2d",
		33 * 24 * time.Hour: "33d",
	}
	for d, want := range cases {
		if got := shortDuration(d); got != want {
			t.Errorf("shortDuration(%s) = %q, want %q", d, got, want)
		}
	}
}

func TestPlural(t *testing.T) {
	cases := map[int]string{0: "0 projects", 1: "1 project", 2: "2 projects"}
	for n, want := range cases {
		if got := plural(n, "project"); got != want {
			t.Errorf("plural(%d) = %q, want %q", n, got, want)
		}
	}
}

// TestWindowMarksAClippedEdge is the gap this closes: a column that fits and
// one that is cut looked identical, so the only way to learn there was more was
// to arrow past the edge.
func TestWindowMarksAClippedEdge(t *testing.T) {
	lines := make([]string, 20)
	for i := range lines {
		lines[i] = fmt.Sprintf("line%d", i)
	}

	top := window(lines, 0, 5, moreGlyph)
	if top[0] == moreGlyph {
		t.Error("marked above the first line, where there is nothing above")
	}
	if top[4] != moreGlyph {
		t.Errorf("bottom row = %q, want the marker", top[4])
	}

	middle := window(lines, 10, 5, moreGlyph)
	if middle[0] != moreGlyph || middle[4] != moreGlyph {
		t.Errorf("mid-list window = %q, want both edges marked", middle)
	}

	end := window(lines, 19, 5, moreGlyph)
	if end[0] != moreGlyph {
		t.Errorf("top row = %q, want the marker", end[0])
	}
	if end[4] == moreGlyph {
		t.Error("marked below the last line, where there is nothing below")
	}
}

// TestWindowNeverMarksOverTheCursor is the property the markers rest on. They
// cost the first and last row, and a cursor hidden behind one would make the
// arrow keys appear to stop working at the edge of a long list.
func TestWindowNeverMarksOverTheCursor(t *testing.T) {
	lines := make([]string, 40)
	for i := range lines {
		lines[i] = fmt.Sprintf("line%d", i)
	}
	for height := 3; height <= 9; height++ {
		for focus := range lines {
			got := window(lines, focus, height, moreGlyph)
			found := false
			for _, l := range got {
				if l == lines[focus] {
					found = true
				}
			}
			if !found {
				t.Fatalf("height %d focus %d: the cursor row is not in %q", height, focus, got)
			}
		}
	}
}

// TestWindowLeavesAFittingListAlone keeps the marker off a list that is whole.
// A column saying there is more when there is not is worse than one saying
// nothing, because it sends the reader looking.
func TestWindowLeavesAFittingListAlone(t *testing.T) {
	lines := []string{"a", "b", "c"}
	got := window(lines, 0, 5, moreGlyph)
	for _, l := range got {
		if l == moreGlyph {
			t.Errorf("marked a list that fits: %q", got)
		}
	}
}

// TestWindowDoesNotMarkATinyColumn covers the degenerate height. The markers
// take the first and last row, so at two lines a clipped column would be all
// marker and no content.
func TestWindowDoesNotMarkATinyColumn(t *testing.T) {
	lines := []string{"a", "b", "c", "d", "e"}
	got := window(lines, 2, 2, moreGlyph)
	for _, l := range got {
		if l == moreGlyph {
			t.Errorf("marked a two-line column: %q", got)
		}
	}
}

// TestWindowDoesNotEditTheList pins that window marks its own copy. Marking in
// place would replace real rows in the caller's own list rather than in this
// view of it.
func TestWindowDoesNotEditTheList(t *testing.T) {
	lines := []string{"a", "b", "c", "d", "e", "f", "g"}
	window(lines, 3, 3, moreGlyph)
	for i, l := range lines {
		if l == moreGlyph {
			t.Errorf("line %d of the caller's list was overwritten: %q", i, lines)
		}
	}
}

// TestWindowedStaysUntilItHasToMove pins the scrolling rule for every previous
// start. The cursor always lands on a row that shows its line, never behind a
// marker. The window keeps its place while the cursor's line is still shown,
// which is what lets a second press land on the line the first one selected.
// When the cursor leaves, the window moves the least distance that shows it
// again, not half a page.
func TestWindowedStaysUntilItHasToMove(t *testing.T) {
	// shownFrom is which lines a window starting at s shows, worked out here
	// rather than asked of windowed, so the test is not grading itself.
	shownFrom := func(n, height, s int) (first, last int) {
		first, last = s, s+height-1
		if height >= 3 && s > 0 {
			first++
		}
		if height >= 3 && s+height < n {
			last--
		}
		return first, last
	}
	for n := 0; n <= 30; n++ {
		for height := 1; height <= 10; height++ {
			for from := -1; from <= n; from++ {
				for focus := 0; focus < n; focus++ {
					rows, start := windowed(n, focus, height, true, from)
					shown := false
					for _, i := range rows {
						shown = shown || i == focus
					}
					if !shown {
						t.Fatalf("n %d height %d from %d focus %d: the cursor's line is not shown in %v",
							n, height, from, focus, rows)
					}
					if n <= height || from < 0 || from > n-height {
						continue
					}
					// Moving at all needs a reason, and the move is the least
					// one that shows the cursor's line: a window one line short
					// of it, towards where it started, would not show it.
					shows := func(s int) bool {
						first, last := shownFrom(n, height, s)
						return focus >= first && focus <= last
					}
					switch {
					case shows(from) && start != from:
						t.Fatalf("n %d height %d from %d focus %d: the cursor's line was shown and the window moved to %d",
							n, height, from, focus, start)
					case start > from && shows(start-1):
						t.Fatalf("n %d height %d from %d focus %d: moved down to %d when %d already showed the cursor",
							n, height, from, focus, start, start-1)
					case start < from && shows(start+1):
						t.Fatalf("n %d height %d from %d focus %d: moved up to %d when %d already showed the cursor",
							n, height, from, focus, start, start+1)
					}
				}
			}
		}
	}
}

// TestTruncateKeepsOneRow: every caller of the two truncators fills a one-row
// slot, so neither a newline nor a tab may survive either of them. A newline
// wraps the slot, and a tab is counted as no cells but drawn as several, so it
// wraps a line that was cut to fit. Either pushes the frame past the
// terminal's height.
func TestTruncateKeepsOneRow(t *testing.T) {
	for name, fit := range map[string]func(string, int) string{
		"truncate": truncate, "truncateStyled": truncateStyled,
	} {
		for _, width := range []int{3, 8, 40} {
			if got := fit("line one\nline two", width); strings.Contains(got, "\n") {
				t.Errorf("%s at width %d kept a newline: %q", name, width, got)
			}
		}
		// A space, not nothing: the words either side stay words.
		if got := fit("line one\nline\ttwo", 40); got != "line one line two" {
			t.Errorf("%s = %q, want the newline and the tab as spaces", name, got)
		}
	}
}

// window returns the height rows of lines that keep focus visible, with more
// on a clipped edge. Pass "" to mark nothing. It has no previous frame to keep,
// so the window centres on focus.
//
// It is windowed and fill in one call, and lives here because only the
// windowing tests draw that way. The columns lay out with windowed and draw
// with fill, so the mouse can read the same layout.
func window(lines []string, focus, height int, more string) []string {
	rows, _ := windowed(len(lines), focus, height, more != "", -1)
	return fill(rows, lines, more)
}
