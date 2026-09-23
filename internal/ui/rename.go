package ui

// Renaming what the dashboard lists. Registering a project is projects.go and
// creating a session is sessions.go; this is only the second name either gets.

import (
	"fmt"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
)

// openRenameForm is e on the dashboard. It renames whichever row the cursor is
// on: the session when the sessions list has focus, the project otherwise.
//
// The focused column decides, the way it decides for x and c — a key that
// reaches into the unfocused column makes the accent border a lie. It does not
// go through focusedSession, because that refuses with a notice when the
// projects list has focus, and here that list has something to rename.
//
// A focused but empty sessions list falls back to the project, which is then
// the only name on screen to change and is what e meant before a session had
// one of its own.
func (m Model) openRenameForm() (tea.Model, tea.Cmd) {
	if m.focus == colContent {
		if sess := m.dashboardSession(); sess != nil {
			m.form = editSessionForm(sess)
			return m, nil
		}
	}
	return m.openEditProjectForm()
}

// openEditProjectForm renames the project under the cursor.
func (m Model) openEditProjectForm() (tea.Model, tea.Cmd) {
	p := m.currentProject()
	if p == nil {
		m.notice = "no project to rename"
		return m, nil
	}
	m.form = editProjectForm(p)
	return m, nil
}

// renameProject applies the rename form.
//
// The project is found by id, not by cursor position: the form may have been
// open while the selection moved, and writing to whatever is selected now
// would rename the wrong row.
func (m Model) renameProject(id, name, description string) (tea.Model, tea.Cmd) {
	p := m.state.Project(id)
	if p == nil {
		m.formProblem(fmt.Errorf("that project is no longer registered"))
		return m, nil
	}
	// Same fallback as registering: an empty name is the directory it sits in,
	// so a cleared field cannot leave a blank row in the sidebar.
	if name == "" {
		name = filepath.Base(p.Path)
	}
	p.Name, p.Description = name, description
	if err := m.state.Save(); err != nil {
		m.formProblem(err)
		return m, nil
	}
	m.form = nil
	m.notice = "renamed to " + p.Name
	return m, nil
}

// renameSession applies the session rename form.
//
// By id rather than by cursor position, for the reason renameProject is. The
// sidebar needs no rebuild: its rows hold the store's own session, so the new
// title is on screen the next frame.
//
// The coordinator is told separately because it keeps its own copy of a
// running session, taken when the agent started. Without this an agent asking
// who its siblings are is answered with the old title for the rest of the run,
// and the title is the only part of that answer that says what a sibling is
// doing.
func (m Model) renameSession(id, title string) (tea.Model, tea.Cmd) {
	sess := m.state.Session(id)
	if sess == nil {
		m.formProblem(fmt.Errorf("that session is no longer open"))
		return m, nil
	}
	sess.Title = title
	if err := m.state.Save(); err != nil {
		m.formProblem(err)
		return m, nil
	}
	if m.coord != nil {
		m.coord.Retitle(id, title)
	}
	m.form = nil
	m.notice = "renamed to " + title
	return m, nil
}
