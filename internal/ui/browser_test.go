package ui

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// drain runs a command and feeds its message back to the browser, repeating
// while the browser keeps producing work. The filepicker loads directories
// asynchronously, so without this it never has any files to select.
func drain(b *browser, cmd tea.Cmd) string {
	for range 8 {
		if cmd == nil {
			return ""
		}
		msg := cmd()
		if msg == nil {
			return ""
		}
		var picked string
		cmd, picked = b.update(msg)
		if picked != "" {
			return picked
		}
	}
	return ""
}

// TestBrowserSelectsARepository is the behaviour the explorer exists for:
// highlight a directory, press enter, get its path back.
//
// It is worth a test because the filepicker both records the selection and
// descends into the directory on the same keypress. A caller that does not
// check DidSelectFile on exactly that message sees only the descent.
func TestBrowserSelectsARepository(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "a-repo")
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	b, cmd := newBrowser(root, 10)
	drain(b, cmd)

	// The directory listing has one entry, so the cursor is already on it.
	_, picked := b.update(tea.KeyMsg{Type: tea.KeyEnter})
	if picked == "" {
		t.Fatalf("enter on %q selected nothing; browser is now in %q",
			repo, b.fp.CurrentDirectory)
	}
	if picked != repo {
		t.Errorf("picked %q, want %q", picked, repo)
	}
}

// TestBrowserDescendsWithRight covers the other half of the interaction: right
// navigates into a directory instead of choosing it.
func TestBrowserDescendsWithRight(t *testing.T) {
	root := t.TempDir()
	inner := filepath.Join(root, "outer", "inner")
	if err := os.MkdirAll(inner, 0o755); err != nil {
		t.Fatal(err)
	}

	b, cmd := newBrowser(root, 10)
	drain(b, cmd)

	c, picked := b.update(tea.KeyMsg{Type: tea.KeyRight})
	if picked != "" {
		t.Fatalf("right selected %q, want navigation only", picked)
	}
	drain(b, c)

	if got := b.fp.CurrentDirectory; got != filepath.Join(root, "outer") {
		t.Errorf("after right, directory = %q, want %q", got, filepath.Join(root, "outer"))
	}
}

// TestBrowserEscIsNotBack pins the rebind. The filepicker binds esc to "up one
// directory" by default, which would leave the modal with no way out.
func TestBrowserEscIsNotBack(t *testing.T) {
	b, _ := newBrowser(t.TempDir(), 10)
	for _, k := range b.fp.KeyMap.Back.Keys() {
		if k == "esc" {
			t.Fatal("esc is still bound to Back, so it cannot cancel the modal")
		}
	}
}

// TestBrowserSelectsAfterNavigating reproduces the real interaction: several
// sibling directories, arrow down to the one you want, then enter. The
// filepicker resets its cursor as part of the same keypress that records the
// selection, so this exercises a different path from selecting the first row.
func TestBrowserSelectsAfterNavigating(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"aaa", "bbb", "ccc", "target", "zzz"} {
		if err := os.MkdirAll(filepath.Join(root, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	want := filepath.Join(root, "target")

	b, cmd := newBrowser(root, 10)
	drain(b, cmd)

	for range 3 { // aaa -> bbb -> ccc -> target
		c, picked := b.update(tea.KeyMsg{Type: tea.KeyDown})
		if picked != "" {
			t.Fatalf("arrow down selected %q", picked)
		}
		drain(b, c)
	}

	_, picked := b.update(tea.KeyMsg{Type: tea.KeyEnter})
	if picked != want {
		t.Fatalf("picked %q, want %q (browser now in %q)", picked, want, b.fp.CurrentDirectory)
	}
}
