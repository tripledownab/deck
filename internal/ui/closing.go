package ui

// Ending a session, which is two outcomes rather than one: closing forgets the
// record and keeps the worktree, deleting removes the worktree and the branch
// behind it. The modal that asks which is pickerctl.go.

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/tripledownab/deck/internal/gitx"
	"github.com/tripledownab/deck/internal/store"
)

// endSelectedFromDashboard is x on the dashboard.
//
// Only an isolated session is asked the question. One that ran in the project
// directory has no worktree and no branch of its own, so there is nothing for
// Delete to remove and a modal offering it would be offering nothing.
func (m Model) endSelectedFromDashboard() (tea.Model, tea.Cmd) {
	sess := m.focusedSession()
	if sess == nil {
		return m, nil
	}
	if !sess.Isolated {
		m.closeSession(*sess)
		return m, nil
	}
	return m.openEndSessionPicker(sess)
}

// endPicked carries out the choice the modal collected.
//
// It resolves the session from pickerSubject rather than from the cursor: the
// figure on the Delete row was measured for that session when the modal opened,
// and a confirmation that states a fact has to act on the thing it measured.
func (m Model) endPicked(choice string) (tea.Model, tea.Cmd) {
	sess := m.state.Session(m.pickerSubject)
	if sess == nil {
		return m, nil
	}
	if choice == endDelete {
		m.deleteSession(*sess)
	} else {
		m.closeSession(*sess)
	}
	return m, nil
}

// closeSession stops a session's agent and forgets it.
//
// The worktree is left on disk on purpose. It may hold uncommitted work, and
// deleting a branch's only checkout to tidy a list is not a trade Deck
// gets to make silently. The notice says where it went.
func (m *Model) closeSession(sess store.Session) {
	m.stopRunner(sess.ID)
	if !m.forget(sess) {
		return
	}
	if sess.Isolated {
		m.notice = "closed " + sess.Name + " — worktree kept at " + sess.Dir
	} else {
		m.notice = "closed " + sess.Name
	}
}

// deleteSession closes a session and removes the worktree and branch behind it.
//
// Nothing is forced. git refuses a worktree holding modified or untracked
// files, and that refusal is the answer: the session stays, so the user can
// open it again, commit the work, and delete it after.
//
// The worktree is the gate and the branch is best effort. A clean tree whose
// branch holds unmerged commits removes fine and then `branch -d` refuses,
// which is the right outcome — the session goes and the work stays — so the
// notice names the branch that was kept.
func (m *Model) deleteSession(sess store.Session) {
	p := m.state.Project(sess.ProjectID)
	// Guarded here and not only where the modal opens. A non-isolated session's
	// Dir is the project directory itself, so a delete that reached this would
	// aim git at the user's own checkout.
	if p == nil || !sess.Isolated {
		m.notice = sess.Name + " has no worktree of its own to delete"
		return
	}

	// The agent stops before the tree is touched, because git counts a file it
	// is still writing and cannot be asked to wait. A refusal below therefore
	// leaves the session in place with its agent stopped, which Deck already
	// treats as a restart rather than a dead end.
	m.stopRunner(sess.ID)
	if err := gitx.RemoveWorktree(p.Path, sess.Dir); err != nil {
		m.fault = err
		return
	}

	kept := ""
	if sess.Branch != "" {
		if err := gitx.DeleteBranch(p.Path, sess.Branch); err != nil {
			kept = sess.Branch
		}
	}

	if !m.forget(sess) {
		return
	}
	m.notice = "deleted " + sess.Name
	if kept != "" {
		m.notice += " — branch " + kept + " kept, it holds unmerged commits"
	}
}

// forget drops the record and saves, then puts the lists and the cursor back in
// a consistent state. It reports whether the save succeeded.
//
// Callers do their disk work first and call this last, so a worktree that
// refused to go still has a row naming it. The reverse order would leave an
// orphan the user can no longer see, retry or delete through Deck.
//
// This is not itself atomic, and the comment says so rather than implying
// otherwise. RemoveSession runs before Save, so a failed Save leaves the
// session gone from memory and still in state.json, and it returns at the next
// launch. Putting it back is not possible from here: RemoveSession also drops
// the links this session held, and the copy passed in does not carry them.
// Making the pair atomic belongs in store, not in a caller working around it.
func (m *Model) forget(sess store.Session) bool {
	// RemoveSession drops the links this session held, so the coordinator has
	// to be told: releaseCoord frees claims and the inbox, but peers is set
	// wholesale and outlives an agent exiting on purpose.
	m.state.RemoveSession(sess.ID)
	if err := m.state.Save(); err != nil {
		m.fault = err
		return false
	}
	m.syncConnections()
	m.rebuildRows()
	left := m.state.SessionsFor(sess.ProjectID)
	m.listIx = clamp(m.listIx, 0, max(len(left)-1, 0))
	// Ending the last one leaves the cursor in a column with nothing in it —
	// the state focusContent refuses to create, reached from the other side.
	// focusedSession's promise that a refusal says why rests on this: with no
	// sessions there is nothing for it to name, so it would refuse in silence.
	if len(left) == 0 {
		m.focus = colProjects
	}
	return true
}
