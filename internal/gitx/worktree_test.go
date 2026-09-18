package gitx

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tripledownab/deck/internal/gittest"
)

// session is a repository with one committed file and an isolated session's
// worktree checked out of it, which is where every removal case starts.
//
// The file is committed rather than left absent because the refusal tests need
// something tracked to modify.
func session(t *testing.T) (repo, dest, branch string) {
	t.Helper()
	repo, _ = gittest.RepoWith(t, "a.txt", "one\n")
	dest = filepath.Join(t.TempDir(), "wt")
	branch = "session/scheming-hawk-jhgk"
	if err := AddWorktree(repo, dest, branch); err != nil {
		t.Fatalf("AddWorktree: %v", err)
	}
	return repo, dest, branch
}

// TestAddWorktree covers what a session needs: a checkout of its own, on its
// own branch.
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

// TestRemoveWorktreeAndBranch is the path a deleted session takes when its work
// is committed and merged: the tree goes, the registration goes, the branch
// goes.
func TestRemoveWorktreeAndBranch(t *testing.T) {
	repo, dest, branch := session(t)

	if err := RemoveWorktree(repo, dest); err != nil {
		t.Fatalf("RemoveWorktree: %v", err)
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Errorf("worktree directory survived: stat err = %v", err)
	}
	// The directory going is not the same as git forgetting it. A stale entry
	// keeps the path reserved and shows up in the user's own worktree list.
	if list := gittest.Run(t, repo, "worktree", "list"); strings.Contains(list, dest) {
		t.Errorf("git still lists the worktree:\n%s", list)
	}

	if err := DeleteBranch(repo, branch); err != nil {
		t.Fatalf("DeleteBranch: %v", err)
	}
	if out := gittest.Run(t, repo, "branch", "--list", branch); out != "" {
		t.Errorf("branch --list = %q, want it gone", out)
	}
}

// TestRemoveWorktreeRefusesWork is the promise that nothing is forced. Both
// cases are the same refusal from git, and the untracked one is here because it
// is the one that surprises: an agent that wrote a file and never added it has
// left the tree dirty, with nothing in `git diff` to show for it.
func TestRemoveWorktreeRefusesWork(t *testing.T) {
	for _, tc := range []struct {
		name string
		file string
	}{
		{"modified tracked file", "a.txt"},
		{"untracked file only", "scratch.txt"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, dest, _ := session(t)
			if err := os.WriteFile(filepath.Join(dest, tc.file), []byte("work\n"), 0o644); err != nil {
				t.Fatal(err)
			}

			err := RemoveWorktree(repo, dest)
			if err == nil {
				t.Fatal("RemoveWorktree deleted a tree holding work")
			}
			if _, statErr := os.Stat(dest); statErr != nil {
				t.Errorf("a refused removal still took the directory: %v", statErr)
			}
			// Not asserting that the message names dest: run formats "git
			// <args>: <stderr>" and dest is an argument, so that would hold
			// with the stderr dropped. See TestRunErrorCarriesGitStderr.
			if strings.Contains(err.Error(), "exit status") {
				t.Errorf("refusal does not say what git objected to: %v", err)
			}
		})
	}
}

// TestDeleteBranchRefusesUnmergedWork is the worktree-is-the-gate rule seen from
// the branch side. A session that committed and never merged leaves a clean tree
// that removes fine, and then the branch is the only copy of the work.
func TestDeleteBranchRefusesUnmergedWork(t *testing.T) {
	repo, dest, branch := session(t)
	if err := os.WriteFile(filepath.Join(dest, "b.txt"), []byte("work\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gittest.Run(t, dest, "add", "b.txt")
	gittest.Run(t, dest, "commit", "-q", "-m", "session work")

	if err := RemoveWorktree(repo, dest); err != nil {
		t.Fatalf("a committed session left a dirty tree: %v", err)
	}

	err := DeleteBranch(repo, branch)
	if err == nil {
		t.Fatal("DeleteBranch dropped the only copy of unmerged work")
	}
	// The branch name is already an argument, so asserting on it would hold with
	// the stderr dropped. "not fully merged" is what only git can have said, and
	// it is the sentence the caller turns into the notice.
	if !strings.Contains(err.Error(), "not fully merged") {
		t.Errorf("refusal does not say why the branch was kept: %v", err)
	}
	if out := gittest.Run(t, repo, "branch", "--list", branch); out == "" {
		t.Error("the branch is gone after a refused delete")
	}
}

// TestDeleteBranchRefusesWhileCheckedOut pins the ordering DeleteBranch's doc
// states. Called before RemoveWorktree it fails on every session, merged or not,
// so getting the order wrong would look like a broken delete rather than a rule.
func TestDeleteBranchRefusesWhileCheckedOut(t *testing.T) {
	repo, _, branch := session(t)

	if err := DeleteBranch(repo, branch); err == nil {
		t.Fatal("DeleteBranch deleted a branch a worktree had checked out")
	}
	if out := gittest.Run(t, repo, "branch", "--list", branch); out == "" {
		t.Error("the branch is gone after a refused delete")
	}
}

// TestRemoveWorktreeAfterTheDirectoryIsGone covers the user who deleted the
// tree by hand. git drops the registration and reports success, which is what
// lets a delete finish instead of stranding the session behind a tree that is
// already gone.
func TestRemoveWorktreeAfterTheDirectoryIsGone(t *testing.T) {
	repo, dest, _ := session(t)
	if err := os.RemoveAll(dest); err != nil {
		t.Fatal(err)
	}

	if err := RemoveWorktree(repo, dest); err != nil {
		t.Fatalf("RemoveWorktree on an absent tree: %v", err)
	}
	if list := gittest.Run(t, repo, "worktree", "list"); strings.Contains(list, dest) {
		t.Errorf("git still lists the removed worktree:\n%s", list)
	}
}
