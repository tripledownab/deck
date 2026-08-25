package agent

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
)

// screenContains polls the emulated screen until want appears.
func screenContains(t *testing.T, r *Runner, want string, timeout time.Duration) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if strings.Contains(ansi.Strip(strings.Join(r.Render(false, nil, nil), "\n")), want) {
			return true
		}
		time.Sleep(25 * time.Millisecond)
	}
	return false
}

func TestRunnerShowsOutput(t *testing.T) {
	r, err := Start(Config{
		Command: "bash",
		Args:    []string{"--norc", "--noprofile", "-c", "echo hello-from-agent; sleep 5"},
		Dir:     t.TempDir(),
		Width:   80,
		Height:  24,
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer r.Stop()

	if !screenContains(t, r, "hello-from-agent", 5*time.Second) {
		t.Errorf("output never reached the screen:\n%s", strings.Join(r.Render(false, nil, nil), "\n"))
	}
}

// TestRunnerAnswersTerminalQueries is the regression for the missing reply
// pump. A terminal is bidirectional: programs ask it questions and block on
// the answer. The emulator queues replies on an io.Pipe, and an io.Pipe write
// blocks until something reads it — so with no reader, the first query an
// agent sends deadlocks the parser while it holds the emulator lock.
//
// The script below asks for the cursor position (DSR) and waits for the reply
// terminator. Without the pump it times out; worse, the session hangs.
func TestRunnerAnswersTerminalQueries(t *testing.T) {
	const script = `printf '\033[6n'; IFS= read -r -d R -t 3 resp; ` +
		`if [ -n "$resp" ]; then echo DSR_ANSWERED; else echo DSR_TIMEOUT; fi; sleep 5`

	r, err := Start(Config{
		Command: "bash",
		Args:    []string{"--norc", "--noprofile", "-c", script},
		Dir:     t.TempDir(),
		Width:   80,
		Height:  24,
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer r.Stop()

	if !screenContains(t, r, "DSR_ANSWERED", 8*time.Second) {
		t.Errorf("cursor-position query went unanswered:\n%s",
			strings.Join(r.Render(false, nil, nil), "\n"))
	}
}

func TestRunnerAcceptsInput(t *testing.T) {
	r, err := Start(Config{
		Command: "bash",
		Args:    []string{"--norc", "--noprofile"},
		Dir:     t.TempDir(),
		Width:   80,
		Height:  24,
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer r.Stop()

	time.Sleep(300 * time.Millisecond) // let the shell reach its prompt
	if err := r.Write([]byte("echo typed-through-pty\r")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if !screenContains(t, r, "typed-through-pty", 5*time.Second) {
		t.Errorf("keystrokes never reached the agent:\n%s",
			strings.Join(r.Render(false, nil, nil), "\n"))
	}
}

func TestRunnerStatusReachesExited(t *testing.T) {
	r, err := Start(Config{
		Command: "bash",
		Args:    []string{"--norc", "--noprofile", "-c", "exit 3"},
		Dir:     t.TempDir(),
		Width:   40,
		Height:  10,
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for r.Status() != Exited && time.Now().Before(deadline) {
		time.Sleep(25 * time.Millisecond)
	}
	if r.Status() != Exited {
		t.Fatalf("status = %v, want Exited", r.Status())
	}
	if r.Err() == nil {
		t.Error("a non-zero exit reported no error")
	}
	// Writing to a dead agent must fail loudly rather than look accepted.
	if err := r.Write([]byte("x")); err == nil {
		t.Error("write to an exited agent returned no error")
	}
}

func TestRunnerResize(t *testing.T) {
	r, err := Start(Config{
		Command: "bash",
		Args:    []string{"--norc", "--noprofile"},
		Dir:     t.TempDir(),
		Width:   80,
		Height:  24,
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer r.Stop()

	r.Resize(100, 30)

	// Asserted through Render rather than a size accessor: the render is what
	// the pane consumes, so this checks the thing that actually has to hold —
	// every row exactly the emulator width in cells, or the pane border drifts.
	rows := r.Render(false, nil, nil)
	if len(rows) != 30 {
		t.Fatalf("rendered %d rows, want 30", len(rows))
	}
	for i, line := range rows {
		if w := ansi.StringWidth(line); w != 100 {
			t.Fatalf("row %d is %d cells wide, want 100", i, w)
		}
	}
}

func TestStartRejectsMissingCommand(t *testing.T) {
	if _, err := Start(Config{Command: "definitely-not-a-real-binary-xyz", Dir: t.TempDir()}); err == nil {
		t.Fatal("starting a missing binary returned no error")
	}
	if _, err := Start(Config{Command: "", Dir: t.TempDir()}); err == nil {
		t.Fatal("empty command returned no error")
	}
}

// TestScrubbedEnvDropsWhatAnAgentMustNotInherit pins two rules at once.
//
// The credentials are the billing constraint carried over from cathode: either
// present makes claude bill the API instead of the logged-in plan.
// CLAUDE_CODE_CHILD_SESSION is the one that cost a debugging session — a deck
// launched from inside a Claude Code session passed it on, and every agent it
// hosted ran with transcript saving off, so --continue silently never applied.
func TestScrubbedEnvDropsWhatAnAgentMustNotInherit(t *testing.T) {
	gone := []string{"ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN", "CLAUDE_CODE_CHILD_SESSION"}
	// KEY and TOKEN are here because the scrub used to be a substring test
	// against the dropped names run together, so any variable whose name was a
	// tail of one of them disappeared too.
	kept := []string{"ANTHROPIC_MODEL", "CLAUDE_CODE_ENTRYPOINT", "KEY", "TOKEN"}

	for _, name := range append(append([]string{}, gone...), kept...) {
		t.Setenv(name, "value-for-"+name)
	}

	survived := map[string]bool{}
	for _, kv := range ScrubbedEnv() {
		if name, _, ok := strings.Cut(kv, "="); ok {
			survived[name] = true
		}
	}
	for _, name := range gone {
		if survived[name] {
			t.Errorf("%s survived the scrub", name)
		}
	}
	for _, name := range kept {
		if !survived[name] {
			t.Errorf("the scrub removed %s, which it has no business dropping", name)
		}
	}
	if len(ScrubbedEnv()) >= len(os.Environ()) {
		t.Error("ScrubbedEnv did not drop anything")
	}
}
