package coord

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// worktreeSession builds a repository with one commit, returns its path and
// HEAD, so a test can register a session that looks like an isolated one.
func worktreeSession(t *testing.T) (dir, head string) {
	t.Helper()
	dir = t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "router.go"), []byte("package gateway\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"config", "user.name", "test"},
		{"config", "user.email", "test@example.test"},
		{"add", "."},
		{"commit", "-q", "-m", "initial"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v: %s", args[0], err, out)
		}
	}
	// Read separately rather than from the loop's last output. Taking it from
	// the loop makes the sha depend on rev-parse staying last in the slice,
	// and a command added after it would leave head empty — surfacing as
	// "no base recorded" and reading like a bug in Work.
	rev := exec.Command("git", "rev-parse", "HEAD")
	rev.Dir = dir
	out, err := rev.Output()
	if err != nil {
		t.Fatalf("rev-parse in %s: %v", dir, err)
	}
	head = strings.TrimSpace(string(out))
	return dir, head
}

// TestWorkReadsASiblingWithoutItsHelp is the point of reading the worktree
// rather than asking the agent: it needs no cooperation and no delivery.
func TestWorkReadsASiblingWithoutItsHelp(t *testing.T) {
	dir, head := worktreeSession(t)
	c := start(t)
	c.Register(Session{ID: "me", ProjectID: "p1", Name: "scheming-hawk-jhgk", Dir: t.TempDir()})
	c.Register(Session{
		ID: "them", ProjectID: "p1", Name: "wily-crane-bbbb", Title: "split the auth middleware",
		Dir: dir, Branch: "session/wily-crane-bbbb", Isolated: true, BaseRef: head,
	})

	if err := os.WriteFile(filepath.Join(dir, "auth.go"), []byte("package gateway\n\nfunc Auth() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := c.Work("me", "wily-crane-bbbb")
	if err != nil {
		t.Fatal(err)
	}
	if got := out["session"]; got != "wily-crane-bbbb" {
		t.Errorf("session = %v", got)
	}
	if !strings.Contains(out["summary"].(string), "auth.go") {
		t.Errorf("summary missing the new file: %v", out["summary"])
	}
	if !strings.Contains(out["patch"].(string), "func Auth()") {
		t.Errorf("patch missing the change: %v", out["patch"])
	}
}

// TestWorkRefusesASharedDirectory is the honest refusal. A session running in
// the project directory has no branch of its own, so a diff there would credit
// it with whatever anyone else had left lying around.
func TestWorkRefusesASharedDirectory(t *testing.T) {
	dir, head := worktreeSession(t)
	c := start(t)
	c.Register(Session{ID: "me", ProjectID: "p1", Name: "scheming-hawk-jhgk", Dir: t.TempDir()})
	c.Register(Session{
		ID: "them", ProjectID: "p1", Name: "wily-crane-bbbb",
		Dir: dir, Isolated: false, BaseRef: head,
	})

	_, err := c.Work("me", "wily-crane-bbbb")
	if err == nil {
		t.Fatal("a shared project directory produced a diff anyway")
	}
	if !strings.Contains(err.Error(), "project directory") {
		t.Errorf("the refusal does not say why: %v", err)
	}
}

// TestWorkStaysInsideTheProject matches how every other tool is scoped.
func TestWorkStaysInsideTheProject(t *testing.T) {
	dir, head := worktreeSession(t)
	c := start(t)
	c.Register(Session{ID: "me", ProjectID: "p1", Name: "scheming-hawk-jhgk", Dir: t.TempDir()})
	c.Register(Session{
		ID: "them", ProjectID: "OTHER", Name: "wily-crane-bbbb",
		Dir: dir, Isolated: true, BaseRef: head,
	})

	if _, err := c.Work("me", "wily-crane-bbbb"); err == nil {
		t.Error("read a session belonging to another project")
	}
}

// TestWorkCannotReadAnExitedSession pins the limit the tool description now
// states. sweepExited removes a session whose process has gone, and Work reads
// the live registry, so a finished session is unreachable even though its
// worktree is still on disk. Written because the description claimed the
// opposite and an agent would have planned around it.
func TestWorkCannotReadAnExitedSession(t *testing.T) {
	dir, head := worktreeSession(t)
	c := start(t)
	c.Register(Session{ID: "me", ProjectID: "p1", Name: "scheming-hawk-jhgk", Dir: t.TempDir()})
	c.Register(Session{
		ID: "them", ProjectID: "p1", Name: "wily-crane-bbbb",
		Dir: dir, Isolated: true, BaseRef: head,
	})

	if _, err := c.Work("me", "wily-crane-bbbb"); err != nil {
		t.Fatalf("the fixture cannot be read at all: %v", err)
	}

	// What the UI does when the agent's process is gone.
	c.Unregister("them")

	if _, err := c.Work("me", "wily-crane-bbbb"); err == nil {
		t.Error("read a session that is no longer registered")
	}
}

// TestWorkIsCallableOverMCP covers the wiring rather than the method. Every
// other test here calls c.Work directly, so the dispatch case and the argument
// tag were both unproven: renaming either left the whole suite green while the
// tool failed for a real agent, and the argument failure is the worse of the
// two — an empty session name comes back as "no live session named \"\"",
// which reads as the agent's own mistake.
func TestWorkIsCallableOverMCP(t *testing.T) {
	dir, head := worktreeSession(t)
	c := start(t)
	c.Register(Session{ID: "s1", ProjectID: "p1", Name: "swift-otter-aaaa", Dir: t.TempDir()})
	c.Register(Session{
		ID: "s2", ProjectID: "p1", Name: "wily-crane-bbbb", Title: "split the auth middleware",
		Dir: dir, Branch: "session/wily-crane-bbbb", Isolated: true, BaseRef: head,
	})
	if err := os.WriteFile(filepath.Join(dir, "auth.go"),
		[]byte("package gateway\n\nfunc Auth() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := rpc(t, c, "s1", "tools/call", map[string]any{
		"name":      "work",
		"arguments": map[string]any{"session": "wily-crane-bbbb"},
	})
	if got.Error != nil {
		t.Fatalf("work: %v", got.Error)
	}
	raw, _ := json.Marshal(got.Result)
	for _, want := range []string{"auth.go", "func Auth()", "wily-crane-bbbb"} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("result is missing %q: %s", want, raw)
		}
	}
}
