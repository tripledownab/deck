package ui

// The sidebar's cursor: which row of the session list, and the project and
// session the cursors resolve to. The dashboard's own cursor is dashcursor.go.
// No function here starts or stops anything.

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
