package ui

import (
	"testing"

	"github.com/tripledownab/deck/internal/store"
)

// TestCompactCountSwitchesUnitAtTheRightPlace pins both boundaries. The badge
// is the only report of what a spawned review is using, and a figure that
// changes unit one short of where it should reads as a review a thousand times
// larger or smaller than it is.
func TestCompactCountSwitchesUnitAtTheRightPlace(t *testing.T) {
	for _, tc := range []struct {
		n    int
		want string
	}{
		{0, "0"},
		{999, "999"},   // last value printed in full
		{1000, "1.0k"}, // first abbreviated one
		{1500, "1.5k"},
		{9999, "10.0k"}, // last with a tenth
		{10000, "10k"},  // first without one
		{23800, "24k"},
	} {
		if got := compactCount(tc.n); got != tc.want {
			t.Errorf("compactCount(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}

// TestSessionLabelFallsBackToTheGeneratedName covers the rule the sidebar used
// to state inline. A session recorded before titles existed has none, and a
// blank row tells the reader nothing about which session it is.
func TestSessionLabelFallsBackToTheGeneratedName(t *testing.T) {
	if got := sessionLabel(store.Session{Title: "rate limiting", Name: "swift-otter-aaaa"}); got != "rate limiting" {
		t.Errorf("label = %q, want the title", got)
	}
	if got := sessionLabel(store.Session{Name: "swift-otter-aaaa"}); got != "swift-otter-aaaa" {
		t.Errorf("label = %q, want the generated name", got)
	}
}

// TestProjectLabelFallsBackToTheDirectory. Name is filled in from the
// directory when the field is left empty, so this stands for a hand-edited
// state file rather than the usual path.
func TestProjectLabelFallsBackToTheDirectory(t *testing.T) {
	st := &store.State{}
	named := st.AddProject(store.Project{Name: "api-gateway", Path: "/src/gw"})
	blank := st.AddProject(store.Project{Path: "/src/billing-service"})

	if got := projectLabel(st, named.ID); got != "api-gateway" {
		t.Errorf("label = %q, want the name", got)
	}
	if got := projectLabel(st, blank.ID); got != "billing-service" {
		t.Errorf("label = %q, want the directory", got)
	}
	if got := projectLabel(st, "gone"); got == "" {
		t.Error("an unknown project rendered as an empty label")
	}
}
