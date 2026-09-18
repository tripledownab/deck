package gitx

// The worktree an isolated session works in: creating one on a branch of its
// own, and refusing when the project has nothing to branch from or the path is
// already taken.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrNoCommits is returned when a repository has an unborn HEAD — freshly
// initialised, nothing committed. There is no commit for a worktree to check
// out, so an isolated session is impossible until the first commit exists.
var ErrNoCommits = errors.New("repository has no commits yet")

// AddWorktree creates branch at the current HEAD of repo and checks it out
// into dest.
//
// dest must not exist, and that is Deck's rule rather than git's. `git worktree
// add` accepts an existing empty directory, so the os.Stat below is the
// stricter check: it refuses any existing path. Reusing one would put a session
// to work in a tree that something else already owns.
func AddWorktree(repo, dest, branch string) error {
	// A project need not be a repository — it may be a directory that only
	// collects them. Say that plainly rather than letting the unborn-HEAD
	// check below report "no commits yet", which is true of a non-repository
	// and tells the user nothing about what is actually wrong.
	if _, err := RepoRoot(repo); err != nil {
		// Wrapping prepends the sentinel's own text, so this half must not
		// repeat it or the message reads "not a git repository: X is not a
		// git repository".
		return fmt.Errorf("%w: %s has no branch to work from — open the session in the project directory instead",
			ErrNotARepo, filepath.Base(repo))
	}
	// Check for an unborn HEAD next. Without this the caller gets git's own
	// "fatal: invalid reference: HEAD", which is accurate and tells a user
	// nothing about what to do next.
	if !HasCommits(repo) {
		return fmt.Errorf("%w: commit something first, or open the session in the project directory", ErrNoCommits)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("create worktree parent: %w", err)
	}
	if _, err := os.Stat(dest); err == nil {
		return fmt.Errorf("worktree path already exists: %s", dest)
	}
	_, err := run(repo, "worktree", "add", "-b", branch, dest, "HEAD")
	return err
}
