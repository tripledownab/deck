package ui

import (
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
		got := window(lines, focus, 5)
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
