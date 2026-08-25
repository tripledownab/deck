package ui

// Cursor and list navigation: which project, which session, which column. No
// function here starts or stops anything.

import (
	"fmt"

	"github.com/tripledownab/deck/internal/agent"
	"github.com/tripledownab/deck/internal/store"
)

func (m *Model) currentSession() *store.Session {
	if m.rowIx < 0 || m.rowIx >= len(m.rows) {
		return nil
	}
	return m.rows[m.rowIx].session
}

func (m *Model) currentRunner() *agent.Runner {
	s := m.currentSession()
	if s == nil {
		return nil
	}
	return m.runners[s.ID]
}

func (m *Model) currentProject() *store.Project {
	if m.projectIx < 0 || m.projectIx >= len(m.state.Projects) {
		return nil
	}
	return &m.state.Projects[m.projectIx]
}

// moveRow steps the sidebar selection, skipping group headers so the cursor
// only ever lands on a session.
func (m *Model) moveRow(delta int) {
	if len(m.rows) == 0 {
		return
	}
	i := m.rowIx
	for range len(m.rows) {
		i += delta
		if i < 0 || i >= len(m.rows) {
			return // stop at the ends rather than wrapping past a group
		}
		if m.rows[i].session != nil {
			m.landOn(i)
			return
		}
	}
}

// landOn puts the cursor on a session row and keeps the attachment honest.
//
// Staying attached when switching between live agents is the point of running
// several. A session with no process has nothing to type into, so focus drops
// back to the chrome keys there, where ↵ starts it.
func (m *Model) landOn(i int) {
	m.rowIx = i
	if r := m.runners[m.rows[i].session.ID]; r == nil || r.Status() == agent.Exited {
		m.attached = false
	}
}

// jumpToSession moves the cursor to the nth session, counting from 1.
//
// The count is over sessions, not over m.rows: the row list interleaves
// project headers, so the fourth row and the fourth session are different
// things, and the number a person reads off the sidebar is the session one.
func (m *Model) jumpToSession(n int) bool {
	seen := 0
	for i, row := range m.rows {
		if row.session == nil {
			continue
		}
		seen++
		if seen == n {
			m.landOn(i)
			return true
		}
	}
	// Saying how many there are beats a key that silently does nothing.
	switch seen {
	case 0:
		m.notice = "no sessions yet — press n to open one"
	case 1:
		m.notice = "there is only session 1"
	default:
		m.notice = fmt.Sprintf("no session %d — there are %d", n, seen)
	}
	return false
}

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
	p := m.currentProject()
	if p == nil || len(m.state.SessionsFor(p.ID)) == 0 {
		m.notice = "no sessions in this project yet — press n to open one"
		return
	}
	m.focus = colContent
}

// sectionLeft and sectionRight move through the detail column's sections —
// Overview, Sessions — and do nothing at all unless that column has focus.
//
// Scoping them matters because the focused column is drawn with an accent top
// border. Switching sections from the projects list made that border a lie:
// it said the keys drove the left column while ← and → drove the right one.
//
// Neither key moves focus. Changing columns is tab's job, and only tab's: a
// key that sometimes navigates within a column and sometimes jumps between
// them is a second surprise on top of the one being fixed.
func (m *Model) sectionLeft() {
	if m.focus != colContent {
		return
	}
	if m.tabIx > 0 {
		m.tabIx--
		m.listIx = 0
	}
}

func (m *Model) sectionRight() {
	if m.focus != colContent {
		return
	}
	if m.tabIx < len(dashboardTabs)-1 {
		m.tabIx++
		m.listIx = 0
	}
}

func (m *Model) moveDashboard(delta int) {
	if m.focus == colProjects {
		n := len(m.state.Projects)
		if n == 0 {
			return
		}
		m.projectIx = clamp(m.projectIx+delta, 0, n-1)
		m.listIx = 0
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

// rebuildRows flattens projects and their sessions into the sidebar list.
// Only projects that have sessions appear; an empty project is dashboard
// business.
func (m *Model) rebuildRows() {
	m.rows = m.rows[:0]
	for i := range m.state.Projects {
		p := &m.state.Projects[i]
		sessions := m.state.SessionsFor(p.ID)
		if len(sessions) == 0 {
			continue
		}
		m.rows = append(m.rows, sidebarRow{project: p})
		for j := range sessions {
			s := m.state.Session(sessions[j].ID)
			m.rows = append(m.rows, sidebarRow{project: p, session: s})
		}
	}
	if m.rowIx >= len(m.rows) {
		m.rowIx = len(m.rows) - 1
	}
	// Never rest on a group header.
	if m.rowIx >= 0 && m.rowIx < len(m.rows) && m.rows[m.rowIx].session == nil {
		m.moveRow(1)
	}
	if m.rowIx < 0 {
		m.rowIx = 0
	}
}
