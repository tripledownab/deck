package gitx

// The worktree an isolated session works in: creating one on a branch of its
// own, removing both when the session is deleted, and the refusals on either
// side that Deck reports rather than forces.

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

// RemoveWorktree removes dest from repo. It never forces.
//
// git refuses a tree holding modified or untracked files, and that refusal is
// the answer rather than an obstacle to work around: the tree is where the
// session's work is. Untracked counts, which is the case that catches people —
// an agent that wrote a file and never added it leaves the tree dirty by this
// test. git's message names the path and says --force exists, so the user has
// the way out without Deck offering it as a button.
//
// A dest the user already deleted by hand is not an error. git drops the
// administrative entry and reports success.
func RemoveWorktree(repo, dest string) error {
	_, err := run(repo, "worktree", "remove", dest)
	return err
}

// DeleteBranch deletes branch from repo. It never forces.
//
// Call it after RemoveWorktree, never before: git refuses to delete a branch
// that a worktree still has checked out, so the reverse order fails on every
// session rather than on the ones worth refusing.
//
// -d refuses a branch whose commits are not merged. git measures against the
// branch's upstream, or against HEAD when there is none, and `worktree add -b`
// off a local HEAD sets none — so for a session branch it is HEAD, unless the
// user has configured git to set tracking on every new branch.
//
// That refusal is the case where the session goes and its work stays, so the
// caller keeps the branch and says so. A session that committed nothing sits on
// an ancestor of HEAD and deletes without complaint.
func DeleteBranch(repo, branch string) error {
	_, err := run(repo, "branch", "-d", branch)
	return err
}
