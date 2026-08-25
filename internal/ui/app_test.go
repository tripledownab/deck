package ui

import (
	"testing"

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
