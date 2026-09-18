package ui

// The project files a worktree does not get from git, and how a session gets
// them anyway.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tripledownab/deck/internal/gitx"
)

// agentInstructions are the project files a hosted agent reads to learn how to
// work in this repository.
//
// CLAUDE.md and .claude cover claude and cathode, AGENTS.md covers codex —
// the agents ui.agentChoices offers. Covering a new one is adding a name here.
var agentInstructions = []string{"CLAUDE.md", "AGENTS.md", ".claude"}

// linkProjectFiles links the project's agent instructions into a new worktree.
//
// `git worktree add` checks out tracked files only, so anything gitignored
// stays behind — and a project's instructions are commonly gitignored, as this
// repository's own CLAUDE.md is. Without this an isolated session starts with
// no instructions at all and behaves differently from one run in the project
// directory, for a reason nothing on screen explains.
//
// A symlink rather than a copy, so the project keeps one copy of each file. An
// edit made in any session is the edit every sibling reads, which is the whole
// point: a copy would be one more thing to drift. The target is absolute
// because a worktree lives under the state directory, arbitrarily far from the
// project.
//
// A name already present in the worktree is left alone. git put it there, so it
// is tracked and travels on its own.
//
// Only a name the project's ignore rules cover is linked, and an untracked one
// they do not cover is left behind on purpose. The link's target is an absolute
// path on this machine, so a link git can see is a machine path one `git add
// -A` away from a public commit. Being ignored is also what keeps the worktree
// removable: git refuses to remove a tree holding untracked files, and an
// ignored symlink does not count — measured, because the rollback in newSession
// depends on it.
//
// Top-level names only. A .claude that is partly tracked arrives as a real
// directory holding the tracked half, and the ignored files inside it still do
// not travel. Linking into an existing directory is a different job and waits
// for someone who needs it.
func linkProjectFiles(project, worktree string) error {
	var errs []error
	for _, name := range agentInstructions {
		src := filepath.Join(project, name)
		// Stat, not Lstat: a dangling link in the project is not something to
		// copy the dangle of into every session.
		if _, err := os.Stat(src); err != nil {
			continue
		}
		dst := filepath.Join(worktree, name)
		// Lstat, so an existing link counts as present rather than being
		// followed to whatever it points at.
		if _, err := os.Lstat(dst); err == nil {
			continue
		}
		if !gitx.Ignores(project, name) {
			continue
		}
		if err := os.Symlink(src, dst); err != nil {
			errs = append(errs, fmt.Errorf("link %s into the worktree: %w", name, err))
		}
	}
	return errors.Join(errs...)
}
