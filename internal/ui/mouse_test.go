package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/tripledownab/deck/internal/store"
)

// mouseState is a store with enough projects that the project list scrolls,
// and a first project with enough sessions that the Overview tab cuts them
// off. Names are zero-padded so no name is a substring of another.
func mouseState(projects, sessions int) *store.State {
	st := &store.State{}
	old := time.Now().Add(-30 * 24 * time.Hour)
	for i := 0; i < projects; i++ {
		p := st.AddProject(store.Project{
			Name: fmt.Sprintf("project-%02d", i), Path: fmt.Sprintf("/code/p%02d", i), CreatedAt: old,
		})
		if i != 0 {
			continue
		}
		for j := 0; j < sessions; j++ {
			st.AddSession(store.Session{
				ProjectID: p.ID, Name: fmt.Sprintf("name-%02d", j), Title: fmt.Sprintf("title-%02d", j),
				Branch: fmt.Sprintf("session/name-%02d", j), Agent: "bash",
				// Newest first on screen, so session j is drawn at list index j.
				CreatedAt: old.Add(-time.Duration(j) * time.Minute),
			})
		}
	}
	return st
}

func sized(m Model, w, h int) Model {
	next, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return next.(Model)
}

func press(m Model, b tea.MouseButton, x, y int) Model {
	next, _ := m.Update(tea.MouseMsg{X: x, Y: y, Button: b, Action: tea.MouseActionPress})
	return next.(Model)
}

// shownFrame is the frame as the terminal shows it. A frame taller than the
// terminal loses its top rows, so every row under them is shown higher than
// View placed it, and a click aimed from View would miss. So a frame of the
// wrong height fails here rather than letting the aim be wrong.
func shownFrame(t *testing.T, m Model) []string {
	t.Helper()
	lines := strings.Split(m.View(), "\n")
	if len(lines) != m.height {
		t.Fatalf("the frame is %d lines on a %d-line terminal", len(lines), m.height)
	}
	return lines
}

type cell struct{ x, y int }

// cellsOf finds every place text is on screen between columns from and to,
// top to bottom. Aiming at the drawn frame is the point: a click placed with
// the hit map's own arithmetic would test the hit map against itself.
func cellsOf(t *testing.T, m Model, text string, from, to int) []cell {
	t.Helper()
	var out []cell
	for y, line := range shownFrame(t, m) {
		for off := 0; ; {
			i := strings.Index(line[off:], text)
			if i < 0 {
				break
			}
			if x := ansi.StringWidth(line[:off+i]); x >= from && x < to {
				out = append(out, cell{x, y})
			}
			off += i + len(text)
		}
	}
	return out
}

func cellOf(t *testing.T, m Model, text string, from, to int) (x, y int) {
	t.Helper()
	found := cellsOf(t, m, text, from, to)
	if len(found) == 0 {
		t.Fatalf("%q is not on screen between columns %d and %d", text, from, to)
	}
	return found[0].x, found[0].y
}

// TestClickSelectsTheProjectDrawnThere clicks rows of a scrolled project list.
// The list is windowed around the cursor, so a click resolved from the row
// number alone would select the wrong project whenever the list has scrolled.
func TestClickSelectsTheProjectDrawnThere(t *testing.T) {
	m := sized(New(mouseState(40, 3), "bash", nil), 100, 20)
	m.projectIx = 20
	navW := m.dashboardLayout().navW

	for _, want := range []int{10, 15, 20} {
		x, y := cellOf(t, m, fmt.Sprintf("project-%02d", want), 0, navW)
		got := press(m, tea.MouseButtonLeft, x, y)
		if got.projectIx != want {
			t.Errorf("clicked project-%02d, selected project-%02d", want, got.projectIx)
		}
		if got.focus != colProjects {
			t.Errorf("clicked project-%02d, focus stayed on the sessions list", want)
		}
	}

	// The markers on both clipped edges stand for no project.
	markers := cellsOf(t, m, moreGlyph, 0, navW)
	if len(markers) != 2 {
		t.Fatalf("found %d markers in the project list, want one on each edge", len(markers))
	}
	for _, c := range markers {
		if got := press(m, tea.MouseButtonLeft, c.x, c.y); got.projectIx != 20 {
			t.Errorf("a click on the marker at row %d selected project-%02d", c.y, got.projectIx)
		}
	}
}

