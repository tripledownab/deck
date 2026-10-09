package ui

// Mouse on the dashboard: which column and row a press lands on, and what a
// click or a wheel notch does there. mouse.go routes the press here.

import tea "github.com/charmbracelet/bubbletea"

// dashboardMouse acts on a press over the dashboard. Every press is recorded
// in lastPress, a wheel notch and a miss included, and handleKey clears it, so
// only two presses on one row with nothing between them count as a second
// click.
func (m Model) dashboardMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	col, t, ok := m.dashboardHit(msg.X, msg.Y)
	again := t == m.lastPress
	m.lastPress = target{}
	if !ok {
		return m, nil
	}
	switch msg.Button {
	case tea.MouseButtonLeft:
		m.lastPress = t
		return m.click(t, again)
	case tea.MouseButtonWheelUp:
		m.wheel(col, -1)
	case tea.MouseButtonWheelDown:
		m.wheel(col, 1)
	}
	return m, nil
}

// dashboardHit resolves a cell of the dashboard to the column it is in and the
// target it stands for. The target is hitNone on a heading, a blank, a marker
// or a column's top border. ok is false outside both columns: on the header
// and the footer.
func (m Model) dashboardHit(x, y int) (col column, t target, ok bool) {
	l := m.dashboardLayout()
	row := y - l.top
	if row < -columnBorderRows || row >= l.rows || x < 0 || x >= m.width {
		return 0, target{}, false
	}
	if x < l.navW {
		lines, rows, _ := m.projectListWindow(l.navW, l.rows)
		if row >= 0 && rows[row] >= 0 {
			t = lines[rows[row]].at(x)
		}
		return colProjects, t, true
	}
	lines := m.detailLines(l.detailW)
	if row >= 0 && row < len(lines) {
		t = lines[row].at(x - l.navW - detailIndent)
	}
	return colContent, t, true
}

// click acts on a left click. A click on a row puts the cursor and the focus
// there. A second press in a row on that same row opens it, as ↵ would: the
// first press already put the cursor and the focus on it, and nothing but a
// press or a key can move them in between, and each of those clears or
// replaces lastPress.
//
// Having the cursor is not enough on its own. At launch the first project
// already has the cursor and the focus, and opening it on a first click would
// start an agent from a click meant only to select. No double-click timing is
// involved, so there is none to get wrong.
func (m Model) click(t target, again bool) (tea.Model, tea.Cmd) {
	switch t.kind {
	case hitProject:
		if again {
			return m.openFromDashboard()
		}
		m.focus = colProjects
		if m.projectIx != t.index {
			m.selectProject(t.index)
		}
	case hitSession:
		if again {
			return m.openFromDashboard()
		}
		m.focus, m.listIx = colContent, t.index
	case hitTab:
		if m.tabIx != t.index {
			m.selectTab(t.index)
		}
		m.focusContent()
	}
	return m, nil
}

// wheel moves the cursor in the column under the pointer, as ↑ and ↓ move it in
// the focused one, and focuses that column so the keys then drive the list that
// just moved.
//
// An empty sessions list has nothing to move, so it takes no focus, and says
// nothing either. Unlike a click, a notch is often not aimed, and a notice
// stays in the footer until another replaces it.
func (m *Model) wheel(col column, delta int) {
	if col == colContent && !m.hasSessionList() {
		return
	}
	m.focus = col
	m.moveDashboard(delta)
}
