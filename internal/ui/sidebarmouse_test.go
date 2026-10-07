package ui

// The mouse over the session view's sidebar. Like the dashboard's click tests
// in mouse_test.go, these aim at the frame as drawn, never at the hit map's
// own arithmetic.

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// sessionView is the session view on a store whose first project has the
// given sessions, sized so a long list clips. Every session has a real
// directory, so opening one starts an agent.
func sessionView(t *testing.T, sessions, w, h int) Model {
	t.Helper()
	st := mouseState(1, sessions)
	for i := range st.Sessions {
		st.Sessions[i].Dir = t.TempDir()
	}
	m := New(st, "bash", nil)
	m.screen = screenSession
	m = sized(m, w, h)
	t.Cleanup(func() { m.Close() })
	return m
}

// TestClickSelectsTheSidebarCardDrawnThere clicks both lines of cards in a
// sidebar that has scrolled, so a click resolved from the row number alone
// would select the wrong session.
func TestClickSelectsTheSidebarCardDrawnThere(t *testing.T) {
	m := sessionView(t, 20, 120, 20)
	m.jumpToSession(10) // title-09: names count from 00
	m = sized(m, 120, 20)
	sidebarW, _ := m.layout()

	for _, want := range []int{4, 7, 9} {
		for _, label := range []string{fmt.Sprintf("title-%02d", want), fmt.Sprintf("session/name-%02d", want)} {
			x, y := cellOf(t, m, label, 0, sidebarW)
			got := press(m, tea.MouseButtonLeft, x, y)
			if s := got.currentSession(); s == nil || s.Title != fmt.Sprintf("title-%02d", want) {
				t.Errorf("clicked %s, selected %v", label, s)
			}
		}
	}
}

// TestDoubleClickOnASidebarCardOpensIt: the second press of a pair starts the
// agent and attaches. The card is near the bottom of a clipped sidebar, where
// a list that re-centred on the first press would move it out from under the
// pointer before the second.
func TestDoubleClickOnASidebarCardOpensIt(t *testing.T) {
	m := sessionView(t, 20, 120, 20)
	sidebarW, _ := m.layout()
	found := cellsOf(t, m, "title-", 0, sidebarW)
	last := found[len(found)-1]
	label := shownFrame(t, m)[last.y]

	once := press(m, tea.MouseButtonLeft, last.x, last.y)
	if once.attached || len(once.runners) != 0 {
		t.Fatal("a first press opened the session")
	}
	twice := press(once, tea.MouseButtonLeft, last.x, last.y)
	t.Cleanup(func() { twice.Close() })
	if !twice.attached || twice.currentRunner() == nil {
		t.Fatalf("the second press did not open the session clicked (%q)", label)
	}
	if s := once.currentSession(); s != twice.currentSession() {
		t.Errorf("the second press opened %v, the first selected %v", twice.currentSession(), s)
	}
}

// TestClickFromOneLiveAgentToAnotherStaysAttached: a click selects as ^g j
// does, and switching between live agents without letting go of the keyboard
// is the point of running several.
func TestClickFromOneLiveAgentToAnotherStaysAttached(t *testing.T) {
	m := sessionView(t, 2, 120, 30)
	for _, s := range m.state.Sessions {
		m.runners[s.ID] = startAgent(t, s.Dir, "")
	}
	m.attached = true
	sidebarW, _ := m.layout()

	x, y := cellOf(t, m, "title-01", 0, sidebarW)
	got := press(m, tea.MouseButtonLeft, x, y)
	if s := got.currentSession(); s == nil || s.Title != "title-01" || !got.attached {
		t.Errorf("selected %v, attached %v; want title-01 and still attached", s, got.attached)
	}
}

