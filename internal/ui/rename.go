package ui

// Renaming what the dashboard lists. Registering a project is projects.go and
// creating a session is sessions.go; this is only the second name either gets.

import (
	"fmt"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
)

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
