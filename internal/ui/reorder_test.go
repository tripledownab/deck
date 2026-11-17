package ui

// Moving a session within its project, by key and by dragging it, aimed at the
// frame as drawn like the other mouse tests.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tripledownab/deck/internal/store"
)

// titles lists the first project's sessions by title, in list order.
func titles(m Model) string {
	var out []string
	for _, s := range m.state.SessionsFor(m.state.Projects[0].ID) {
		out = append(out, s.Title)
	}
	return strings.Join(out, ",")
}

func motion(m Model, x, y int) Model {
	next, _ := m.Update(tea.MouseMsg{X: x, Y: y, Button: tea.MouseButtonLeft, Action: tea.MouseActionMotion})
	return next.(Model)
}

func release(m Model, x, y int) Model {
	next, _ := m.Update(tea.MouseMsg{X: x, Y: y, Button: tea.MouseButtonNone, Action: tea.MouseActionRelease})
	return next.(Model)
}

func sendKey(m Model, msg tea.KeyMsg) Model {
	next, _ := m.Update(msg)
	return next.(Model)
}

// TestKeysMoveTheFocusedSessionOnTheDashboard: both spellings of each
// direction, the cursor staying on the session it moved, the order surviving
// a reload, and the end of the list saying so instead of doing nothing.
func TestKeysMoveTheFocusedSessionOnTheDashboard(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	m := sized(New(mouseState(1, 3), "bash", nil), 120, 30)
	m.focusContent()

	m = sendKey(m, typed("J"))
	if got := titles(m); got != "title-01,title-00,title-02" {
		t.Fatalf("J gave %s", got)
	}
	m = sendKey(m, tea.KeyMsg{Type: tea.KeyShiftDown})
	if got := titles(m); got != "title-01,title-02,title-00" {
		t.Fatalf("shift+down gave %s", got)
	}
	if s := m.dashboardSession(); s == nil || s.Title != "title-00" {
		t.Errorf("the cursor is on %v, want it to follow title-00", s)
	}
	saved, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := titles(Model{state: saved}); got != "title-01,title-02,title-00" {
		t.Errorf("the saved order is %s", got)
	}

	m = sendKey(m, typed("J"))
	if !strings.Contains(m.notice, "bottom") {
		t.Errorf("J at the bottom said %q", m.notice)
	}
	m = sendKey(m, tea.KeyMsg{Type: tea.KeyShiftUp})
	m = sendKey(m, typed("K"))
	if got := titles(m); got != "title-00,title-01,title-02" {
		t.Errorf("shift+up then K gave %s", got)
	}
	m = sendKey(m, typed("K"))
	if !strings.Contains(m.notice, "top") {
		t.Errorf("K at the top said %q", m.notice)
	}
}

// TestTheSidebarDrawsTheNewOrder reads the order off the frame. The store's
// order and the cursor can both be right while the sidebar still draws the
// rows it built before the move.
func TestTheSidebarDrawsTheNewOrder(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	m := sessionView(t, 3, 120, 30)
	sidebarW, _ := m.layout()

	m = sendKey(m, typed("J"))
	_, y0 := cellOf(t, m, "title-00", 0, sidebarW)
	_, y1 := cellOf(t, m, "title-01", 0, sidebarW)
	if y1 >= y0 {
		t.Errorf("after J, title-00 is drawn on line %d and title-01 on %d; want title-01 above", y0, y1)
	}
}

// TestAFailedSaveIsReportedAndTheMoveKept: the list on screen is the one in
// memory, so a move whose save failed stays, and says why it is not on disk.
func TestAFailedSaveIsReportedAndTheMoveKept(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_STATE_HOME", blocker)
	m := sized(New(mouseState(1, 2), "bash", nil), 120, 30)
	m.focusContent()

	m = sendKey(m, typed("J"))
	if m.fault == nil {
		t.Error("a failed save said nothing")
	}
	if got := titles(m); got != "title-01,title-00" {
		t.Errorf("the move was undone by its failed save: %s", got)
	}
}