// TestClickSelectsTheSessionDrawnThere covers both tabs. The Sessions tab draws
// two lines per session, so its rows and the session indices differ by a
// factor the hit map has to get from the drawing, not assume. The margin cell
// before a row's text belongs to the row.
func TestClickSelectsTheSessionDrawnThere(t *testing.T) {
	for tab, texts := range map[int][]string{
		0: {"title-00", "title-03", "title-05"},
		1: {"title-02", "session/name-02", "title-04"},
	} {
		m := sized(New(mouseState(3, 9), "bash", nil), 120, 40)
		m.tabIx = tab
		navW := m.dashboardLayout().navW
		for _, text := range texts {
			var want int
			fmt.Sscanf(text[strings.LastIndex(text, "-")+1:], "%d", &want)
			_, y := cellOf(t, m, text, navW, m.width)
			for _, x := range []int{navW, navW + 10} {
				got := press(m, tea.MouseButtonLeft, x, y)
				if got.listIx != want || got.focus != colContent {
					t.Errorf("tab %d: clicked %q at column %d, got listIx %d focus %v, want %d on the sessions list",
						tab, text, x, got.listIx, got.focus, want)
				}
			}
		}
	}

	// A session of the second project, so a click that also moved the project
	// cursor would show.
	st := mouseState(3, 2)
	p := &st.Projects[1]
	st.AddSession(store.Session{ProjectID: p.ID, Name: "name-other", Title: "other-title", Agent: "bash"})
	m := sized(New(st, "bash", nil), 120, 30)
	m.projectIx = 1
	x, y := cellOf(t, m, "other-title", m.dashboardLayout().navW, m.width)
	if got := press(m, tea.MouseButtonLeft, x, y); got.projectIx != 1 || got.listIx != 0 || got.focus != colContent {
		t.Errorf("clicked project-01's session: projectIx %d listIx %d focus %v", got.projectIx, got.listIx, got.focus)
	}
}

// TestClickOnTheSelectedRowOpensIt is ↵ by mouse, on the second of two presses
// in a row. A project with no sessions opens the new-session form, and a
// session opens its pane. The project the dashboard starts on already has the
// cursor and the focus, and a first click on it only selects: opening would
// start an agent from a click meant to choose.
func TestClickOnTheSelectedRowOpensIt(t *testing.T) {
	st := mouseState(2, 1)
	st.Sessions[0].Dir = t.TempDir()
	m := sized(New(st, "bash", nil), 120, 30)
	t.Cleanup(func() { m.Close() })
	navW := m.dashboardLayout().navW

	x, y := cellOf(t, m, "project-00", 0, navW)
	if got := press(m, tea.MouseButtonLeft, x, y); got.screen != screenDashboard || got.form != nil {
		t.Fatal("a first click on the project that starts selected opened it")
	}

	// The same for a session that has the cursor and the focus before the click.
	focused := m
	focused.focus = colContent
	x, y = cellOf(t, focused, "title-00", navW, m.width)
	if got := press(focused, tea.MouseButtonLeft, x, y); got.screen != screenDashboard {
		t.Fatal("a first click on the session that starts selected opened it")
	}

	x, y = cellOf(t, m, "project-01", 0, navW)
	once := press(m, tea.MouseButtonLeft, x, y)
	if once.form != nil {
		t.Fatal("the first click opened the project; it should only select it")
	}
	if twice := press(once, tea.MouseButtonLeft, x, y); twice.form == nil {
		t.Error("a second click on the selected project did not open the new-session form")
	}
	// Any press between the two makes them not a pair.
	between := press(press(once, tea.MouseButtonWheelDown, 60, 0), tea.MouseButtonLeft, x, y)
	if between.form != nil {
		t.Error("a click after a wheel notch opened the project")
	}
	// ↑ then ↓ puts the cursor back on project-01, so only the key tells.
	keyed, _ := once.Update(tea.KeyMsg{Type: tea.KeyUp})
	keyed, _ = keyed.(Model).Update(tea.KeyMsg{Type: tea.KeyDown})
	if keyed.(Model).projectIx != 1 {
		t.Fatalf("the keys left the cursor on project-%02d", keyed.(Model).projectIx)
	}
	if got := press(keyed.(Model), tea.MouseButtonLeft, x, y); got.form != nil {
		t.Error("a click after a key opened the project")
	}

	x, y = cellOf(t, m, "title-00", navW, m.width)
	once = press(m, tea.MouseButtonLeft, x, y)
	if once.screen != screenDashboard {
		t.Fatal("the first click opened the session; it should only select it")
	}
	m = press(once, tea.MouseButtonLeft, x, y)
	if m.screen != screenSession || m.currentSession() == nil || m.currentSession().ID != st.Sessions[0].ID {
		t.Errorf("a second click on the selected session did not open it: screen %v", m.screen)
	}
}

