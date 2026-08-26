package ui

// Agent lifecycle: starting a session's process, wiring it to the
// coordination server, keeping its terminal the size of the pane, and
// releasing it when it goes away.

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/tripledownab/deck/internal/agent"
	"github.com/tripledownab/deck/internal/coord"
)

// attach starts the agent if it is not running, then hands it the keyboard.
func (m Model) attach() (tea.Model, tea.Cmd) {
	sess := m.currentSession()
	if sess == nil {
		return m, nil
	}
	if r, ok := m.runners[sess.ID]; ok && r.Status() != agent.Exited {
		m.attached = true
		m.screen = screenSession
		return m, nil
	}

	w, h := m.paneSize()
	r, err := agent.Start(agent.Config{
		Command: sess.Agent,
		Args:    m.agentArgsFor(sess),
		Dir:     sess.Dir,
		Width:   w,
		Height:  h,
	})
	if err != nil {
		m.fault = err
		return m, nil
	}
	m.runners[sess.ID] = r
	m.attached = true
	m.screen = screenSession
	m.notice = "started " + sess.Agent + " in " + sess.Dir

	// Announce the session so its siblings can see it. Registering after the
	// process starts keeps the registry to sessions that actually exist.
	if m.coord != nil {
		m.coord.Register(coord.Session{
			ID: sess.ID, ProjectID: sess.ProjectID,
			Name: sess.Name, Title: sess.Title, Branch: sess.Branch, Dir: sess.Dir,
		})
	}
	return m, nil
}

func (m *Model) stopCurrent() {
	sess := m.currentSession()
	if sess == nil {
		return
	}
	r, ok := m.runners[sess.ID]
	if !ok {
		m.notice = "session is not running"
		return
	}
	r.Stop()
	delete(m.runners, sess.ID)
	m.releaseCoord(sess.ID)
	m.attached = false
	m.notice = "stopped " + sess.Name
}

// releaseCoord drops a session from the coordinator, freeing its claims. A
// claim held by a process that has exited is worse than no claim: the next
// agent believes someone is working there.
func (m *Model) releaseCoord(sessionID string) {
	if m.coord != nil {
		m.coord.Unregister(sessionID)
	}
}

// sweepExited releases sessions whose agent stopped without being told to.
//
// stopCurrent and closeSelectedFromDashboard cover the deliberate paths, but
// an agent can also leave on its own — /exit, a crash, the process being
// killed. Nothing calls back into the UI when that happens, so without this
// sweep the coordinator keeps listing a dead session and holding its claims,
// and every sibling is told a process that no longer exists owns those files.
//
// It runs off the frame tick, which already reads Status for the sidebar.
// Iterating the coordinator's own registry rather than the runner map keeps it
// self-limiting: once released, a session is not seen again, so this is not
// re-doing work twenty times a second.
func (m *Model) sweepExited() {
	if m.coord == nil {
		return
	}
	for _, id := range m.coord.Registered() {
		r, running := m.runners[id]
		if !running || r.Status() == agent.Exited {
			m.coord.Unregister(id)
		}
	}
}

// dropDeadAttachment releases the keyboard when the attached agent has gone.
//
// landOn keeps the attachment honest in one direction — the cursor moves to a
// session with no process. This is the other direction, where the cursor stays
// put and the process leaves. Without it every key is still routed to a dead
// PTY, and the first one to be refused surfaces "send to agent: process has
// exited", which reads as a fault in Deck rather than as the agent quitting.
//
// It runs off the frame tick beside sweepExited, which already reads Status.
func (m *Model) dropDeadAttachment() {
	if !m.attached {
		return
	}
	if r := m.currentRunner(); r == nil || r.Status() == agent.Exited {
		m.attached = false
	}
}

// resizePane keeps the attached agent's terminal the same shape as the pane.
func (m *Model) resizePane() {
	r := m.currentRunner()
	if r == nil {
		return
	}
	w, h := m.paneSize()
	r.Resize(w, h)
}
