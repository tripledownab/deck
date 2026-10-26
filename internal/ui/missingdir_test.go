package ui

// Ending and recreating a session from the session view, including the one
// whose worktree was deleted by hand. That session cannot start, so x in the
// sidebar was the only way out of it and it used to answer "not running".

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tripledownab/deck/internal/gittest"
	"github.com/tripledownab/deck/internal/store"
)

// inSessionView puts a deckWithWorktree model on its session's sidebar row.
func inSessionView(t *testing.T, m Model) Model {
	t.Helper()
	m.screen = screenSession
	m.rebuildRows()
	if m.currentSession() == nil {
		t.Fatal("the sidebar has no session selected; the test proves nothing")
	}
	return m
}

// TestXInTheSessionViewEndsAStoppedSession is the dead end this file fixes.
// Both routes to x are driven: the bare key and the one after the prefix.
func TestXInTheSessionViewEndsAStoppedSession(t *testing.T) {
	m, _ := deckWithWorktree(t)
	m = inSessionView(t, m)

	for name, press := range map[string]func(Model) (tea.Model, tea.Cmd){
		"x":    func(m Model) (tea.Model, tea.Cmd) { return m.sessionKey(typed("x")) },
		"^g x": func(m Model) (tea.Model, tea.Cmd) { return m.command(typed("x")) },
	} {
		got, _ := press(m)
		if got.(Model).picker == nil {
			t.Errorf("%s on a stopped session did not open the end-session modal", name)
		}
	}
}

// TestXInTheSessionViewStopsALiveAgent keeps the first x what it was. Ending a
// live session takes a second press, so one key never takes a running agent
// and its session together.
func TestXInTheSessionViewStopsALiveAgent(t *testing.T) {
	m, sess := deckWithWorktree(t)
	m = inSessionView(t, m)
	m.runners[sess.ID] = startAgent(t, sess.Dir, "")

	got, _ := m.sessionKey(typed("x"))
	after := got.(Model)

	if _, ok := after.runners[sess.ID]; ok {
		t.Error("x left the agent running")
	}
	if after.picker != nil {
		t.Error("x on a live agent went straight to the end-session modal")
	}
}

// TestRecreateRestoresAGoneWorktree drives the whole path a user takes: x on
// the stuck session, the modal opening on Recreate, and enter.
func TestRecreateRestoresAGoneWorktree(t *testing.T) {
	m, sess := deckWithWorktree(t)
	if err := os.RemoveAll(sess.Dir); err != nil {
		t.Fatal(err)
	}
	m = inSessionView(t, m)

	opened, _ := m.sessionKey(typed("x"))
	asked := opened.(Model)
	if asked.picker == nil {
		t.Fatal("x did not open the end-session modal")
	}
	if got := asked.picker.selected(); got != endRecreate {
		t.Fatalf("the modal opened on %q, want %q", got, endRecreate)
	}

	picked, _ := asked.pickerKey(tea.KeyMsg{Type: tea.KeyEnter})
	done := picked.(Model)

	if done.fault != nil {
		t.Fatalf("Recreate reported: %v", done.fault)
	}
	if _, err := os.Stat(sess.Dir); err != nil {
		t.Errorf("the worktree is still missing: %v", err)
	}
	if n := len(done.state.Sessions); n != 1 {
		t.Errorf("sessions = %d, want the session kept", n)
	}
}

// TestGoneWorktreeIsNotClaimedKept holds every place that would otherwise say
// the tree is still on disk: the pane's hint, the Close row, and the notice.
func TestGoneWorktreeIsNotClaimedKept(t *testing.T) {
	m, sess := deckWithWorktree(t)
	if err := os.RemoveAll(sess.Dir); err != nil {
		t.Fatal(err)
	}
	m = inSessionView(t, m)

	if pane := m.renderPane(80, 10); !strings.Contains(pane, "Press x") {
		t.Errorf("the pane does not point at x:\n%s", pane)
	}
	opened, _ := m.sessionKey(typed("x"))
	for _, row := range opened.(Model).picker.rows {
		if strings.Contains(row.desc, "keep the worktree") {
			t.Errorf("row %q offers to keep a worktree that is gone", row.label)
		}
	}
	m.closeSession(sess)
	if strings.Contains(m.notice, "kept") {
		t.Errorf("notice = %q, want no claim that the worktree was kept", m.notice)
	}
}

