package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/creack/pty"

	"github.com/tripledownab/deck/internal/termquery"
)

// TestSmoke drives the real binary through a pseudo-terminal: register a
// repository, open an isolated session, and confirm the agent process is live
// in its own git worktree.
//
// The agent under test is bash, not claude. The point is the frame Deck
// puts around a process, and bash exercises the same PTY path for free.
func TestSmoke(t *testing.T) {
	bin := buildBinary(t)
	repo := initRepo(t)
	stateDir := t.TempDir()

	cmd := exec.Command(bin, "-agent", "bash", "-agent-args", "--norc --noprofile")
	cmd.Dir = repo
	cmd.Env = append(os.Environ(),
		"XDG_STATE_HOME="+stateDir,
		"TERM=xterm-256color",
		"PS1=bash$ ",
	)

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 40, Cols: 120})
	if err != nil {
		t.Fatalf("start deck: %v", err)
	}
	screen := newScreen(ptmx)
	defer func() {
		_ = ptmx.Close()
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}()

	// The cwd repository is seeded on a first run, so the dashboard should
	// come up already pointed at it.
	screen.await(t, "PROJECTS", 5*time.Second)
	screen.await(t, filepath.Base(repo), 5*time.Second)

	// n opens the new-session form.
	screen.send(t, "n")
	screen.await(t, "New session", 5*time.Second)
	screen.await(t, "Isolated git worktree", 2*time.Second)

	// Type a title and commit with ctrl+s. The default working copy is an
	// isolated worktree.
	//
	// The title is multi-word on purpose: "up" names a key, and both our
	// dispatch and bubbles/textinput used to match bindings against the
	// stringified key message, so this exact sentence lost a word.
	screen.send(t, "wire up the smoke test")
	screen.send(t, "\x13") // ctrl+s

	// The session view should attach to the pane and the agent should be live.
	screen.await(t, "ATTACHED", 10*time.Second)
	screen.await(t, "bash$", 10*time.Second)
	screen.await(t, "wire up the smoke test", 5*time.Second)

	// Type a spaced command into the live agent. This is the end-to-end guard
	// for key encoding: bubbles/textinput handles KeySpace itself, so a form
	// that accepts spaces proves nothing about the PTY path, where a dropped
	// space turned "echo hello world" into "echohelloworld".
	screen.send(t, "echo spaces reach the agent\r")
	screen.await(t, "spaces reach the agent", 10*time.Second)

	// The worktree must exist on disk, on its own session branch.
	worktrees := filepath.Join(stateDir, "deck", "worktrees")
	branch := findWorktreeBranch(t, repo)
	if !strings.HasPrefix(branch, "session/") {
		t.Errorf("worktree branch = %q, want a session/ branch", branch)
	}
	if _, err := os.Stat(worktrees); err != nil {
		t.Errorf("worktree root not created: %v", err)
	}

	// The agent owns the keyboard now, so quitting goes through the prefix.
	screen.send(t, "\x07q") // ctrl+g, then q

	if err := waitFor(cmd, 5*time.Second); err != nil {
		t.Errorf("deck did not exit after ^g q: %v", err)
	}
	t.Logf("final frame:\n%s", screen.text())
}

func TestSeedSkipsNonRepo(t *testing.T) {
	bin := buildBinary(t)
	stateDir := t.TempDir()
	plain := t.TempDir()

	cmd := exec.Command(bin, "-agent", "bash")
	cmd.Dir = plain
	cmd.Env = append(os.Environ(), "XDG_STATE_HOME="+stateDir, "TERM=xterm-256color")

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 30, Cols: 100})
	if err != nil {
		t.Fatalf("start deck: %v", err)
	}
	screen := newScreen(ptmx)
	defer func() {
		_ = ptmx.Close()
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}()

	// Outside a repository there is nothing to seed, so the dashboard must say
	// so rather than inventing a project.
	screen.await(t, "No projects registered", 5*time.Second)
	screen.send(t, "q")
	if err := waitFor(cmd, 5*time.Second); err != nil {
		t.Errorf("deck did not exit: %v", err)
	}
}

// ---- harness ----

var buildOnce struct {
	sync.Once
	path string
	err  error
}

func buildBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "deck-bin")
		if err != nil {
			buildOnce.err = err
			return
		}
		out := filepath.Join(dir, "deck")
		cmd := exec.Command("go", "build", "-o", out, ".")
		if b, err := cmd.CombinedOutput(); err != nil {
			buildOnce.err = err
			t.Logf("go build: %s", b)
			return
		}
		buildOnce.path = out
	})
	if buildOnce.err != nil {
		t.Fatalf("build deck: %v", buildOnce.err)
	}
	return buildOnce.path
}

