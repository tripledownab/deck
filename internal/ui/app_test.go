package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tripledownab/deck/internal/store"
)

func modelWith(sessions int) Model {
	st := &store.State{}
	p := st.AddProject(store.Project{Name: "demo", Path: "/demo"})
	for i := range sessions {
		st.AddSession(store.Session{
			ProjectID: p.ID,
			Name:      "sess",
			Title:     "session",
			Isolated:  true,
			Dir:       "/worktrees/sess",
		})
		_ = i
	}
	return New(st, "bash", nil)
}

// TestToggleColumnRefusesEmptyList is the regression for "tab appears to do
// nothing". Focusing an empty session list gives the arrows no row to move and
// shows no cursor, so it reads as a heading that recolours for no reason.
func TestToggleColumnRefusesEmptyList(t *testing.T) {
	m := modelWith(0)
	m.toggleColumn()

	if m.focus != colProjects {
		t.Error("focus moved to an empty session list")
	}
	if m.notice == "" {
		t.Error("refusing to move focus said nothing about why")
	}
}

func TestToggleColumnMovesBothWays(t *testing.T) {
	m := modelWith(2)

	m.toggleColumn()
	if m.focus != colContent {
		t.Fatal("tab did not move focus to the session list")
	}

	m.toggleColumn()
	if m.focus != colProjects {
		t.Fatal("tab did not move focus back to the projects list")
	}
}

// TestCursorMarkerKeepsUnfocusedPosition pins the visual half of the same fix:
// both columns hold a position at all times, so the inactive one keeps a
// dimmed cursor rather than losing it.
func TestCursorMarkerKeepsUnfocusedPosition(t *testing.T) {
	m := modelWith(1)

	focused, _ := m.cursorMarker(true, true)
	unfocused, _ := m.cursorMarker(true, false)
	absent, _ := m.cursorMarker(false, true)

	if focused == unfocused {
		t.Error("focused and unfocused cursors render identically")
	}
	if unfocused == absent {
		t.Error("the unfocused cursor is invisible, so its row cannot be seen")
	}
}

// TestArrowsFollowFocus checks that the selection the arrows move depends on
// which column has focus.
func TestArrowsFollowFocus(t *testing.T) {
	m := modelWith(3)

	m.moveDashboard(1)
	if m.projectIx != 0 {
		t.Errorf("with one project, projectIx = %d, want 0", m.projectIx)
	}
	if m.listIx != 0 {
		t.Errorf("project focus moved the session cursor to %d", m.listIx)
	}

	m.toggleColumn()
	m.moveDashboard(1)
	if m.listIx != 1 {
		t.Errorf("session cursor = %d, want 1", m.listIx)
	}

	m.moveDashboard(-5) // clamps, never wraps past the top
	if m.listIx != 0 {
		t.Errorf("session cursor = %d after clamping, want 0", m.listIx)
	}
}

// TestSectionKeysRespectFocus is the regression for ←/→ driving the right
// column while the left one had focus.
//
// The focused column is drawn with an accent top border. Section keys that
// ignored focus made that border a lie — it said the keys drove the projects
// list while they were changing the detail column's tabs.
func TestSectionKeysRespectFocus(t *testing.T) {
	m := modelWith(2)
	if m.focus != colProjects {
		t.Fatalf("focus = %v, want the projects column", m.focus)
	}

	m.sectionRight()
	if m.tabIx != 0 {
		t.Errorf("→ changed the section to %d while the projects list had focus", m.tabIx)
	}
	// And it does not move focus either: changing columns is tab's job alone.
	if m.focus != colProjects {
		t.Errorf("→ from the projects list moved focus to %v", m.focus)
	}

	// With content focused, the same key switches sections.
	m.focusContent()
	m.sectionRight()
	if m.tabIx != 1 {
		t.Errorf("section = %d after → with content focused, want 1", m.tabIx)
	}
	m.sectionLeft()
	if m.tabIx != 0 {
		t.Errorf("section = %d after ←, want 0", m.tabIx)
	}
}

// TestSectionLeftIsNoOpOnProjects pins the other direction: the projects list
// is already leftmost, so ← there must not reach across to the detail column.
func TestSectionLeftIsNoOpOnProjects(t *testing.T) {
	m := modelWith(2)
	m.focusContent()
	m.sectionRight() // move to the Sessions section
	m.focus = colProjects

	m.sectionLeft()
	if m.tabIx != 1 {
		t.Errorf("← from the projects list changed the section to %d, want it untouched at 1", m.tabIx)
	}
	if m.focus != colProjects {
		t.Errorf("← from the projects list moved focus to %v", m.focus)
	}
}

// TestSectionKeysNeverMoveFocus pins the division of labour: ← and → stay
// inside the detail column, tab is the only key that changes columns.
func TestSectionKeysNeverMoveFocus(t *testing.T) {
	m := modelWith(2)
	for _, step := range []func(){m.sectionRight, m.sectionLeft} {
		step()
		if m.focus != colProjects {
			t.Fatalf("a section key moved focus to %v", m.focus)
		}
	}
}

