package ui

// A session whose worktree was deleted by hand: telling that apart from one
// still on disk, and checking its branch out again so the session continues.

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/tripledownab/deck/internal/gitx"
	"github.com/tripledownab/deck/internal/store"
)

// dirGone reports whether dir no longer exists, which is how a worktree deleted
// by hand looks from here. Any other stat error is not taken as gone: the text
// that depends on it then says what it would say for a tree still on disk.
// Lstat, so a dangling link left at the path counts as something there: git
// refuses to add over it, and offering Recreate would only lead to that.
func dirGone(dir string) bool {
	_, err := os.Lstat(dir)
	return errors.Is(err, fs.ErrNotExist)
}

// canRecreate reports whether a session's branch is still there to check out.
// A branch deleted since — merged and pruned, say — leaves nothing to recreate,
// and a row offering it would promise what RestoreWorktree then refuses. A
// failed look leaves the row out and is reported, so its absence has a reason
// on screen; Close and Delete are still offered.
func (m *Model) canRecreate(sess store.Session) bool {
	p := m.state.Project(sess.ProjectID)
	if p == nil || sess.Branch == "" {
		return false
	}
	ok, err := gitx.HasBranch(p.Path, sess.Branch)
	if err != nil {
		m.fault = err
	}
	return ok
}

// recreateWorktree checks a session's branch out again where its worktree was,
// so the session continues rather than ends. The record already names that
// directory and branch, and the agent starts with ↵ as it does for any session
// that is not running.
//
// The project's ignored instruction files are linked again, as newSession links
// them, because a tree without them gives an agent that behaves differently
// for a reason nothing on screen explains. A link that fails is reported and
// the tree stays: unlike newSession there is no record to roll back to, and
// the tree is the thing the user asked for.
func (m *Model) recreateWorktree(sess store.Session) {
	p := m.state.Project(sess.ProjectID)
	if p == nil {
		m.notice = sess.Name + " has no project to recreate its worktree in"
		return
	}
	if err := gitx.RestoreWorktree(p.Path, sess.Dir, sess.Branch); err != nil {
		m.fault = err
		return
	}
	if err := linkProjectFiles(p.Path, sess.Dir); err != nil {
		m.fault = fmt.Errorf("recreated %s, but: %w", sess.Name, err)
		return
	}
	m.notice = "recreated " + sess.Name + " at " + sess.Dir + " — ↵ starts the agent"
}
