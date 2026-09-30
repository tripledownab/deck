package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tripledownab/deck/internal/store"
)

// TestLiteralPrefixToAnExitedPaneIsReported: ^g ^g types a literal ctrl+g into
// the pane. It used to discard a failed write, so typing it into an agent that
// had gone did nothing and said nothing, where any other keystroke reported it.
func TestLiteralPrefixToAnExitedPaneIsReported(t *testing.T) {
	st := &store.State{}
	p := st.AddProject(store.Project{Name: "api-gateway", Path: "/code/api-gateway"})
	st.AddSession(store.Session{ProjectID: p.ID, Name: "swift-otter-aaaa", Dir: t.TempDir(), Agent: "bash"})
	m := New(st, "bash", nil)
	r := startAgent(t, st.Sessions[0].Dir, "exit 0")
	waitExited(t, r)
	m.runners[st.Sessions[0].ID] = r
	m.screen, m.attached = screenSession, true

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlG})
	next, _ = next.(Model).Update(tea.KeyMsg{Type: tea.KeyCtrlG})
	got := next.(Model)
	if got.fault == nil || !strings.Contains(got.fault.Error(), "send to agent") {
		t.Errorf("fault = %v, want the failed write reported", got.fault)
	}
	if got.attached {
		t.Error("still attached to a pane that cannot be written")
	}
}
