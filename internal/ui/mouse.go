package ui

// Mouse routing: where an event lands and what it does there. The agent pane
// takes the wheel as arrow keys. The dashboard and the session sidebar take
// clicks and the wheel, and resolve them against the lines their builders drew
// (hittest.go), never against a second copy of the layout.

import tea "github.com/charmbracelet/bubbletea"

// handleMouse routes a mouse event.
//
// Only presses act. A drag and a release arrive as events of their own, and
// acting on them too would take one click as two. A wheel notch arrives as a
// press.
//
// A press cancels an armed prefix. The prefix arms the next key, and a click
// that left it armed would hand whatever key came after it to the command
// table.
//
// A modal takes nothing. It replaces the frame on screen, so a click would act
// on a row the user cannot see and a wheel notch would reach a pane that is
// not showing.
func (m Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if msg.Action != tea.MouseActionPress {
		return m, nil
	}
	m.armed = false
	if m.picker != nil || m.browser != nil || m.form != nil || m.showHelp {
		return m, nil
	}
	if m.screen == screenSession {
		return m.sessionMouse(msg)
	}
	return m.dashboardMouse(msg)
}

// sessionMouse acts on a press over the session view. Over the sidebar it
// works as the dashboard's lists do: a click selects a session, a second press
// in a row on it opens it, and the wheel moves the cursor. Anywhere else the
// wheel goes to the pane.
//
// Selecting is landOn, as ^g j is, so a click from one live agent to another
// stays attached.
func (m Model) sessionMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	t, over := m.sidebarHit(msg.X, msg.Y)
	again := t == m.lastPress
	m.lastPress = target{}
	if !over {
		m.scrollPane(msg.Button)
		return m, nil
	}
	switch msg.Button {
	case tea.MouseButtonLeft:
		if t.kind != hitRow {
			return m, nil
		}
		m.lastPress = t
		if again {
			return m.attach()
		}
		m.landOn(t.index)
	case tea.MouseButtonWheelUp:
		m.moveRow(-1)
	case tea.MouseButtonWheelDown:
		m.moveRow(1)
	}
	return m, nil
}

// sidebarHit resolves a cell of the session view to the sidebar target under
// it. over is false outside the sidebar, and always when the view is too
// narrow to draw one.
func (m Model) sidebarHit(x, y int) (t target, over bool) {
	sidebarW, bodyH := m.layout()
	row := y - sessionBodyTop
	if x < 0 || x >= sidebarW || row < 0 || row >= bodyH {
		return target{}, false
	}
	lines, shown, _ := m.sidebarWindow(sidebarW, bodyH)
	if i := shown[row]; i >= 0 {
		t = lines[i].at(x)
	}
	return t, true
}

// scrollPane sends a wheel notch to the attached agent as the ↑ or ↓ key. What
// that does is up to the agent. A pane that is not attached is not listening.
func (m *Model) scrollPane(b tea.MouseButton) {
	switch b {
	case tea.MouseButtonWheelUp:
		m.sendToAgent([]byte("\x1b[A"))
	case tea.MouseButtonWheelDown:
		m.sendToAgent([]byte("\x1b[B"))
	}
}

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
