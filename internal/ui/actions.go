package ui

// Creating and removing the things the dashboard lists: projects, sessions,
// and the worktrees behind them. The agent process itself lives in runner.go.

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/tripledownab/deck/internal/gitx"
)

func (m Model) openNewSessionForm() (tea.Model, tea.Cmd) {
	if len(m.state.Projects) == 0 {
		m.fault = fmt.Errorf("no projects yet: add one with `a` on the dashboard")
		return m, nil
	}

	// Which project the form opens on. In the session view that is the current
	// session's project; on the dashboard it is the highlighted one. Either
	// way it is only the default — the field can be changed.
	var want string
	if m.screen == screenSession {
		if sess := m.currentSession(); sess != nil {
			want = sess.ProjectID
		}
	}
	if want == "" {
		if p := m.currentProject(); p != nil {
			want = p.ID
		}
	}

	choices := make([]choice, 0, len(m.state.Projects))
	selected := 0
	for i := range m.state.Projects {
		p := &m.state.Projects[i]
		if p.ID == want {
			selected = i
		}
		choices = append(choices, choice{label: p.Name, value: p.ID, help: p.Path})
	}

	// Asked of git rather than remembered on the project, so a directory that
	// becomes a repository — or stops being one — is never described from a
	// stale flag. It is one `git rev-parse` per form open.
	canWorktree := false
	if p := m.state.Project(m.state.Projects[selected].ID); p != nil {
		_, err := gitx.RepoRoot(p.Path)
		canWorktree = err == nil
	}

	m.form = newSessionForm(choices, selected, canWorktree, m.agentCmd)
	return m, nil
}

// commitForm applies the open form. The form is dismissed by the handler on
// success only — a failure leaves it up, carrying its reason.
func (m Model) commitForm() (tea.Model, tea.Cmd) {
	f := m.form
	switch f.kind {
	case formAddProject:
		return m.addProject(f.fields[0].value(), f.fields[1].value())
	case formNewSession:
		return m.newSession(
			f.fields[sessionFieldProject].value(),
			f.fields[sessionFieldTitle].value(),
			f.fields[sessionFieldWorkingCopy].value() == "worktree",
			f.fields[sessionFieldAgent].value(),
		)
	}
	m.form = nil
	return m, nil
}

// formProblem reports a failure without closing the form.
//
// Dismissing it would throw away what the user typed and hide the choice that
// caused the failure. Opening a session in a repository with no commits is the
// worked example: the fix is to pick the other working copy, which is only one
// keystroke away as long as the form is still on screen.
func (m *Model) formProblem(err error) {
	if m.form == nil {
		m.fault = err
		return
	}
	m.form.problem = err.Error()
}
