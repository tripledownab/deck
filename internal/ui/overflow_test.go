package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// TestFormNeverOverflowsTheModal renders the new-session form for a range of
// project counts and terminal widths, and asserts no line is wider than the
// screen. Chips are laid out horizontally, so a growing project list is the
// obvious way to burst the box.
func TestFormNeverOverflowsTheModal(t *testing.T) {
	s := newStyles(paletteFor("cinder"))
	names := []string{
		"deck", "api-gateway", "billing-service", "content-pipeline",
		"design-system", "edge-router", "feature-flags", "graph-explorer",
		"image-resizer", "job-scheduler", "kv-store", "log-shipper",
	}

	for _, width := range []int{80, 100, 120} {
		for n := 1; n <= len(names); n++ {
			choices := make([]choice, 0, n)
			for _, nm := range names[:n] {
				choices = append(choices, choice{label: nm, value: nm})
			}
			f := newSessionForm(choices, 0, true, "claude")

			view := f.view(s, width, 40)
			for i, line := range strings.Split(view, "\n") {
				if w := ansi.StringWidth(line); w > width {
					t.Fatalf("%d projects at width %d: line %d is %d cells wide\n%s",
						n, width, i, w, line)
				}
			}
		}
	}
}

// TestChipsGiveWayToTheCycler pins the handover. Chips must be used while they
// fit and abandoned when they would not, because the alternative to switching
// is a row that runs past the modal border.
func TestChipsGiveWayToTheCycler(t *testing.T) {
	s := newStyles(paletteFor("cinder"))
	const width = 60

	chips, cycler := 0, 0
	for n := 1; n <= 12; n++ {
		choices := make([]choice, 0, n)
		for i := range n {
			choices = append(choices, choice{label: fmt.Sprintf("project-%d", i), value: "p"})
		}
		f := newSessionForm(choices, 0, true, "claude")
		out := renderChoices(s, &f.fields[sessionFieldProject], width)

		if strings.Contains(out, "\u2039") {
			cycler++
			continue
		}
		chips++
		// While chips are still in use they must actually fit.
		for _, line := range strings.Split(out, "\n") {
			if w := ansi.StringWidth(line); w > width {
				t.Errorf("%d projects: chips are %d cells wide, budget is %d", n, w, width)
			}
		}
	}

	if chips == 0 {
		t.Error("chips were never used, even for a single option")
	}
	if cycler == 0 {
		t.Error("the cycler never took over, so a long list would overflow")
	}
}
