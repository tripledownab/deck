package ui

// Switching a session's agent: ahead of time from its edit form, and from the
// list ↵ opens once its agent has exited. Fake programs on a PATH the test
// owns stand in for the agents, as in agentinfo_test.go.

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tripledownab/deck/internal/store"
)

// oneSession is a model on the session view with one session running agent.
func oneSession(t *testing.T, agentCmd string) (Model, *store.Session) {
	t.Helper()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	st := &store.State{}
	p := st.AddProject(store.Project{Name: "demo", Path: t.TempDir()})
	sess := st.AddSession(store.Session{ProjectID: p.ID, Name: "s", Title: "work", Agent: agentCmd, Dir: t.TempDir()})
	m := sized(New(st, agentCmd, nil), 120, 30)
	m.screen = screenSession
	t.Cleanup(func() { m.Close() })
	return m, sess
}

// TestTheEditFormSwitchesTheAgent: saved, and said to wait for the next
// start — with a pointer at x when the old agent is still running.
func TestTheEditFormSwitchesTheAgent(t *testing.T) {
	m, sess := oneSession(t, "claude")

	next, _ := m.editSession(sess.ID, "work", "cathode")
	m = next.(Model)
	saved, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := saved.Session(sess.ID).Agent; got != "cathode" {
		t.Errorf("saved agent = %q, want cathode", got)
	}
	if !strings.Contains(m.notice, "will start cathode next time") || strings.Contains(m.notice, "x stops") {
		t.Errorf("notice = %q, want the switch said to wait for the next start", m.notice)
	}

	m.runners[sess.ID] = startAgent(t, sess.Dir, "")
	next, _ = m.editSession(sess.ID, "work", "codex")
	if notice := next.(Model).notice; !strings.Contains(notice, "x stops the one running now") {
		t.Errorf("with an agent running, notice = %q", notice)
	}

	next, _ = next.(Model).editSession(sess.ID, "retitled", "codex")
	if notice := next.(Model).notice; notice != "renamed to retitled" {
		t.Errorf("a title-only edit said %q", notice)
	}
}

// TestPickingAnotherAgentAfterAnExitStartsIt: ↵ on the exited agent opens the
// list on it, and choosing another starts that one and keeps it.
func TestPickingAnotherAgentAfterAnExitStartsIt(t *testing.T) {
	m, sess := oneSession(t, "claude")
	exited := startAgent(t, sess.Dir, "exit 0") // bash, so before PATH is replaced
	waitExited(t, exited)
	m.runners[sess.ID] = exited
	bin := fakeBin(t, map[string]string{"claude": fakeClaude, "cathode": "#!/bin/sh\nexec /bin/sleep 30\n"})

	m = sendKey(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.picker == nil || m.picker.kind != pickAgent || m.picker.selected() != "claude" {
		t.Fatalf("↵ on an exited agent did not open the agent list on claude: %+v", m.picker)
	}
	m = sendKey(m, tea.KeyMsg{Type: tea.KeyDown})
	if m.picker.selected() != "cathode" {
		t.Fatalf("the row after claude is %q; the test aims at cathode", m.picker.selected())
	}
	m = sendKey(m, tea.KeyMsg{Type: tea.KeyEnter})
	t.Cleanup(func() { m.Close() })

	r := m.runners[sess.ID]
	if r == nil || r == exited || r.Path() != filepath.Join(bin, "cathode") {
		t.Fatalf("runner after picking cathode = %v", r)
	}
	saved, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := saved.Session(sess.ID).Agent; got != "cathode" {
		t.Errorf("saved agent = %q, want the one picked kept", got)
	}
}

// TestEscapingTheAgentListStartsNothing: the list is a question, and esc
// leaves the exited agent and the session's choice as they were.
func TestEscapingTheAgentListStartsNothing(t *testing.T) {
	// cathode, not the first row, so a list that opened on its first row
	// rather than on the session's agent would show.
	m, sess := oneSession(t, "cathode")
	exited := startAgent(t, sess.Dir, "exit 0")
	waitExited(t, exited)
	m.runners[sess.ID] = exited

	m = sendKey(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.picker == nil || m.picker.selected() != "cathode" {
		t.Fatalf("the list did not open on the session's agent: %+v", m.picker)
	}
	m = sendKey(m, tea.KeyMsg{Type: tea.KeyDown})
	m = sendKey(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.picker != nil || m.runners[sess.ID] != exited || sess.Agent != "cathode" {
		t.Errorf("esc changed something: picker %v, agent %q", m.picker, sess.Agent)
	}
}