// TestSidebarMouseActsOnlyOnCards: a heading, the pane, and a wheel notch over
// the pane leave the cursor where it was and open nothing; the wheel over the
// sidebar moves it. A heading and an empty last press are the same zero
// target, so a click on one must not count as the second of a pair.
//
// Each case gets its own model. Copies of a Model share one runners map, so an
// agent one case started would be blamed on the next.
func TestSidebarMouseActsOnlyOnCards(t *testing.T) {
	probe := sessionView(t, 3, 120, 30)
	sidebarW, _ := probe.layout()
	hx, hy := cellOf(t, probe, "PROJECT-00", 0, sidebarW)

	for _, c := range []struct {
		name string
		b    tea.MouseButton
		x, y int
	}{
		{"click on a heading", tea.MouseButtonLeft, hx, hy},
		{"click on the pane", tea.MouseButtonLeft, sidebarW + 10, hy + 2},
		{"wheel on the pane", tea.MouseButtonWheelDown, sidebarW + 10, hy + 2},
	} {
		m := sessionView(t, 3, 120, 30)
		got := press(m, c.b, c.x, c.y)
		if got.rowIx != m.rowIx {
			t.Errorf("%s moved the cursor from row %d to %d", c.name, m.rowIx, got.rowIx)
		}
		if got.attached || len(got.runners) != 0 {
			t.Errorf("%s opened a session", c.name)
		}
	}
	if got := press(probe, tea.MouseButtonWheelDown, 2, hy+2); got.rowIx == probe.rowIx {
		t.Error("the wheel over the sidebar did not move the cursor")
	}
}

// TestMovingShowsTheWholeCard: the sidebar keeps its place between frames,
// and it must still bring the whole selected card into view, not one line of
// it. Moving down risks hiding the branch behind the bottom marker, moving
// back up the title behind the top one.
func TestMovingShowsTheWholeCard(t *testing.T) {
	m := sessionView(t, 20, 120, 20)
	sidebarW, _ := m.layout()
	for _, k := range []tea.KeyType{tea.KeyDown, tea.KeyUp} {
		for range 19 {
			next, _ := m.Update(tea.KeyMsg{Type: k})
			m = next.(Model)
			s := m.currentSession()
			if len(cellsOf(t, m, s.Title, 0, sidebarW)) == 0 || len(cellsOf(t, m, s.Branch, 0, sidebarW)) == 0 {
				t.Fatalf("%s to %s, but its card is not shown whole", k, s.Title)
			}
		}
	}
}

// TestAPressBetweenBreaksThePair: a press anywhere else between two clicks on
// a card makes the second a first click again, as a key does.
func TestAPressBetweenBreaksThePair(t *testing.T) {
	m := sessionView(t, 3, 120, 30)
	sidebarW, _ := m.layout()
	x, y := cellOf(t, m, "title-01", 0, sidebarW)
	for name, between := range map[string]tea.MouseButton{
		"a click on the pane": tea.MouseButtonLeft,
		"a wheel notch":       tea.MouseButtonWheelDown,
	} {
		got := press(press(press(m, tea.MouseButtonLeft, x, y), between, sidebarW+10, y), tea.MouseButtonLeft, x, y)
		if got.attached || len(got.runners) != 0 {
			t.Errorf("%s between two clicks still opened the session", name)
		}
	}
}

// TestShortTerminalClicksLandOnTheCardDrawn: the agent's terminal is never
// under five rows, and drawing all of them on a shorter screen made the frame
// taller than the terminal, so every card was drawn a row above where a click
// resolved it. shownFrame fails first if the frame is too tall.
func TestShortTerminalClicksLandOnTheCardDrawn(t *testing.T) {
	for h := 4; h <= 9; h++ {
		m := sessionView(t, 3, 120, h)
		sidebarW, _ := m.layout()
		frame := shownFrame(t, m)
		for _, c := range cellsOf(t, m, "title-", 0, sidebarW) {
			want := ansi.Strip(frame[c.y])
			want = want[strings.Index(want, "title-"):][:len("title-00")]
			after := press(m, tea.MouseButtonLeft, c.x, c.y)
			if got := after.currentSession(); got == nil || got.Title != want {
				t.Errorf("height %d: clicked %s, selected %v", h, want, got)
			}
		}
	}
}