// TestEndingTheDraggedSessionEndsTheDrag holds the button on a card and ends
// that session from the keyboard. Motion after it must find nothing to move,
// not a session that is no longer in the store.
func TestEndingTheDraggedSessionEndsTheDrag(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	m := sessionView(t, 3, 120, 30)
	sidebarW, _ := m.layout()
	fromX, fromY := cellOf(t, m, "title-00", 0, sidebarW)
	toX, toY := cellOf(t, m, "title-02", 0, sidebarW)

	m = press(m, tea.MouseButtonLeft, fromX, fromY)
	m = sendKey(m, typed("x")) // not isolated: closes outright, no modal
	if len(m.state.Sessions) != 2 {
		t.Fatalf("x did not end the session; %d remain", len(m.state.Sessions))
	}
	m = motion(m, toX, toY)
	if m.dragging != "" {
		t.Error("a drag outlived the session it was moving")
	}

	// The store is still asked, for a drag whose session went another way.
	// Aimed afresh: the close moved every card below the one it ended.
	toX, toY = cellOf(t, m, "title-02", 0, sidebarW)
	m.dragging = "gone"
	if got := titles(motion(m, toX, toY)); got != "title-01,title-02" {
		t.Errorf("motion for a session no longer in the store gave %s", got)
	}
}

// TestKeysMoveTheSidebarSession covers the session view, detached with the bare
// key and attached through the prefix, where the bare key would go to the agent.
func TestKeysMoveTheSidebarSession(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	m := sessionView(t, 3, 120, 30)

	m = sendKey(m, typed("J"))
	if got := titles(m); got != "title-01,title-00,title-02" {
		t.Fatalf("J gave %s", got)
	}
	if s := m.currentSession(); s == nil || s.Title != "title-00" {
		t.Errorf("the sidebar cursor is on %v, want it to follow title-00", s)
	}
	m.attached = true
	m = sendKey(m, tea.KeyMsg{Type: tea.KeyCtrlG})
	m = sendKey(m, typed("J"))
	if got := titles(m); got != "title-01,title-02,title-00" {
		t.Errorf("^g J gave %s", got)
	}
}

// TestDragMovesASidebarSession presses a card, drags it over another, and
// lets go. The press that started it must not then pair with the next press:
// it was made on a row that now holds another session.
func TestDragMovesASidebarSession(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	m := sessionView(t, 4, 120, 30)
	sidebarW, _ := m.layout()
	fromX, fromY := cellOf(t, m, "title-00", 0, sidebarW)
	toX, toY := cellOf(t, m, "title-02", 0, sidebarW)

	m = press(m, tea.MouseButtonLeft, fromX, fromY)
	m = motion(m, toX, toY)
	if got := titles(m); got != "title-01,title-02,title-00,title-03" {
		t.Fatalf("dragging title-00 onto title-02 gave %s", got)
	}
	m = release(m, toX, toY)
	if m.dragging != "" {
		t.Error("the release did not end the drag")
	}

	m = press(m, tea.MouseButtonLeft, fromX, fromY)
	if m.attached || len(m.runners) != 0 {
		t.Error("the press after a drag paired with the one that started it and opened a session")
	}
}

// TestDragMovesADashboardSession is the same in the dashboard's session list.
func TestDragMovesADashboardSession(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	m := sized(New(mouseState(1, 4), "bash", nil), 120, 30)
	navW := m.dashboardLayout().navW
	fromX, fromY := cellOf(t, m, "title-03", navW, m.width)
	toX, toY := cellOf(t, m, "title-00", navW, m.width)

	m = press(m, tea.MouseButtonLeft, fromX, fromY)
	m = motion(m, toX, toY)
	if got := titles(m); got != "title-03,title-00,title-01,title-02" {
		t.Fatalf("dragging title-03 onto title-00 gave %s", got)
	}
	if s := m.dashboardSession(); s == nil || s.Title != "title-03" {
		t.Errorf("the cursor is on %v, want it on the dragged session", s)
	}
}

