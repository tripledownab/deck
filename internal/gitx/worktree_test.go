package gitx

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tripledownab/deck/internal/gittest"
)

// TestAddWorktree covers what a session needs: a checkout of its own, on its
// own branch.
//
// There is no removal half. Deck deliberately leaves a worktree on disk
// when a session closes, because it may hold uncommitted work — so there is no
// remove function to test, and adding one for the test's sake would be code
// with no caller.
func TestAddWorktree(t *testing.T) {
	repo := testRepo(t)
	dest := filepath.Join(t.TempDir(), "worktrees", "scheming-hawk-jhgk")
	const branch = "session/scheming-hawk-jhgk"

	if err := AddWorktree(repo, dest, branch); err != nil {
		t.Fatalf("AddWorktree: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, ".git")); err != nil {
		t.Fatalf("worktree has no .git: %v", err)
	}
	got, err := HeadBranch(dest)
	if err != nil {
		t.Fatalf("HeadBranch in worktree: %v", err)
	}
	if got != branch {
		t.Errorf("worktree branch = %q, want %q", got, branch)
	}
}

// TestAddWorktreeRefusesExistingPath matters because silently reusing a
// populated directory would put an agent to work in someone else's tree.
func TestAddWorktreeRefusesExistingPath(t *testing.T) {
	repo := testRepo(t)
	dest := filepath.Join(t.TempDir(), "taken")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := AddWorktree(repo, dest, "session/x"); err == nil {
		t.Fatal("AddWorktree overwrote an existing path")
	}
}

// TestAddWorktreeOnUnbornHead covers a freshly `git init`-ed repository. git's
// own message is "fatal: invalid reference: HEAD", which is accurate and tells
// a user nothing about what to do next.
func TestAddWorktreeOnUnbornHead(t *testing.T) {
	// gittest.Repo initialises without committing, which is the unborn HEAD
	// this test is about.
	dir := gittest.Repo(t)

	if HasCommits(dir) {
		t.Fatal("a repository with no commits reported HasCommits")
	}

	err := AddWorktree(dir, filepath.Join(t.TempDir(), "wt"), "session/x")
	if !errors.Is(err, ErrNoCommits) {
		t.Fatalf("error = %v, want ErrNoCommits", err)
	}
	if !strings.Contains(err.Error(), "project directory") {
		t.Errorf("error does not suggest the way out: %v", err)
	}
}

// TestAddWorktreeOnNonRepo covers a project that is a collector of
// repositories rather than one itself. Without the explicit check the
// unborn-HEAD branch reports "no commits yet", which is true of a
// non-repository and explains nothing.
func TestAddWorktreeOnNonRepo(t *testing.T) {
	plain := t.TempDir()

	err := AddWorktree(plain, filepath.Join(t.TempDir(), "wt"), "session/x")
	if !errors.Is(err, ErrNotARepo) {
		t.Fatalf("error = %v, want ErrNotARepo", err)
	}
	if strings.Contains(err.Error(), "no commits") {
		t.Errorf("a non-repository was reported as having no commits: %v", err)
	}
	if !strings.Contains(err.Error(), "project directory") {
		t.Errorf("error does not suggest the way out: %v", err)
	}
}
