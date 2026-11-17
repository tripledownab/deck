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
	if !strings.Contains(m.notice, "will start cathode next time") || strings.Contains(m.notice, "keeps running") {
		t.Errorf("notice = %q, want the switch said to wait for the next start", m.notice)
	}

	m.runners[sess.ID] = startAgent(t, sess.Dir, "")
	next, _ = m.editSession(sess.ID, "work", "codex")
	if notice := next.(Model).notice; !strings.Contains(notice, "bash keeps running until it is stopped") {
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

// TestAnExitedAgentIsNotSaidToKeepRunning: an exited runner is still in the
// map, and a notice saying it keeps running would be false.
func TestAnExitedAgentIsNotSaidToKeepRunning(t *testing.T) {
	m, sess := oneSession(t, "claude")
	exited := startAgent(t, sess.Dir, "exit 0")
	waitExited(t, exited)
	m.runners[sess.ID] = exited

	next, _ := m.editSession(sess.ID, "work", "codex")
	if notice := next.(Model).notice; strings.Contains(notice, "keeps running") {
		t.Errorf("notice = %q about an agent that has exited", notice)
	}
}

// TestBothChangesAreSaid: a title and an agent changed together are both
// reported, rather than the agent replacing the rename.
func TestBothChangesAreSaid(t *testing.T) {
	m, sess := oneSession(t, "claude")
	next, _ := m.editSession(sess.ID, "new title", "codex")
	if notice := next.(Model).notice; !strings.Contains(notice, "renamed to new title") || !strings.Contains(notice, "codex") {
		t.Errorf("notice = %q, want both the rename and the switch", notice)
	}
}

// TestARunningSessionIsLabelledByWhatRuns: a switch waits for the next start,
// so until then the context bar names the program in the pane, and the
// exited pane does not promise to start the new one "again".
func TestARunningSessionIsLabelledByWhatRuns(t *testing.T) {
	m, sess := oneSession(t, "bash")
	r := startAgent(t, sess.Dir, "exit 0")
	m.runners[sess.ID] = r
	next, _ := m.editSession(sess.ID, "work", "codex")
	m = next.(Model)

	if bar := m.contextBar(); !strings.Contains(bar, "bash") || strings.Contains(bar, "codex") {
		t.Errorf("context bar = %q, want the running bash, not the pending codex", bar)
	}
	waitExited(t, r)
	if pane := m.renderPane(100, 12); strings.Contains(pane, "start codex again") || !strings.Contains(pane, "start codex") {
		t.Errorf("exited pane after a switch:\n%s", pane)
	}
}

// TestAPickThatFailsToStartIsNotKept: an agent that is not installed leaves
// the session's agent as it was, so the next ↵ does not offer it as current.
func TestAPickThatFailsToStartIsNotKept(t *testing.T) {
	m, sess := oneSession(t, "claude")
	exited := startAgent(t, sess.Dir, "exit 0")
	waitExited(t, exited)
	m.runners[sess.ID] = exited
	fakeBin(t, map[string]string{"claude": fakeClaude}) // no codex

	m.pickerSubject = sess.ID
	next, _ := m.agentPicked("codex")
	m = next.(Model)
	if m.fault == nil {
		t.Fatal("starting a missing agent reported nothing")
	}
	if sess.Agent != "claude" {
		t.Errorf("session agent = %q after a failed start, want claude kept", sess.Agent)
	}
	if saved, err := store.Load(); err == nil && saved.Session(sess.ID) != nil && saved.Session(sess.ID).Agent == "codex" {
		t.Error("the failed choice was saved")
	}
}
