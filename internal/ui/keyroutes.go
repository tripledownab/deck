package ui

// Which handler a key reaches once the modals have declined it: the
// dashboard, the session view, or the directory explorer.

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) dashboardKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.Help):
		m.showHelp = true
	case key.Matches(msg, m.keys.SwitchCol):
		m.toggleColumn()
	case key.Matches(msg, m.keys.Left):
		m.sectionLeft()
	case key.Matches(msg, m.keys.Right):
		m.sectionRight()
	case key.Matches(msg, m.keys.Up):
		m.moveDashboard(-1)
	case key.Matches(msg, m.keys.Down):
		m.moveDashboard(1)
	case key.Matches(msg, m.keys.NewSession):
		return m.openNewSessionForm()
	case key.Matches(msg, m.keys.AddProject):
		return m.openBrowser()
	case key.Matches(msg, m.keys.Theme):
		return m.openThemePicker()
	case key.Matches(msg, m.keys.Delete):
		m.closeSelectedFromDashboard()
	case key.Matches(msg, m.keys.Enter):
		return m.openFromDashboard()
	}
	return m, nil
}

func (m Model) sessionKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, m.keys.Help):
		m.showHelp = true
	case key.Matches(msg, m.keys.Up):
		m.moveRow(-1)
	case key.Matches(msg, m.keys.Down):
		m.moveRow(1)
	case key.Matches(msg, m.keys.Enter):
		return m.attach()
	case key.Matches(msg, m.keys.NewSession):
		return m.openNewSessionForm()
	case key.Matches(msg, m.keys.Delete):
		m.stopCurrent()
	case key.Matches(msg, m.keys.Theme):
		return m.openThemePicker()
	case msg.String() == "d":
		m.screen = screenDashboard
	}
	return m, nil
}

// browserKey routes a key to the explorer, and turns a selection into the
// project form with the path already filled in.
func (m Model) browserKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyEsc {
		m.browser = nil
		return m, nil
	}

	// Where we were before this key. The filepicker descends into a directory
	// on the same keypress that selects it, so a rejected pick would otherwise
	// strand the user inside the directory we just refused.
	from := m.browser.fp.CurrentDirectory

	cmd, picked := m.browser.update(msg)
	if picked == "" {
		return m, cmd
	}

	// Reject a bad pick here rather than in the form. The explorer is still on
	// screen and one keystroke from the right directory, so keeping the user
	// in it beats bouncing them into a form to read the error.
	reject := func(reason string) (tea.Model, tea.Cmd) {
		b, c := newBrowser(from, clamp(m.height-16, 5, 16))
		b.problem = reason
		m.browser = b
		return m, c
	}

	// Any directory may be a project — see resolveProject. Only an
	// unreadable path or an already-registered one is refused.
	root, err := resolveProject(picked)
	if err != nil {
		return reject(err.Error())
	}
	if existing := m.state.ProjectByPath(root); existing != nil {
		return reject(existing.Name + " is already registered")
	}

	m.browser = nil
	m.form = newProjectForm(root)
	return m, cmd
}