// TestClickOnTheSelectedProjectKeepsTheSessionCursor: clicking the project
// that is already selected, from the sessions list, moves focus and nothing
// else. The session cursor still indexes that project's list.
func TestClickOnTheSelectedProjectKeepsTheSessionCursor(t *testing.T) {
	m := sized(New(mouseState(3, 4), "bash", nil), 120, 30)
	m.focus, m.listIx = colContent, 2
	x, y := cellOf(t, m, "project-00", 0, m.dashboardLayout().navW)
	got := press(m, tea.MouseButtonLeft, x, y)
	if got.focus != colProjects || got.listIx != 2 {
		t.Errorf("focus %v listIx %d, want the projects list with the session cursor kept at 2", got.focus, got.listIx)
	}
}

// TestClickSwitchesTab aims at the boundary between the two tab labels. Each
// label is padded by a cell either side, so the cell just left of "Sessions" is
// its own padding and the cell left of that is Overview's. A click resolved one
// cell off in either direction lands on the wrong tab here and nowhere else.
//
// "Sessions" also appears in the metadata row and the Overview heading, so the
// tab line is found by the label only it carries.
func TestClickSwitchesTab(t *testing.T) {
	m := sized(New(mouseState(2, 3), "bash", nil), 120, 30)
	navW := m.dashboardLayout().navW
	_, tabRow := cellOf(t, m, "Overview", navW, m.width)
	frame := shownFrame(t, m)
	i := strings.Index(frame[tabRow], "Sessions")
	if i < 0 {
		t.Fatalf("no Sessions tab on the tab line %q", frame[tabRow])
	}
	sessionsAt := ansi.StringWidth(frame[tabRow][:i])

	for x, want := range map[int]int{
		sessionsAt - 1:                   1,  // Sessions' own left padding
		sessionsAt + len("Sessions"):     1,  // Sessions' right padding
		sessionsAt - 2:                   0,  // Overview's right padding
		sessionsAt + len("Sessions") + 1: -1, // past the last tab
	} {
		m.tabIx = 1 - max(want, 0) // start on the other tab, so a hit shows
		m.listIx = 2
		got := press(m, tea.MouseButtonLeft, x, tabRow)
		switch {
		case want < 0 && got.tabIx != m.tabIx:
			t.Errorf("a click past the tabs at column %d switched to tab %d", x, got.tabIx)
		case want >= 0 && got.tabIx != want:
			t.Errorf("a click at column %d chose tab %d, want %d", x, got.tabIx, want)
		case want >= 0 && got.focus != colContent:
			t.Errorf("a click on tab %d left focus on the projects", want)
		case want >= 0 && got.listIx != 0:
			t.Errorf("switching to tab %d kept the session cursor at %d", want, got.listIx)
		}
	}
}

