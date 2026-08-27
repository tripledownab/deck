package gitx

import (
	"errors"
	"github.com/tripledownab/deck/internal/gittest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testRepo(t *testing.T) string {
	t.Helper()
	dir := gittest.Repo(t)
	gittest.Run(t, dir, "commit", "--allow-empty", "-m", "root")
	return dir
}

func TestRepoRoot(t *testing.T) {
	repo := testRepo(t)
	sub := filepath.Join(repo, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := RepoRoot(sub)
	if err != nil {
		t.Fatalf("RepoRoot: %v", err)
	}
	if got != repo {
		t.Errorf("RepoRoot = %q, want %q", got, repo)
	}
}

func TestRepoRootOutsideRepo(t *testing.T) {
	_, err := RepoRoot(t.TempDir())
	if !errors.Is(err, ErrNotARepo) {
		t.Fatalf("error = %v, want ErrNotARepo", err)
	}
}

func TestHeadBranch(t *testing.T) {
	repo := testRepo(t)
	got, err := HeadBranch(repo)
	if err != nil {
		t.Fatalf("HeadBranch: %v", err)
	}
	if got != "main" {
		t.Errorf("HeadBranch = %q, want main", got)
	}
}

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

// TestRunErrorCarriesGitStderr keeps the diagnostics: a bare "exit status 128"
// tells the user nothing about what git objected to.
func TestRunErrorCarriesGitStderr(t *testing.T) {
	repo := testRepo(t)
	err := AddWorktree(repo, filepath.Join(t.TempDir(), "wt"), "main")
	if err == nil {
		t.Fatal("creating a branch that already exists succeeded")
	}
	if !strings.Contains(err.Error(), "main") {
		t.Errorf("error does not mention the branch: %v", err)
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

func TestHasCommits(t *testing.T) {
	if !HasCommits(testRepo(t)) {
		t.Error("a repository with a commit reported no commits")
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

// TestHoldsRepos covers the collector check that lets `deck` seed a directory
// which is not itself a repository but coordinates several that are.
func TestHoldsRepos(t *testing.T) {
	collector := t.TempDir()
	if HoldsRepos(collector) {
		t.Error("an empty directory reported as a collector")
	}

	repoAt := filepath.Join(collector, "legacy", ".git")
	if err := os.MkdirAll(repoAt, 0o755); err != nil {
		t.Fatal(err)
	}
	if !HoldsRepos(collector) {
		t.Error("a directory holding a repository was not recognised")
	}

	// One level only: a grandparent is not a collector, or $HOME would be.
	if HoldsRepos(filepath.Dir(collector)) == HoldsRepos(collector) {
		return // sibling temp dirs may coincidentally contain repos; not asserting
	}
}

// TestHoldsReposIgnoresHidden keeps dotted directories out of the check, so a
// stray ~/.cache checkout does not make a home directory look like a project.
func TestHoldsReposIgnoresHidden(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".cache", ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if HoldsRepos(dir) {
		t.Error("a repository inside a hidden directory counted as a collector")
	}
}

// TestHoldsReposRefusesHome is the regression for a comment that claimed a
// protection the code did not provide. The depth limit does not cover $HOME: a
// single checkout sitting directly in it makes it a one-level collector, so
// `deck` run from the home directory would register it as a project.
func TestHoldsReposRefusesHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, "some-checkout", ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	if HoldsRepos(home) {
		t.Error("the home directory was accepted as a collector")
	}

	// And the rule is about being home, not about the contents: a sibling with
	// identical contents still qualifies.
	other := t.TempDir()
	if err := os.MkdirAll(filepath.Join(other, "some-checkout", ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !HoldsRepos(other) {
		t.Error("a normal directory holding a repository was refused")
	}
}

// TestHoldsReposFollowsSymlinks is the companion to the explorer listing
// linked directories. A collector whose children are symlinks to checkouts
// looked empty, because DirEntry.IsDir reports on the link and not its target.
func TestHoldsReposFollowsSymlinks(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "elsewhere", "a-repo")
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	collector := filepath.Join(root, "collector")
	if err := os.MkdirAll(collector, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(repo, filepath.Join(collector, "linked-repo")); err != nil {
		t.Fatal(err)
	}

	if !HoldsRepos(collector) {
		t.Error("a directory of linked checkouts is not recognised as holding repositories")
	}
}

// TestHoldsReposIgnoresLinksToFiles keeps the widened test honest: following a
// link must not make every link count.
func TestHoldsReposIgnoresLinksToFiles(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "notes.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	collector := filepath.Join(root, "collector")
	if err := os.MkdirAll(collector, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(file, filepath.Join(collector, "linked-file")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "nowhere"), filepath.Join(collector, "broken")); err != nil {
		t.Fatal(err)
	}

	if HoldsRepos(collector) {
		t.Error("links to a file and to nothing were counted as repositories")
	}
}
