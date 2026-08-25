package main

// The tests here drive a real, interactive `claude`. Opt-in, because each
// spends a turn on the user's subscription:
//
//	DECK_LIVE=1 go test -run TestLiveInteractive .
//
// internal/coord already drives claude with -p, which proves the settings
// shape and the turn-boundary events. It cannot reach any of these three: -p
// raises no permission dialog, so the Notification hooks behind "◆ Needs you"
// were only ever inferred; it has no keyboard, so a turn cannot be
// interrupted; and it paints no TUI, so there is no pane whose silence means
// anything.
//
// They run claude inside an agent.Runner rather than a bare PTY, which is the
// path deck itself uses — same emulator, same reply pump answering the
// terminal queries claude sends on startup.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tripledownab/deck/internal/agent"
	"github.com/tripledownab/deck/internal/coord"
)

const liveSessionID = "s1"

// liveClaude is a running claude and the coordinator its hooks report to.
type liveClaude struct {
	r *agent.Runner
	c *coord.Coordinator
}

// startLiveClaude boots claude wired to a coordinator by the same hooks config
// Deck writes, and leaves it at the prompt ready to type.
func startLiveClaude(t *testing.T) *liveClaude {
	t.Helper()
	if os.Getenv("DECK_LIVE") == "" {
		t.Skip("set DECK_LIVE=1 to run; this test drives the real claude CLI")
	}
	if _, err := exec.LookPath("claude"); err != nil {
		t.Skip("claude not on PATH")
	}

	c, err := coord.Start(t.TempDir())
	if err != nil {
		t.Fatalf("coordinator: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	work := t.TempDir()
	c.Register(coord.Session{ID: liveSessionID, ProjectID: "p1",
		Name: "swift-otter-aaaa", Dir: work})
	settings := filepath.Join(work, "hooks.json")
	if err := os.WriteFile(settings, []byte(c.HooksConfigJSON(liveSessionID)), 0o600); err != nil {
		t.Fatal(err)
	}

	// Manual permission mode, or a tool call is approved before anyone is
	// asked and there is no notification to observe.
	r, err := agent.Start(agent.Config{
		Command: "claude",
		Args:    []string{"--settings", settings, "--permission-mode", "default"},
		Dir:     work,
		Width:   120, Height: 40,
	})
	if err != nil {
		t.Fatalf("start claude: %v", err)
	}
	t.Cleanup(r.Stop)

	lc := &liveClaude{r: r, c: c}
	// A fresh directory raises the trust dialog, and it swallows whatever is
	// typed next. Answering it first is not optional setup noise: skip it and
	// the prompt lands in the dialog and the turn never starts.
	lc.await(t, "trust", 20*time.Second)
	lc.send(t, "\r")
	lc.await(t, "manualmode", 20*time.Second)
	return lc
}

// frame is the emulated screen with spaces removed.
//
// The spaces go because claude's TUI positions the cursor rather than printing
// runs of text, so a phrase on screen may have no spaces between its words in
// the cell buffer. Matching compacted text is the difference between a
// reliable await and one that depends on how the TUI laid out that frame.
func (lc *liveClaude) frame() string {
	joined := strings.Join(lc.r.Render(false, nil, nil), "\n")
	return strings.ReplaceAll(joined, " ", "")
}

func (lc *liveClaude) send(t *testing.T, keys string) {
	t.Helper()
	if err := lc.r.Write([]byte(keys)); err != nil {
		t.Fatalf("write %q: %v", keys, err)
	}
}

func (lc *liveClaude) await(t *testing.T, want string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if strings.Contains(lc.frame(), want) {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %q\n--- frame ---\n%s",
		want, strings.Join(lc.r.Render(false, nil, nil), "\n"))
}

// submit types a prompt and then presses Enter as a separate write.
//
// Sending "text\r" in one write leaves the text sitting on the input line
// unsent: claude reads a bulk write as a paste and keeps the newline as part
// of it. The turn then never starts, and the test fails looking like a hook
// problem.
func (lc *liveClaude) submit(t *testing.T, prompt string) {
	t.Helper()
	lc.send(t, prompt)
	time.Sleep(time.Second)
	lc.send(t, "\r")
}

// awaitState polls for a reported state, returning what it found and whether
// it arrived in time. One caller needs the "did not arrive" answer rather than
// a fatal, so this reports instead of failing.
func (lc *liveClaude) awaitState(want coord.State, timeout time.Duration) (coord.State, bool) {
	deadline := time.Now().Add(timeout)
	var last coord.State
	for time.Now().Before(deadline) {
		if s, ok := lc.c.StateOf(liveSessionID); ok {
			last = s
			if s == want {
				return s, true
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return last, false
}

// TestLiveInteractiveClaudeReportsAPermissionPrompt closes the last inferred
// link in the status feature: that claude posts to our endpoint when it raises
// a Notification, which is the only thing that lights "◆ Needs you".
func TestLiveInteractiveClaudeReportsAPermissionPrompt(t *testing.T) {
	lc := startLiveClaude(t)

	// A command no allow-list is going to carry. A machine that pre-approves
	// this would auto-run it, and the test says so below rather than passing
	// on a turn that never prompted.
	lc.submit(t, "Run this exact shell command and tell me what it prints: deck-probe-xyz --version")

	if got, ok := lc.awaitState(coord.StateWaiting, 90*time.Second); !ok {
		t.Fatalf("no waiting state arrived; last reported %v.\n"+
			"If the frame below shows the tool ran without asking, this machine's "+
			"permission settings cover it and the run proves nothing about the "+
			"Notification hooks.\n--- frame ---\n%s",
			got, strings.Join(lc.r.Render(false, nil, nil), "\n"))
	}
}

// TestLiveInteractiveClaudeReportsNothingOnInterrupt pins the gap that
// ui.staleWorkingReport exists to cover: esc ends a turn and fires no hook.
//
// It asserts the absence, which is deliberate. Stop and StopFailure are the
// only events that close a turn and neither covers an interrupt, so the
// coordinator is left saying "working" about a turn that is over. Deck
// compensates in the UI rather than upstream, and this test is the reason that
// compensation is not superstition.
//
// If it ever fails, claude has started reporting interrupts — read the events
// and the staleness fallback can probably go.
func TestLiveInteractiveClaudeReportsNothingOnInterrupt(t *testing.T) {
	lc := startLiveClaude(t)

	lc.submit(t, "Count slowly from 1 to 500, one number per line.")
	if _, ok := lc.awaitState(coord.StateWorking, 60*time.Second); !ok {
		t.Fatalf("the turn never started, so there is nothing to interrupt.\n%s", lc.frame())
	}

	lc.send(t, "\x1b") // esc

	// Long enough for a closing event to arrive if one were coming: the whole
	// turn boundary is a single local POST.
	got, cleared := lc.awaitState(coord.StateIdle, 20*time.Second)
	if cleared {
		t.Errorf("esc now clears the reported state (%v). That is better than "+
			"when this was written — check which event fired and consider "+
			"dropping ui.staleWorkingReport.", got)
	}
	if got != coord.StateWorking {
		t.Errorf("after esc the state is %v; this test assumed Working, so the "+
			"premise behind ui.staleWorkingReport needs rechecking", got)
	}
}

// TestLiveInteractiveClaudePaneNeverGoesQuietMidTurn is the calibration guard
// for ui.staleWorkingReport.
//
// That threshold lets a silent pane override a "working" report, and it is
// only safe because claude repaints a spinner while it works. If a future
// version stopped animating, an ordinary turn would look like a finished one
// and the dot would lie the other way. Nothing else in the suite would notice.
//
// What this does NOT cover is a long foreground tool call, which is where a
// pane would most plausibly stop being repainted. Producing one is the
// obstacle: claude's own Bash tool refuses a foreground `sleep` and runs it in
// the background instead, so the turn ends in seconds. See docs/backlog.md.
func TestLiveInteractiveClaudePaneNeverGoesQuietMidTurn(t *testing.T) {
	lc := startLiveClaude(t)

	lc.submit(t, "Write a haiku about harbours, then explain each line.")
	if _, ok := lc.awaitState(coord.StateWorking, 60*time.Second); !ok {
		t.Fatalf("the turn never started.\n%s", lc.frame())
	}

	// Sample until Stop, which the coordinator knows and the screen does not:
	// the prompt is echoed into the pane, so a sentinel word in the
	// instruction is already on screen. Watching for one ended an earlier
	// version of this loop in four seconds having measured only the startup.
	var longest time.Duration
	start := time.Now()
	for time.Since(start) < 120*time.Second {
		if q := lc.r.Quiet(); q > longest {
			longest = q
		}
		if s, ok := lc.c.StateOf(liveSessionID); ok && s == coord.StateIdle {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if watched := time.Since(start); watched < 3*time.Second {
		t.Fatalf("the turn was over in %v, which is too short to have sampled "+
			"anything\n--- frame ---\n%s",
			watched, strings.Join(lc.r.Render(false, nil, nil), "\n"))
	}

	// Measured at just under a second when this was written. Half the
	// threshold is the point at which the margin has stopped being comfortable
	// and the number needs revisiting, not the point at which it breaks.
	t.Logf("longest silence mid-turn: %v (threshold 10s)", longest)
	if longest > 5*time.Second {
		t.Errorf("the pane went quiet for %v mid-turn. ui.staleWorkingReport is "+
			"10s, so a turn is close to being mistaken for a finished one — "+
			"raise the threshold or stop trusting pane silence", longest)
	}
}