// TestCloseIgnoresProjectsColumn is the regression for x closing a session
// nobody had pointed at.
//
// It is TestSectionKeysRespectFocus applied to the one key that cannot be
// undone. The projects list draws no session cursor and the footer does not
// offer x there, so the key was destructive and undocumented in the same place.
//
// Driven through dashboardKey rather than through the helper: the guard being
// correct proves nothing if the route does not reach it.
func TestCloseIgnoresProjectsColumn(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	m := modelWith(2)
	if m.focus != colProjects {
		t.Fatalf("focus = %v, want the projects column", m.focus)
	}

	refused, _ := m.dashboardKey(typed("x"))
	after := refused.(Model)
	if after.picker != nil {
		t.Error("x opened the end-session modal from the projects list")
	}
	if n := len(after.state.Sessions); n != 2 {
		t.Errorf("sessions = %d after x on the projects list, want both kept", n)
	}
	if after.notice == "" {
		t.Error("the refused key said nothing about why")
	}

	// The same key still asks with the session list focused, so what refused
	// above was the guard and not a route that had stopped working.
	after.focusContent()
	asked, _ := after.dashboardKey(typed("x"))
	if asked.(Model).picker == nil {
		t.Error("x did not open the modal with the session list focused")
	}
}

// TestEndModalClosesAndKeepsTheWorktree drives the whole route: the key, the
// modal, and the default row.
//
// Close is the row the cursor starts on, so enter without moving is the
// reversible outcome. Naming the directory is the whole promise of keeping it —
// a notice that only said "closed" would leave the work somewhere the user
// cannot find.
func TestEndModalClosesAndKeepsTheWorktree(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	m := modelWith(2)
	m.focusContent()

	opened, _ := m.dashboardKey(typed("x"))
	asked := opened.(Model)
	if asked.picker == nil {
		t.Fatal("x did not open the end-session modal")
	}
	if got := asked.picker.selected(); got != endClose {
		t.Errorf("the modal opened on %q, want the reversible row %q", got, endClose)
	}

	closed, _ := asked.pickerKey(tea.KeyMsg{Type: tea.KeyEnter})
	done := closed.(Model)
	if done.picker != nil {
		t.Error("the modal stayed up after a choice")
	}
	if n := len(done.state.Sessions); n != 1 {
		t.Errorf("sessions = %d after choosing Close, want 1", n)
	}
	if !strings.Contains(done.notice, "/worktrees/sess") {
		t.Errorf("notice = %q, want it to name where the worktree was kept", done.notice)
	}
}

// TestEndModalEscapeKeepsTheSession is the reason the modal exists. x alone
// used to end a session outright, so the key had no step at which the user
// could change their mind.
func TestEndModalEscapeKeepsTheSession(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	m := modelWith(2)
	m.focusContent()

	opened, _ := m.dashboardKey(typed("x"))
	away, _ := opened.(Model).pickerKey(tea.KeyMsg{Type: tea.KeyEsc})
	after := away.(Model)

	if after.picker != nil {
		t.Error("esc left the modal up")
	}
	if n := len(after.state.Sessions); n != 2 {
		t.Errorf("sessions = %d after esc, want both kept", n)
	}
}

// TestEndSkipsTheModalWithoutAWorktree covers a session that ran in the project
// directory. It has no worktree and no branch of its own, so Delete would have
// nothing to remove and a modal offering it would be offering nothing.
func TestEndSkipsTheModalWithoutAWorktree(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	st := &store.State{}
	p := st.AddProject(store.Project{Name: "demo", Path: "/demo"})
	st.AddSession(store.Session{ProjectID: p.ID, Name: "inplace", Title: "in place", Dir: "/demo"})
	m := New(st, "bash", nil)
	m.focusContent()

	ended, _ := m.dashboardKey(typed("x"))
	after := ended.(Model)

	if after.picker != nil {
		t.Fatal("a session with no worktree was asked what to do with one")
	}
	if n := len(after.state.Sessions); n != 0 {
		t.Errorf("sessions = %d, want the session closed outright", n)
	}
	if strings.Contains(after.notice, "worktree") {
		t.Errorf("notice = %q, want no mention of a worktree it never had", after.notice)
	}
}

// TestClosingTheLastSessionReleasesFocus keeps colContent meaning "a session is
// selected", which is what focusedSession's refusal depends on.
//
// focusContent refuses to focus an empty session list. Closing the last session
// reached that same state from the other side, and the next x then refused in
// silence — there was no session for the notice to be about.
func TestClosingTheLastSessionReleasesFocus(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	m := modelWith(1)
	m.focusContent()
	if m.focus != colContent {
		t.Fatal("could not focus the session list")
	}

	m.closeSession(*m.focusedSession())

	if m.focus != colProjects {
		t.Errorf("focus = %v after closing the last session, want the projects column", m.focus)
	}
}

// TestConnectIgnoresProjectsColumn covers the other key that resolves through
// the cursor, and with it the notice surviving the route.
//
// dashboardKey takes a value receiver, so the refusal is written into a copy of
// the model that has to be the one returned. A far project exists here on
// purpose: without it the picker would refuse for its own reason and the test
// would pass while proving nothing.
func TestConnectIgnoresProjectsColumn(t *testing.T) {
	m := modelWith(1)
	far := m.state.AddProject(store.Project{Name: "other", Path: "/other"})
	m.state.AddSession(store.Session{ProjectID: far.ID, Name: "far", Title: "far"})
	m.rebuildRows()

	refused, _ := m.dashboardKey(typed("c"))
	after := refused.(Model)
	if after.picker != nil {
		t.Error("c opened the connect picker from the projects list")
	}
	if after.notice == "" {
		t.Error("the refused key said nothing about why")
	}

	after.focusContent()
	opened, _ := after.dashboardKey(typed("c"))
	if opened.(Model).picker == nil {
		t.Error("c did not open the picker with the session list focused")
	}
}