// TestDragOnlyMovesWithinAProjectAndAPress: a session never moves to another
// project, motion with no press behind it moves nothing, and neither does
// motion after the release or after a key, here one that opens a modal.
func TestDragOnlyMovesWithinAProjectAndAPress(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	st := mouseState(2, 3)
	st.AddSession(store.Session{ProjectID: st.Projects[1].ID, Name: "name-other",
		Title: "other-title", Agent: "bash", Dir: t.TempDir()})
	for i := range st.Sessions {
		st.Sessions[i].Dir = t.TempDir()
	}
	m := New(st, "bash", nil)
	m.screen = screenSession
	m = sized(m, 120, 30)
	t.Cleanup(func() { m.Close() })
	sidebarW, _ := m.layout()
	// From the bottom of its list, so a drop wrongly taken as "the other
	// project's first row" would move it to the top and show.
	fromX, fromY := cellOf(t, m, "title-02", 0, sidebarW)
	otherX, otherY := cellOf(t, m, "other-title", 0, sidebarW)
	toX, toY := cellOf(t, m, "title-00", 0, sidebarW)
	want := titles(m)

	if got := titles(motion(m, toX, toY)); got != want {
		t.Errorf("motion with no press moved the list to %s", got)
	}
	m = press(m, tea.MouseButtonLeft, fromX, fromY)
	if got := titles(motion(m, otherX, otherY)); got != want {
		t.Errorf("dragging onto another project's session gave %s", got)
	}
	if got := titles(motion(release(m, fromX, fromY), toX, toY)); got != want {
		t.Errorf("motion after the release moved the list to %s", got)
	}
	helped := sendKey(m, typed("?"))
	if !helped.showHelp {
		t.Fatal("? did not open the help; the check below proves nothing")
	}
	if got := titles(motion(helped, toX, toY)); got != want {
		t.Errorf("motion after a key opened a modal moved the list to %s", got)
	}
}

// TestAPressEndsADragWhoseReleaseNeverCame: a button let go outside the window
// sends no release, so the next press has to end the drag, or motion from a
// later click elsewhere would still drag the old session.
func TestAPressEndsADragWhoseReleaseNeverCame(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	m := sessionView(t, 3, 120, 30)
	sidebarW, _ := m.layout()
	fromX, fromY := cellOf(t, m, "title-00", 0, sidebarW)
	toX, toY := cellOf(t, m, "title-02", 0, sidebarW)
	want := titles(m)

	m = press(m, tea.MouseButtonLeft, fromX, fromY)
	m = press(m, tea.MouseButtonLeft, sidebarW+10, toY) // over the pane
	if got := titles(motion(m, toX, toY)); got != want {
		t.Errorf("motion after a press elsewhere still dragged: %s", got)
	}
}

// TestMotionWithoutTheButtonDoesNotDrag: only a held left button drags.
func TestMotionWithoutTheButtonDoesNotDrag(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	m := sessionView(t, 3, 120, 30)
	sidebarW, _ := m.layout()
	fromX, fromY := cellOf(t, m, "title-00", 0, sidebarW)
	toX, toY := cellOf(t, m, "title-02", 0, sidebarW)
	want := titles(m)

	m = press(m, tea.MouseButtonLeft, fromX, fromY)
	next, _ := m.Update(tea.MouseMsg{X: toX, Y: toY, Button: tea.MouseButtonNone, Action: tea.MouseActionMotion})
	if got := titles(next.(Model)); got != want {
		t.Errorf("motion with no button held moved the list to %s", got)
	}
}

// TestADashboardDragStaysInItsProject: the dashboard shows one project's
// sessions, and a key can change which while the button is held. A drop on the
// project now shown must not move a session of the project it left.
func TestADashboardDragStaysInItsProject(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	st := mouseState(2, 3)
	st.AddSession(store.Session{ProjectID: st.Projects[1].ID, Name: "name-other", Title: "other-title", Agent: "bash"})
	m := sized(New(st, "bash", nil), 120, 30)
	navW := m.dashboardLayout().navW
	fromX, fromY := cellOf(t, m, "title-02", navW, m.width)
	want := titles(m)

	m = press(m, tea.MouseButtonLeft, fromX, fromY)
	m.selectProject(1)
	toX, toY := cellOf(t, m, "other-title", navW, m.width)
	if got := titles(motion(m, toX, toY)); got != want {
		t.Errorf("a drop on another project's list moved the dragged session: %s", got)
	}
}
