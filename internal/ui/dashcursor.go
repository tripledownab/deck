package ui

// The dashboard's cursor: which column has focus, which project, which section
// and which session in it. The moves that carry a rule live here, so the keys
// and the mouse apply them alike: a new project or section puts the session
// cursor back at the top, and an empty session list takes no focus.
// No function here starts or stops anything.

// toggleColumn moves keyboard focus between the project list and the session
// list. It refuses to focus an empty session list, which would look like tab
// doing nothing: there would be no cursor to show and no row for the arrows
// to move.
func (m *Model) toggleColumn() {
	if m.focus == colContent {
		m.focus = colProjects
		return
	}
	m.focusContent()
}

// focusContent moves focus to the session list, refusing when it is empty.
func (m *Model) focusContent() {
	if !m.hasSessionList() {
		m.notice = "no sessions in this project yet — press n to open one"
		return
	}
	m.focus = colContent
}

// hasSessionList reports whether the selected project has a session for the
// session list's cursor to rest on.
func (m *Model) hasSessionList() bool {
	p := m.currentProject()
	return p != nil && len(m.state.SessionsFor(p.ID)) > 0
}

// sectionLeft and sectionRight move through the detail column's sections —
// Overview, Sessions — and do nothing at all unless that column has focus.
//
// Scoping them matters because the focused column is drawn with an accent top
// border. Switching sections from the projects list made that border a lie:
// it said the keys drove the left column while ← and → drove the right one.
//
// Neither key moves focus. Among the keys, changing columns is tab's job
// alone: a key that sometimes navigates within a column and sometimes jumps
// between them is a second surprise on top of the one being fixed. The mouse is
// different: a click or a wheel notch lands in a column, and that column takes
// the focus, unless it is an empty session list.
func (m *Model) sectionLeft() {
	if m.focus != colContent {
		return
	}
	if m.tabIx > 0 {
		m.selectTab(m.tabIx - 1)
	}
}

func (m *Model) sectionRight() {
	if m.focus != colContent {
		return
	}
	if m.tabIx < len(dashboardTabs)-1 {
		m.selectTab(m.tabIx + 1)
	}
}

// selectTab switches the detail column to section i and puts the session
// cursor back at the top.
func (m *Model) selectTab(i int) {
	m.tabIx, m.listIx = i, 0
}

// selectProject puts the project cursor on project i. The session cursor goes
// back to the top, because the list it indexed belongs to the project it left.
func (m *Model) selectProject(i int) {
	m.projectIx, m.listIx = i, 0
}

func (m *Model) moveDashboard(delta int) {
	if m.focus == colProjects {
		n := len(m.state.Projects)
		if n == 0 {
			return
		}
		m.selectProject(clamp(m.projectIx+delta, 0, n-1))
		return
	}
	p := m.currentProject()
	if p == nil {
		return
	}
	n := len(m.state.SessionsFor(p.ID))
	if n == 0 {
		return
	}
	m.listIx = clamp(m.listIx+delta, 0, n-1)
}
