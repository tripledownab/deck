package ui

// The footer's fault line: shown until a key Deck handles acknowledges it,
// and never acknowledged by a key the agent receives.

import (
	"errors"
	"strings"
	"testing"
)

// TestAKeyDeckHandlesAcknowledgesTheFault: a fault that stayed for the whole
// run hid every notice after it. The key's own notice is the one shown next.
func TestAKeyDeckHandlesAcknowledgesTheFault(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	m := sized(New(mouseState(1, 3), "bash", nil), 120, 30)
	m.focusContent()
	m.fault = errors.New("an earlier failure")

	m = sendKey(m, typed("K")) // at the top already: sets a notice
	if m.fault != nil {
		t.Errorf("fault = %v after a key Deck handled", m.fault)
	}
	if !strings.Contains(m.dashboardFooter(), "already at the top") {
		t.Errorf("footer = %q, want the key's own notice", m.dashboardFooter())
	}
}

// TestAKeyForTheAgentLeavesTheFault: typing into an attached agent is not
// reading Deck's footer, so the fault stays until a key reaches Deck.
func TestAKeyForTheAgentLeavesTheFault(t *testing.T) {
	m := sessionView(t, 1, 120, 30)
	sess := m.currentSession()
	m.runners[sess.ID] = startAgent(t, sess.Dir, "")
	m.attached = true
	m.fault = errors.New("an earlier failure")

	m = sendKey(m, typed("a"))
	if m.fault == nil {
		t.Error("a key sent to the agent cleared the fault")
	}
}
