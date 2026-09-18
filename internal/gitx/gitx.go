// Package gitx wraps the handful of git commands Deck needs. It shells
// out to the git binary rather than linking a library: worktree support is the
// whole point of this package, and the CLI is the only implementation that is
// guaranteed to agree with the user's own `git worktree list`.
package gitx

// Running git at all, and the questions asked of a repository before Deck
// commits to anything: where its root is, what it has checked out, whether it
// has a commit yet, and whether a directory is a project or a shelf of them.
// The worktree a session gets is worktree.go.

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ErrNotARepo is returned when a path is not inside a git working tree.
var ErrNotARepo = errors.New("not a git repository")

// run executes git in dir and returns trimmed stdout. git writes its
// diagnostics to stderr, so the error carries them verbatim — a wrapped
// "exit status 128" on its own tells the user nothing.
func run(dir string, args ...string) (string, error) {
	return runEnv(dir, nil, args...)
}

// runEnv is run with extra environment. It exists for GIT_INDEX_FILE, which
// is how Diff stages a worktree without touching the index its owner is using.
func runEnv(dir string, env []string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
	}
	return strings.TrimSpace(stdout.String()), nil
}

// RepoRoot returns the top level of the working tree containing dir.
func RepoRoot(dir string) (string, error) {
	out, err := run(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrNotARepo, dir)
	}
	return out, nil
}

// HeadBranch returns the checked-out branch, or "" on a detached HEAD.
func HeadBranch(dir string) (string, error) {
	out, err := run(dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	if out == "HEAD" {
		return "", nil // detached; not an error, just nothing to name
	}
	return out, nil
}

// HasCommits reports whether HEAD resolves to a commit. A freshly initialised
// repository has an unborn HEAD and answers false.
func HasCommits(dir string) bool {
	_, err := run(dir, "rev-parse", "--verify", "HEAD")
	return err == nil
}

// HoldsRepos reports whether dir directly contains at least one git
// repository — a collector: several checkouts side by side, coordinated from
// the parent.
//
// One level only, so a grandparent full of project directories is not itself
// called a collector.
//
// The home directory is refused outright. Depth alone does not protect it: a
// single checkout sitting directly in $HOME makes it a one-level collector by
// this test, and it is a place you happen to be rather than a project. The
// comment here used to claim the depth limit covered this case; it does not.
func HoldsRepos(dir string) bool {
	if home, err := os.UserHomeDir(); err == nil {
		if a, err1 := filepath.Abs(dir); err1 == nil {
			if b, err2 := filepath.Abs(home); err2 == nil && a == b {
				return false
			}
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		// DirEntry.IsDir reports on the link, not its target, so it is false
		// for every symlink. Testing it alone hid a directory whose children
		// are linked checkouts. The Stat below follows the link and settles
		// what the entry really is, so a plain file or a broken link still
		// fails there.
		if !e.IsDir() && e.Type()&os.ModeSymlink == 0 {
			continue
		}
		if info, err := os.Stat(filepath.Join(dir, e.Name(), ".git")); err == nil && info.IsDir() {
			return true
		}
	}
	return false
}

// HeadCommit returns the commit dir's HEAD points at.
func HeadCommit(dir string) (string, error) {
	return run(dir, "rev-parse", "HEAD")
}
