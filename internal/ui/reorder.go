package ui

// Reordering sessions within their project: the keys that move the selected
// one a step, and dragging one with the mouse. The order itself is the store's
// (store.State.MoveSession); this file decides which session moves where,
// saves, and keeps the cursors on the sessions they were on.

import (
	"slices"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tripledownab/deck/internal/store"
)

// moveSelected is K and J, and shift+↑ and shift+↓: it moves the selected
// session one step within its project. On the dashboard that is the session
// under the list's cursor, and in the session view the sidebar's.
func (m Model) moveSelected(delta int) (tea.Model, tea.Cmd) {
	var sess *store.Session
	if m.screen == screenDashboard {
		sess = m.focusedSession()
	} else {
		sess = m.currentSession()
	}
	if sess == nil {
		return m, nil
	}
	name := sess.Name
	if !m.moveSession(sess.ID, positionOf(m.state, sess.ID)+delta) {
		end := "top"
		if delta > 0 {
			end = "bottom"
		}
		m.notice = name + " is already at the " + end + " of " + projectLabel(m.state, sess.ProjectID)
	}
	return m, nil
}

// moveSession puts a session at position to in its project's list, saves, and
// reports whether it moved.
//
// Both cursors stay on the sessions they were on, rather than on the row index
// they held: the rows under an index change when the order does, and a cursor
// that stayed put would now select a different session.
//
// A failed save is reported and the move stays on screen. The list in front of
// the user is the one in memory, and the next save writes it.
func (m *Model) moveSession(id string, to int) bool {
	onRow, onList := idOf(m.currentSession()), idOf(m.dashboardSession())
	if !m.state.MoveSession(id, to) {
		return false
	}
	if err := m.state.Save(); err != nil {
		m.fault = err
	}
	m.rebuildRows()
	if i := slices.IndexFunc(m.rows, func(r sidebarRow) bool {
		return r.session != nil && r.session.ID == onRow
	}); i >= 0 {
		m.rowIx = i
	}
	if onList != "" {
		m.listIx = positionOf(m.state, onList)
	}
	return true
}

// dragMouse carries a drag that a press on a session row started. Motion with
// the left button held moves the session to the row under the pointer, if that
// row is in the same project; a release ends the drag.
//
// Every step is saved as it happens rather than once at the release, because
// a release is not guaranteed to arrive: a button let go outside the window
// may send none. Any key or press ends a drag (handleKey, handleMouse), so
// nothing that changes the rows on screen leaves one running.
//
// The press that started the drag no longer counts toward a double click once
// the session moves. It was recorded against a row index, and that index now
// holds another session, which a second press would otherwise open.
func (m Model) dragMouse(msg tea.MouseMsg) Model {
	if m.dragging == "" {
		return m
	}
	if msg.Action == tea.MouseActionRelease {
		m.dragging = ""
		return m
	}
	if msg.Action != tea.MouseActionMotion || msg.Button != tea.MouseButtonLeft {
		return m
	}
	if to, ok := m.dropTarget(msg.X, msg.Y); ok && m.moveSession(m.dragging, to) {
		m.lastPress = target{}
	}
	return m
}

// dropTarget is the position in the dragged session's project list that the
// cell at x, y stands for. ok is false over anything but a session of that
// project: a session never moves between projects, because its worktree
// belongs to the repository it was opened in.
//
// The dragged session can be gone: a key that ends it also ends the drag, but
// the store is the authority on whether it still exists, so it is asked.
func (m Model) dropTarget(x, y int) (to int, ok bool) {
	sess := m.state.Session(m.dragging)
	if sess == nil {
		return 0, false
	}
	if m.screen == screenSession {
		t, over := m.sidebarHit(x, y)
		if !over || t.kind != hitRow {
			return 0, false
		}
		under := m.rows[t.index].session
		if under == nil || under.ProjectID != sess.ProjectID {
			return 0, false
		}
		return positionOf(m.state, under.ID), true
	}
	_, t, hit := m.dashboardHit(x, y)
	p := m.currentProject()
	if !hit || t.kind != hitSession || p == nil || p.ID != sess.ProjectID {
		return 0, false
	}
	return t.index, true
}

// positionOf is a session's index in its project's list, or -1.
func positionOf(state *store.State, id string) int {
	sess := state.Session(id)
	if sess == nil {
		return -1
	}
	return slices.IndexFunc(state.SessionsFor(sess.ProjectID), func(s store.Session) bool {
		return s.ID == id
	})
}

// idOf is a session's id, or "" for none.
func idOf(sess *store.Session) string {
	if sess == nil {
		return ""
	}
	return sess.ID
}
