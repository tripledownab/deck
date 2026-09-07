package ui

// Ending a session: stopping its agent, forgetting the record, and what is
// deliberately left on disk behind it.

// closeSelectedFromDashboard stops a session's agent and forgets it.
//
// The worktree is left on disk on purpose. It may hold uncommitted work, and
// deleting a branch's only checkout to tidy a list is not a trade Deck
// gets to make silently. The notice says where it went.
func (m *Model) closeSelectedFromDashboard() {
	p := m.currentProject()
	selected := m.dashboardSession()
	if p == nil || selected == nil {
		return
	}
	sess := *selected
	if r, ok := m.runners[sess.ID]; ok {
		r.Stop()
		delete(m.runners, sess.ID)
	}
	m.releaseCoord(sess.ID)
	// RemoveSession drops the links this session held, so the coordinator has
	// to be told: releaseCoord frees claims and the inbox, but peers is set
	// wholesale and outlives an agent exiting on purpose.
	m.state.RemoveSession(sess.ID)
	if err := m.state.Save(); err != nil {
		m.fault = err
		return
	}
	m.syncConnections()
	m.rebuildRows()
	m.listIx = clamp(m.listIx, 0, max(len(m.state.SessionsFor(p.ID))-1, 0))
	if sess.Isolated {
		m.notice = "closed " + sess.Name + " — worktree kept at " + sess.Dir
	} else {
		m.notice = "closed " + sess.Name
	}
}
