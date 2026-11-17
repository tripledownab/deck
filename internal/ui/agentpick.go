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
// choice lands on the session the list was opened for. A failed save is
// reported and the agent still starts: the user asked for it, and the session
// in memory says what is running.
func (m Model) agentPicked(choice string) (tea.Model, tea.Cmd) {
	sess := m.state.Session(m.pickerSubject)
	if sess == nil {
		return m, nil
	}
	if sess.Agent != choice {
		sess.Agent = choice
		if err := m.state.Save(); err != nil {
			m.fault = err
		}
	}
	return m.start(sess)
}