// TestGoneWorktreeUnderAnExitedAgent covers the tree deleted while its agent
// ran. The agent exits, the pane keeps its last screen, and the hint under it
// must not offer ↵, which can no longer start anything there.
func TestGoneWorktreeUnderAnExitedAgent(t *testing.T) {
	m, sess := deckWithWorktree(t)
	r := startAgent(t, sess.Dir, "exit 0")
	waitExited(t, r)
	m.runners[sess.ID] = r
	if err := os.RemoveAll(sess.Dir); err != nil {
		t.Fatal(err)
	}
	m = inSessionView(t, m)

	pane := m.renderPane(80, 10)
	if !strings.Contains(pane, "Press x") || strings.Contains(pane, "Press ↵") {
		t.Errorf("the hint does not point at x alone:\n%s", pane)
	}
}

// TestXEndsASessionWhoseAgentExited covers the other half of "nothing
// running". An exited runner stays in the map until it is replaced, and x on
// it must end the session rather than stop what has already stopped.
func TestXEndsASessionWhoseAgentExited(t *testing.T) {
	m, sess := deckWithWorktree(t)
	r := startAgent(t, sess.Dir, "exit 0")
	waitExited(t, r)
	m.runners[sess.ID] = r
	m = inSessionView(t, m)
	m.attached = true

	got, _ := m.command(typed("x"))
	after := got.(Model)

	if after.picker == nil {
		t.Error("x on an exited agent did not open the end-session modal")
	}
	if after.attached {
		t.Error("the pane stayed attached under the modal")
	}
}

// TestNoRecreateWithoutTheBranch keeps the modal from offering what
// RestoreWorktree will refuse, and keeps the rows saying something.
func TestNoRecreateWithoutTheBranch(t *testing.T) {
	m, sess := deckWithWorktree(t)
	repo := m.state.Project(sess.ProjectID).Path
	if err := os.RemoveAll(sess.Dir); err != nil {
		t.Fatal(err)
	}
	gittest.Run(t, repo, "worktree", "prune")
	gittest.Run(t, repo, "branch", "-D", sess.Branch)
	m = inSessionView(t, m)

	opened, _ := m.sessionKey(typed("x"))
	for _, row := range opened.(Model).picker.rows {
		if row.id == endRecreate {
			t.Error("Recreate is offered for a branch that no longer exists")
		}
		if row.desc == "" {
			t.Errorf("row %q says nothing", row.label)
		}
	}
}

// TestRecreateLinksTheProjectFiles is newSession's promise kept on the way
// back: an agent in a recreated tree gets the same ignored instructions.
func TestRecreateLinksTheProjectFiles(t *testing.T) {
	m, sess := deckWithWorktree(t)
	repo := m.state.Project(sess.ProjectID).Path
	if err := os.WriteFile(filepath.Join(repo, ".gitignore"), []byte("CLAUDE.md\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "CLAUDE.md"), []byte("rules\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(sess.Dir); err != nil {
		t.Fatal(err)
	}

	m.recreateWorktree(sess)

	if m.fault != nil {
		t.Fatalf("Recreate reported: %v", m.fault)
	}
	if _, err := os.Lstat(filepath.Join(sess.Dir, "CLAUDE.md")); err != nil {
		t.Errorf("the recreated tree has no instructions: %v", err)
	}
}

// TestEndingFromTheSidebarLeavesTheDashboardCursor is forget's promise for a
// session the dashboard is not showing. Before the sidebar could end one,
// the ended session was always in the dashboard's project.
func TestEndingFromTheSidebarLeavesTheDashboardCursor(t *testing.T) {
	m, sess := deckWithWorktree(t)
	other := m.state.AddProject(store.Project{Name: "other", Path: t.TempDir()})
	for range 3 {
		m.state.AddSession(store.Session{ProjectID: other.ID, Name: "o", Dir: other.Path})
	}
	m.rebuildRows()
	m.projectIx = 1
	m.listIx = 2
	m.focusContent()

	m.closeSession(sess)

	if m.listIx != 2 || m.focus != colContent {
		t.Errorf("listIx = %d, focus = %v, want the cursor left on the other project", m.listIx, m.focus)
	}
}

// TestADanglingLinkIsNotAGoneWorktree keeps Recreate off a path git will
// refuse to add over. Stat follows the link and would call it gone.
func TestADanglingLinkIsNotAGoneWorktree(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wt")
	if err := os.Symlink(filepath.Join(t.TempDir(), "nowhere"), path); err != nil {
		t.Fatal(err)
	}
	if dirGone(path) {
		t.Error("a dangling link at the path counts as gone")
	}
}
