package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// link makes a symlink at name pointing at target, failing the test rather
// than the assertion if the filesystem refuses.
func link(t *testing.T, target, name string) {
	t.Helper()
	if err := os.Symlink(target, name); err != nil {
		t.Fatalf("symlink %s -> %s: %v", name, target, err)
	}
}

// TestBrowserSelectsARepository is the behaviour the explorer exists for:
// highlight a directory, press enter, get its path back.
func TestBrowserSelectsARepository(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "a-repo")
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	b := newBrowser(root, 10)
	if picked := b.update(tea.KeyMsg{Type: tea.KeyEnter}); picked != repo {
		t.Errorf("picked %q, want %q; explorer is in %q", picked, repo, b.dir())
	}
}

// TestBrowserDescendsWithRight covers the other half of the interaction: right
// navigates into a directory instead of choosing it.
func TestBrowserDescendsWithRight(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "outer", "inner"), 0o755); err != nil {
		t.Fatal(err)
	}

	b := newBrowser(root, 10)
	if picked := b.update(tea.KeyMsg{Type: tea.KeyRight}); picked != "" {
		t.Fatalf("right selected %q, want navigation only", picked)
	}
	if got, want := b.dir(), filepath.Join(root, "outer"); got != want {
		t.Errorf("after right, directory = %q, want %q", got, want)
	}
}

// TestBrowserEscIsNotHandled pins that esc reaches the caller. The explorer
// must not treat it as "up one directory", or the modal has no way out.
func TestBrowserEscIsNotHandled(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	b := newBrowser(filepath.Join(root, "sub"), 10)

	if picked := b.update(tea.KeyMsg{Type: tea.KeyEsc}); picked != "" {
		t.Errorf("esc selected %q", picked)
	}
	if got, want := b.dir(), filepath.Join(root, "sub"); got != want {
		t.Errorf("esc moved the explorer to %q; it must leave navigation alone", got)
	}
}

// TestBrowserSelectsAfterNavigating reproduces the real interaction: several
// sibling directories, arrow down to the one you want, then enter.
func TestBrowserSelectsAfterNavigating(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"aaa", "bbb", "ccc", "target", "zzz"} {
		if err := os.MkdirAll(filepath.Join(root, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	want := filepath.Join(root, "target")

	b := newBrowser(root, 10)
	for range 3 { // aaa -> bbb -> ccc -> target
		if picked := b.update(tea.KeyMsg{Type: tea.KeyDown}); picked != "" {
			t.Fatalf("arrow down selected %q", picked)
		}
	}
	if picked := b.update(tea.KeyMsg{Type: tea.KeyEnter}); picked != want {
		t.Fatalf("picked %q, want %q (explorer in %q)", picked, want, b.dir())
	}
}

// TestBrowserListsOnlyDirectories is the reason the explorer stopped wrapping
// bubbles/filepicker. A project is a directory, so a file is a row that can be
// neither entered nor chosen.
func TestBrowserListsOnlyDirectories(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "a-dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a-file.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	d := readDirList(root, 10)
	if len(d.rows) != 1 || d.rows[0].name != "a-dir" {
		t.Fatalf("rows = %v, want just a-dir", names(d))
	}
}

// TestBrowserListsSymlinkedDirectories is the bug this replaced the filepicker
// for. DirEntry.IsDir reports on the link rather than its target, so a linked
// directory sorted below every real one and read as absent.
func TestBrowserListsSymlinkedDirectories(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "zzz-target")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "mmm-plain"), 0o755); err != nil {
		t.Fatal(err)
	}
	link(t, target, filepath.Join(root, "aaa-linked"))

	d := readDirList(root, 10)
	got := names(d)
	// One alphabetical run: a linked directory is a directory, so it sorts
	// with them rather than after them.
	want := "aaa-linked mmm-plain zzz-target"
	if got != want {
		t.Fatalf("rows = %q, want %q", got, want)
	}
	// EvalSymlinks, not the raw path: on macOS t.TempDir() is itself reached
	// through a link, so the resolved target differs from the string we made.
	resolved, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatal(err)
	}
	if d.rows[0].target != resolved {
		t.Errorf("linked row target = %q, want %q", d.rows[0].target, resolved)
	}
	if d.rows[1].target != "" {
		t.Errorf("plain directory reported a link target %q", d.rows[1].target)
	}
}

// TestBrowserSkipsLinksThatAreNotDirectories keeps the filter honest: a link
// to a file, and a link to nothing, are both unusable as projects.
func TestBrowserSkipsLinksThatAreNotDirectories(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "real"), 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "real-file.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	link(t, file, filepath.Join(root, "link-to-file"))
	link(t, filepath.Join(root, "nowhere"), filepath.Join(root, "broken-link"))

	if got := names(readDirList(root, 10)); got != "real" {
		t.Errorf("rows = %q, want just real", got)
	}
}

// TestBrowserDescendsThroughASymlink checks that entering a link lands in the
// target rather than in a path that only exists as a pointer.
func TestBrowserDescendsThroughASymlink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	if err := os.MkdirAll(filepath.Join(target, "inside"), 0o755); err != nil {
		t.Fatal(err)
	}
	link(t, target, filepath.Join(root, "aaa-link"))

	b := newBrowser(root, 10)
	b.update(tea.KeyMsg{Type: tea.KeyRight})

	resolved, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatal(err)
	}
	if b.dir() != resolved {
		t.Errorf("descended into %q, want the target %q", b.dir(), resolved)
	}
	if got := names(b.list); got != "inside" {
		t.Errorf("after descending, rows = %q, want inside", got)
	}
}

// TestBrowserMarksSymlinksInADifferentColour pins the styling apart from the
// listing: the two are separate asks and a regression in either is silent.
func TestBrowserMarksSymlinksInADifferentColour(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "zzz-target")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "mmm-plain"), 0o755); err != nil {
		t.Fatal(err)
	}
	link(t, target, filepath.Join(root, "nnn-linked"))

	// Force 24-bit colour. Under the profile a test process gets by default
	// every style renders as bare text, and an assertion about colour would
	// pass whatever the code did.
	restore := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(restore)

	s := newStyles(paletteFor(defaultThemeID))
	// The cursor sits on the first row, which is the plain one, so the colour
	// under test is the linked row's own rather than the cursor's.
	out := readDirList(root, 10).view(s, 60)

	linkColour := string(paletteFor(defaultThemeID).Link)
	if !strings.Contains(out, ansiOf(s.Link, "nnn-linked")) {
		t.Errorf("linked row is not rendered in the Link colour %s:\n%s", linkColour, out)
	}
	if strings.Contains(out, ansiOf(s.Link, "mmm-plain")) {
		t.Errorf("plain directory is rendered in the Link colour:\n%s", out)
	}
}

// names joins the row names, which makes an ordering failure readable.
func names(d *dirList) string {
	out := make([]string, 0, len(d.rows))
	for _, r := range d.rows {
		out = append(out, r.name)
	}
	return strings.Join(out, " ")
}

// ansiOf renders text in one style, so a test can look for that exact styled
// run rather than for a bare colour code that another role might share.
func ansiOf(style lipgloss.Style, text string) string { return style.Render(text) }
