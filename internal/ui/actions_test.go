package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func repoAt(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-b", "main"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

// TestResolveProjectAcceptsACollector is the point of allowing non-repository
// projects: a directory whose children are the repositories, coordinated from
// the parent. An agent run there sees all of them at once.
func TestResolveProjectAcceptsACollector(t *testing.T) {
	root := t.TempDir()
	collector := filepath.Join(root, "Collector")
	repoAt(t, filepath.Join(collector, "legacy"))
	repoAt(t, filepath.Join(collector, "quantum"))

	got, err := resolveProject(collector)
	if err != nil {
		t.Fatalf("resolveProject(collector): %v", err)
	}
	resolved, _ := filepath.EvalSymlinks(collector)
	if got != resolved {
		t.Errorf("resolved to %q, want the collector itself at %q", got, resolved)
	}
}

// TestResolveProjectCollapsesToRepoRoot keeps one repository from being
// registered twice through two of its own subdirectories.
func TestResolveProjectCollapsesToRepoRoot(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repoAt(t, root)
	sub := filepath.Join(root, "internal", "deep")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := resolveProject(sub)
	if err != nil {
		t.Fatalf("resolveProject(sub): %v", err)
	}
	if got != root {
		t.Errorf("resolved to %q, want the repo root %q", got, root)
	}
}

func TestResolveProjectRejectsNonDirectories(t *testing.T) {
	file := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveProject(file); err == nil {
		t.Error("a file was accepted as a project")
	}
	if _, err := resolveProject(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Error("a missing path was accepted as a project")
	}
}

// TestFormDefaultsToDirectoryForACollector covers the form side: with no
// repository at the project root there is no branch to work from, so the
// isolated worktree must not be the default.
func TestFormDefaultsToDirectoryForACollector(t *testing.T) {
	f := newSessionForm([]choice{{label: "collector", value: "p1"}}, 0, false, "claude")
	if got := f.fields[sessionFieldWorkingCopy].value(); got != "cwd" {
		t.Errorf("default working copy = %q, want cwd for a non-repository", got)
	}
	// The choice remains available, because the project field can change.
	if n := len(f.fields[sessionFieldWorkingCopy].choices); n != 2 {
		t.Errorf("working-copy choices = %d, want both still offered", n)
	}
}
