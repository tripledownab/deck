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

// TestRunErrorCarriesGitStderr keeps the diagnostics: a bare "exit status 128"
// tells the user nothing about what git objected to.
//
// It asserts on git's own words rather than on the branch name, because run
// formats the message as "git <args>: <stderr>" and the args already contain
// the branch. The earlier assertion was that the message mentioned "main",
// which stayed green with the stderr read replaced by err.Error() — measured,
// not reasoned about. The negative assertion is the discriminating one.
func TestRunErrorCarriesGitStderr(t *testing.T) {
	repo := testRepo(t)
	err := AddWorktree(repo, filepath.Join(t.TempDir(), "wt"), "main")
	if err == nil {
		t.Fatal("creating a branch that already exists succeeded")
	}
	if strings.Contains(err.Error(), "exit status") {
		t.Errorf("error fell back to the exit code: %v", err)
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error does not carry git's complaint: %v", err)
	}
}

func TestHasCommits(t *testing.T) {
	if !HasCommits(testRepo(t)) {
		t.Error("a repository with a commit reported no commits")
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

	// One level only: a grandparent is not a collector, or $HOME would be. The
	// tree is built here rather than read from filepath.Dir(collector), whose
	// contents the test does not control.
	deep := t.TempDir()
	if err := os.MkdirAll(filepath.Join(deep, "mid", "repo", ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if HoldsRepos(deep) {
		t.Error("a repository two levels down made its grandparent a collector")
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
