package ui

// Mouse routing, and the mouse on the session view. The agent pane takes the
// wheel as arrow keys. The dashboard (dashmouse.go) and the session sidebar
// take clicks and the wheel, and resolve them against the lines their builders
// drew (hittest.go), never against a second copy of the layout.

import tea "github.com/charmbracelet/bubbletea"

// handleMouse routes a mouse event.
//
// Only presses act. A drag and a release arrive as events of their own, and
// acting on them too would take one click as two. A wheel notch arrives as a
// press.
//
// A press cancels an armed prefix, and any number pending behind it. The
// prefix arms the next key, and a click that left it armed would hand whatever
// key came after it to the command table.
//
// A modal takes nothing. It replaces the frame on screen, so a click would act
// on a row the user cannot see and a wheel notch would reach a pane that is
// not showing.
func (m Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if msg.Action != tea.MouseActionPress {
		return m, nil
	}
	m.armed, m.jumpDigits = false, 0
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
