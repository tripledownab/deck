package ui

import (
	"testing"

	"github.com/tripledownab/deck/internal/store"
)

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
