package ui

// Creating, selecting and closing sessions, and the worktrees behind them.

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

	if isolated {
		root, err := store.WorktreeDir()
		if err != nil {
			m.formProblem(err)
			return m, nil
		}
		dir = filepath.Join(root, naming.Slug(p.Name), name)
		branch = naming.Branch(name)
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

// closeSelectedFromDashboard stops a session's agent and forgets it.
//
// The worktree is left on disk on purpose. It may hold uncommitted work, and
// deleting a branch's only checkout to tidy a list is not a trade Deck
// gets to make silently. The notice says where it went.
func (m *Model) closeSelectedFromDashboard() {
	p := m.currentProject()
	if p == nil {
		return
	}
	sessions := m.state.SessionsFor(p.ID)
	if len(sessions) == 0 || m.listIx >= len(sessions) {
		return
	}
	sess := sessions[m.listIx]
	if r, ok := m.runners[sess.ID]; ok {
		r.Stop()
		delete(m.runners, sess.ID)
	}
	m.releaseCoord(sess.ID)
	m.state.RemoveSession(sess.ID)
	if err := m.state.Save(); err != nil {
		m.fault = err
		return
	}
	m.rebuildRows()
	m.listIx = clamp(m.listIx, 0, max(len(m.state.SessionsFor(p.ID))-1, 0))
	if sess.Isolated {
		m.notice = "closed " + sess.Name + " — worktree kept at " + sess.Dir
	} else {
		m.notice = "closed " + sess.Name
	}
}