// TestWheelMovesTheColumnUnderThePointer: a notch over a column moves that
// column's cursor and focuses it, and a notch over an empty sessions list
// moves nothing and says nothing.
func TestWheelMovesTheColumnUnderThePointer(t *testing.T) {
	m := sized(New(mouseState(5, 3), "bash", nil), 120, 30)
	m.focus = colContent
	navW := m.dashboardLayout().navW
	px, py := cellOf(t, m, "project-03", 0, navW)
	sx, sy := cellOf(t, m, "title-01", navW, m.width)

	got := press(m, tea.MouseButtonWheelDown, px, py)
	if got.focus != colProjects || got.projectIx != 1 {
		t.Errorf("wheel over the projects: focus %v projectIx %d, want projects at 1", got.focus, got.projectIx)
	}

	// project-01 has no sessions, so the right column has nothing to move.
	got = press(got, tea.MouseButtonWheelDown, sx, sy)
	if got.focus != colProjects || got.projectIx != 1 {
		t.Errorf("wheel over project-01's empty list moved something: focus %v projectIx %d", got.focus, got.projectIx)
	}
	if got.notice != "" {
		t.Errorf("wheel over an empty list left a notice: %q", got.notice)
	}

	m.focus = colProjects
	got = press(m, tea.MouseButtonWheelDown, sx, sy)
	if got.focus != colContent || got.listIx != 1 {
		t.Errorf("wheel over the sessions: focus %v listIx %d, want sessions at 1", got.focus, got.listIx)
	}
}

// TestMouseActsOnlyWhereItShould: a release, a drag, the header, the footer,
// a cell off either side of the screen, and anything under a modal all leave
// the dashboard as it was.
func TestMouseActsOnlyWhereItShould(t *testing.T) {
	m := sized(New(mouseState(5, 3), "bash", nil), 120, 30)
	x, y := cellOf(t, m, "project-03", 0, m.dashboardLayout().navW)

	unmoved := func(name string, got Model) {
		t.Helper()
		if got.projectIx != 0 || got.focus != m.focus || got.listIx != m.listIx {
			t.Errorf("%s moved the cursor: projectIx %d focus %v listIx %d", name, got.projectIx, got.focus, got.listIx)
		}
	}
	for name, msg := range map[string]tea.MouseMsg{
		"release":           {X: x, Y: y, Button: tea.MouseButtonLeft, Action: tea.MouseActionRelease},
		"drag":              {X: x, Y: y, Button: tea.MouseButtonLeft, Action: tea.MouseActionMotion},
		"click on header":   {X: x, Y: 0, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress},
		"wheel on header":   {X: x, Y: 0, Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress},
		"wheel on footer":   {X: x, Y: m.height - 1, Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress},
		"wheel left of it":  {X: -1, Y: y, Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress},
		"wheel right of it": {X: m.width, Y: y, Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress},
	} {
		next, _ := m.Update(msg)
		unmoved(name, next.(Model))
	}

	picker, _ := m.openThemePicker()
	help, form, browser := m, m, m
	help.showHelp = true
	form.form = newProjectForm("/code/p00")
	browser.browser = newBrowser(t.TempDir(), 10)
	for name, under := range map[string]Model{
		"picker": picker.(Model), "help": help, "form": form, "browser": browser,
	} {
		unmoved("a click under the "+name, press(under, tea.MouseButtonLeft, x, y))
		unmoved("a wheel notch under the "+name, press(under, tea.MouseButtonWheelDown, x, y))
	}
}

// TestClickCancelsTheArmedPrefix: the prefix arms the next key, and a click
// that left it armed would hand the key after the click to the command table.
func TestClickCancelsTheArmedPrefix(t *testing.T) {
	m := sized(New(mouseState(3, 0), "bash", nil), 120, 30)
	m.armed = true
	x, y := cellOf(t, m, "project-01", 0, m.dashboardLayout().navW)
	if got := press(m, tea.MouseButtonLeft, x, y); got.armed {
		t.Error("the prefix is still armed after a click")
	}
}

// TestAPressCancelsTheArmedPrefixInThePane is the same rule where the prefix
// is used most: over a session, where the key after it is a command.
func TestAPressCancelsTheArmedPrefixInThePane(t *testing.T) {
	m := sized(New(mouseState(1, 1), "bash", nil), 120, 30)
	m.screen, m.armed = screenSession, true
	if got := press(m, tea.MouseButtonWheelUp, 60, 10); got.armed {
		t.Error("the prefix is still armed after a wheel notch over the pane")
	}
}

