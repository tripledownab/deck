package ui

// Choosing which agent a session runs when it starts again: the agent list
// as a picker, and starting the one chosen. The session's edit form changes
// the same field ahead of time; formsession.go builds that.

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/tripledownab/deck/internal/store"
)

// openAgentPicker asks which agent to start a session with, on its current
// one so ↵ ↵ restarts what was running. The rows are the Agent field's own
// choices, so the list and the form cannot offer different programs.
func (m Model) openAgentPicker(sess *store.Session) (tea.Model, tea.Cmd) {
	choices := agentField(sess.Agent, "").choices
	rows := make([]pickerRow, 0, len(choices))
	for _, c := range choices {
		rows = append(rows, pickerRow{id: c.value, label: c.label, desc: c.help})
	}
	m.pickerSubject = sess.ID
	m.picker = newPicker(pickAgent, "Start "+sessionLabel(*sess)+" with", rows, sess.Agent)
	return m, nil
}

// agentPicked starts the session with the agent chosen, and keeps the choice
// as the session's agent, so the next start runs it too.
//
// It resolves the session from pickerSubject, as endPicked does, so the
// choice lands on the session the list was opened for. The choice is kept
// only if that agent started: one that is not installed, or a worktree that is
// gone, leaves the session's agent as it was, and start reports why. A save
// that fails after a good start is reported, and the agent keeps running.
func (m Model) agentPicked(choice string) (tea.Model, tea.Cmd) {
	sess := m.state.Session(m.pickerSubject)
	if sess == nil {
		return m, nil
	}
	before, prev := m.runners[sess.ID], sess.Agent
	sess.Agent = choice
	next, cmd := m.start(sess)
	nm := next.(Model)
	if r := nm.runners[sess.ID]; r == nil || r == before {
		sess.Agent = prev
		return nm, cmd
	}
	if choice != prev {
		if err := nm.state.Save(); err != nil {
			nm.fault = err
		}
	}
	return nm, cmd
}
