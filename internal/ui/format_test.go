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