// TestClickWithARunningSession is the case that exposed the overflow. A
// running session makes the metadata row read "Active · 1 running", wider than
// the column at 80 columns, and the wrapped frame drew every project a row
// higher than a click resolved it. shownFrame fails first if the frame is too tall.
func TestClickWithARunningSession(t *testing.T) {
	st := mouseState(5, 1)
	st.Projects[0].Path = "/code/some/fairly/long/path/to/the/repository"
	st.Sessions[0].Dir = t.TempDir()
	m := sized(New(st, "bash", nil), 80, 24)
	m.runners[st.Sessions[0].ID] = startAgent(t, st.Sessions[0].Dir, "")

	for _, want := range []int{1, 3} {
		x, y := cellOf(t, m, fmt.Sprintf("project-%02d", want), 0, m.dashboardLayout().navW)
		if got := press(m, tea.MouseButtonLeft, x, y); got.projectIx != want {
			t.Errorf("clicked project-%02d, selected project-%02d", want, got.projectIx)
		}
	}
}

// TestWritesToAnExitedPaneAreReported covers the writes a key and the mouse
// make into a pane. The wheel and the literal prefix used to discard the
// error, so writing to an agent that had gone did nothing and said nothing,
// where an ordinary keystroke reported it.
func TestWritesToAnExitedPaneAreReported(t *testing.T) {
	st := mouseState(1, 1)
	st.Sessions[0].Dir = t.TempDir()
	base := sized(New(st, "bash", nil), 120, 30)
	r := startAgent(t, st.Sessions[0].Dir, "exit 0")
	waitExited(t, r)
	base.runners[st.Sessions[0].ID] = r
	base.screen, base.attached = screenSession, true

	literalPrefix := func(m Model) Model {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlG})
		next, _ = next.(Model).Update(tea.KeyMsg{Type: tea.KeyCtrlG})
		return next.(Model)
	}
	for name, act := range map[string]func(Model) Model{
		"wheel up":       func(m Model) Model { return press(m, tea.MouseButtonWheelUp, 60, 10) },
		"wheel down":     func(m Model) Model { return press(m, tea.MouseButtonWheelDown, 60, 10) },
		"literal prefix": literalPrefix,
	} {
		got := act(base)
		if got.fault == nil || !strings.Contains(got.fault.Error(), "send to agent") {
			t.Errorf("%s: fault = %v, want the failed write reported", name, got.fault)
		}
		if got.attached {
			t.Errorf("%s: still attached to a pane that cannot be written", name)
		}
	}
}

// TestDoubleClickInAScrolledListOpensTheRowClicked: the list must not move
// under the pointer between two presses. It used to re-centre on every cursor
// move, so the second press landed on whichever project had scrolled there
// and selected it instead of opening the first.
func TestDoubleClickInAScrolledListOpensTheRowClicked(t *testing.T) {
	m := sized(New(mouseState(40, 0), "bash", nil), 100, 20)
	for range 20 {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = next.(Model)
	}
	x, y := cellOf(t, m, "project-15", 0, m.dashboardLayout().navW)
	once := press(m, tea.MouseButtonLeft, x, y)
	if once.projectIx != 15 {
		t.Fatalf("the first press selected project-%02d", once.projectIx)
	}
	if twice := press(once, tea.MouseButtonLeft, x, y); twice.projectIx != 15 || twice.form == nil {
		t.Errorf("the second press left project-%02d selected and opened nothing", twice.projectIx)
	}
}

// TestAKeyToAModalBreaksThePair: a key is a key wherever it goes. A double
// click opens the new-session form, esc closes it, and the next press is a
// first press again rather than half of a pair that reopens the form.
func TestAKeyToAModalBreaksThePair(t *testing.T) {
	m := sized(New(mouseState(2, 0), "bash", nil), 120, 30)
	x, y := cellOf(t, m, "project-01", 0, m.dashboardLayout().navW)
	opened := press(press(m, tea.MouseButtonLeft, x, y), tea.MouseButtonLeft, x, y)
	if opened.form == nil {
		t.Fatal("the double click did not open the form")
	}
	closed, _ := opened.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if got := press(closed.(Model), tea.MouseButtonLeft, x, y); got.form != nil {
		t.Error("one press after esc reopened the form")
	}
}
