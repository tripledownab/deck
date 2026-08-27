// Package gittest builds throwaway git repositories for tests.
//
// It exists because four packages had grown their own copy of "make a
// repository with one commit", and they had already drifted: one resolved
// symlinks and the others did not, which is the difference that produced two
// registrations of a single directory elsewhere in this repository.
package gittest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Repo creates a git repository in a temporary directory and returns its path.
//
// The path is symlink-resolved. t.TempDir hands back a symlinked path on
// macOS, and git reports resolved ones, so a caller comparing the two without
// this sees a mismatch that has nothing to do with what it is testing.
//
// The identity is set per repository rather than read from the machine, so a
// test that inspects a commit cannot pick up whoever is running it.
func Repo(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve temp dir: %v", err)
	}
	Run(t, dir, "init", "-q", "-b", "main")
	Run(t, dir, "config", "user.name", "test")
	Run(t, dir, "config", "user.email", "test@example.test")
	return dir
}

// RepoWith is Repo plus one committed file, returning the repository and the
// sha of that commit.
func RepoWith(t *testing.T, name, body string) (dir, head string) {
	t.Helper()
	dir = Repo(t)
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	Run(t, dir, "add", ".")
	Run(t, dir, "commit", "-q", "-m", "initial")
	return dir, headCommit(t, dir)
}

// headCommit is the commit dir currently points at. Unexported, and not named
// head, because RepoWith's own return value is called that.
func headCommit(t *testing.T, dir string) string {
	t.Helper()
	return Run(t, dir, "rev-parse", "HEAD")
}

// Run executes git in dir and returns its trimmed output, failing the test on
// error so a caller never has to decide whether a fixture step mattered.
func Run(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s in %s: %v\n%s", strings.Join(args, " "), dir, err, out)
	}
	return strings.TrimSpace(string(out))
}