func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	// macOS temp dirs are symlinked through /private; git reports the resolved
	// path, and the test compares against it.
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("resolve temp dir: %v", err)
	}
	steps := [][]string{
		{"init", "-b", "main"},
		{"config", "user.email", "smoke@example.test"},
		{"config", "user.name", "Smoke Test"},
		{"commit", "--allow-empty", "-m", "root"},
	}
	for _, args := range steps {
		cmd := exec.Command("git", args...)
		cmd.Dir = resolved
		if b, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, b)
		}
	}
	return resolved
}

func findWorktreeBranch(t *testing.T, repo string) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		cmd := exec.Command("git", "worktree", "list", "--porcelain")
		cmd.Dir = repo
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git worktree list: %v\n%s", err, out)
		}
		for _, line := range strings.Split(string(out), "\n") {
			if ref, ok := strings.CutPrefix(line, "branch refs/heads/"); ok && ref != "main" {
				return ref
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return ""
}

// screen accumulates PTY output and answers substring queries against it with
// the escape sequences stripped.
type screen struct {
	ptmx *os.File
	mu   sync.Mutex
	buf  strings.Builder
}

func newScreen(ptmx *os.File) *screen {
	s := &screen{ptmx: ptmx}
	go func() {
		b := make([]byte, 4096)
		for {
			n, err := ptmx.Read(b)
			if n > 0 {
				s.mu.Lock()
				s.buf.Write(b[:n])
				s.mu.Unlock()
				if reply := termquery.Answer(b[:n]); len(reply) > 0 {
					_, _ = ptmx.Write(reply)
				}
			}
			if err != nil {
				return
			}
		}
	}()
	return s
}

func (s *screen) text() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return ansi.Strip(s.buf.String())
}

func (s *screen) send(t *testing.T, keys string) {
	t.Helper()
	// A short settle keeps keystrokes from landing before the frame that
	// should receive them; the UI repaints every 50ms.
	time.Sleep(150 * time.Millisecond)
	if _, err := s.ptmx.WriteString(keys); err != nil {
		t.Fatalf("send %q: %v", keys, err)
	}
}

func (s *screen) await(t *testing.T, want string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if strings.Contains(s.text(), want) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %q\n--- output ---\n%s", want, s.text())
}

func waitFor(cmd *exec.Cmd, timeout time.Duration) error {
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		return os.ErrDeadlineExceeded
	}
}

// TestStoreIsGlobalNotPerDirectory pins the scoping rule: projects and
// sessions live in one store, so a session opened in one repository stays
// reachable from anywhere. It also covers the cwd repository being registered
// on every launch, not only the first — that used to be gated on an empty
// store, so cd-ing into a new repository showed every project except that one.
func TestStoreIsGlobalNotPerDirectory(t *testing.T) {
	bin := buildBinary(t)
	stateDir := t.TempDir()
	repoA := initRepo(t)
	repoB := initRepo(t)

	run := func(dir string, keys []string, want string, timeout time.Duration) *screen {
		t.Helper()
		cmd := exec.Command(bin, "-agent", "bash", "-agent-args", "--norc --noprofile")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"XDG_STATE_HOME="+stateDir, "TERM=xterm-256color", "PS1=bash$ ")
		ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 40, Cols: 120})
		if err != nil {
			t.Fatalf("start deck in %s: %v", dir, err)
		}
		sc := newScreen(ptmx)
		t.Cleanup(func() {
			_ = ptmx.Close()
			_ = cmd.Process.Kill()
			_, _ = cmd.Process.Wait()
		})
		// Wait for the first frame before typing. Keys sent while Bubble Tea is
		// still negotiating with the terminal land nowhere.
		sc.await(t, "PROJECTS", 10*time.Second)
		for _, k := range keys {
			sc.send(t, k)
		}
		sc.await(t, want, timeout)
		return sc
	}

	// Open a session in repo A.
	a := run(repoA, []string{"n", "session in repo A", "\x13"}, "ATTACHED", 15*time.Second)
	a.await(t, "session in repo A", 5*time.Second)
	a.send(t, "\x07q")

	// Launch from repo B. Repo A's project and session must still be listed,
	// and repo B must have registered itself despite the store being non-empty.
	b := run(repoB, nil, "PROJECTS", 10*time.Second)
	b.await(t, filepath.Base(repoA), 5*time.Second)
	b.await(t, filepath.Base(repoB), 5*time.Second)
	b.await(t, "2 projects", 5*time.Second)
	b.send(t, "q")
}
