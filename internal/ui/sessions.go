package ui

// Creating a session and the worktree behind it, and resolving which session
// the dashboard cursor is on. Ending one is closing.go.

import (
	"fmt"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tripledownab/deck/internal/gitx"
	"github.com/tripledownab/deck/internal/naming"
	"github.com/tripledownab/deck/internal/store"
)

// newSession creates the session record, its worktree when isolated, and
// starts the agent.
//
// Every step that can fail reports and stops. In particular a failed worktree
// is never quietly downgraded to running in the project directory: that would
// put an agent to work in a tree the user believed was untouched.
func (m Model) newSession(projectID, title string, isolated bool, agent string) (tea.Model, tea.Cmd) {
	p := m.state.Project(projectID)
	if p == nil {
		m.formProblem(fmt.Errorf("project %s is gone", projectID))
		return m, nil
	}

	name := naming.Session()
	dir := p.Path
	branch := ""
	baseRef := ""

	if isolated {
		root, err := store.WorktreeDir()
		if err != nil {
			m.formProblem(err)
			return m, nil
		}
		dir = filepath.Join(root, naming.Slug(p.Name), name)
		branch = naming.Branch(name)
		// The commit the worktree starts at, which is what this session's work
		// is later measured against. AddWorktree branches from the same HEAD
		// and leaves the project's own HEAD alone, so reading it here or after
		// makes no difference.
		baseRef, err = gitx.HeadCommit(p.Path)
		if err != nil {
			m.formProblem(err)
			return m, nil
		}
		if err := gitx.AddWorktree(p.Path, dir, branch); err != nil {
			m.formProblem(err)
			return m, nil
		}
	}

	sess := m.state.AddSession(store.Session{
		ProjectID: p.ID,
		Title:     title,
		Name:      name,
		Branch:    branch,
		BaseRef:   baseRef,
		Dir:       dir,
		Isolated:  isolated,
		Agent:     agent,
	})
	if err := m.state.Save(); err != nil {
		m.formProblem(err)
		return m, nil
	}
	// The choice sticks: this session runs it, and the next form opens on it.
	m.agentCmd = agent
	if m.settings.Agent != agent {
		m.settings.Agent = agent
		if err := m.settings.Save(); err != nil {
			m.formProblem(err)
			return m, nil
		}
	}

	m.form = nil
	m.rebuildRows()
	m.selectSession(sess.ID)
	m.screen = screenSession
	return m.attach()
}

// selectSession puts the cursor on a session by id. Its sibling
// jumpToSession does the same by ordinal; both land through landOn, so the
// rule about what happens to the attachment lives in one place.
func (m *Model) selectSession(id string) {
	for i, row := range m.rows {
		if row.session != nil && row.session.ID == id {
			m.landOn(i)
			return
		}
	}
}

// openFromDashboard opens the highlighted project's selected session, or the
// new-session form when the project has none yet.
func (m Model) openFromDashboard() (tea.Model, tea.Cmd) {
	p := m.currentProject()
	if p == nil {
		return m, nil
	}
	sessions := m.state.SessionsFor(p.ID)
	if len(sessions) == 0 {
		return m.openNewSessionForm()
	}
	if m.focus == colProjects || dashboardTabs[m.tabIx] == "Overview" {
		m.listIx = clamp(m.listIx, 0, len(sessions)-1)
	}
	m.rebuildRows()
	m.selectSession(sessions[clamp(m.listIx, 0, len(sessions)-1)].ID)
	m.screen = screenSession
	return m.attach()
}

// dashboardSession is the session the dashboard cursor is on, or nil.
//
// The dashboard resolves a session from the highlighted project and the list
// index, which is a different rule from the session view's cursor.
//
// openFromDashboard deliberately does not use this. It *clamps* listIx into
// range and writes it back, where this *guards* and answers nil, and the two
// disagree exactly when the index is past the end: opening lands on the last
// session, while closing and connecting do nothing. That is the right split —
// opening a session the cursor is near is helpful, closing or connecting one
// the user cannot see is not — so do not fold them together.
func (m Model) dashboardSession() *store.Session {
	p := m.currentProject()
	if p == nil {
		return nil
	}
	sessions := m.state.SessionsFor(p.ID)
	if len(sessions) == 0 || m.listIx >= len(sessions) {
		return nil
	}
	return m.state.Session(sessions[m.listIx].ID)
}
