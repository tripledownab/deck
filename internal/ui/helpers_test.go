package ui

// Shared fixtures for the tests that need a real agent, and the guard that
// keeps the whole package off the real state directory. Everything here drives
// bash under a PTY rather than faking a Runner: the UI reads Status(), Err()
// and Render() off a live process, and a stub would prove only that the stub
// behaves as written.

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/tripledownab/deck/internal/agent"
)

// TestMain points the state directory somewhere disposable for every test in
// the package.
//
// This is not tidiness. Any test that renames, adds or closes something calls
// store.Save, which writes $XDG_STATE_HOME/deck/state.json — and with the
// variable unset that path is the real one under the home directory. Running
// `go test ./internal/ui` replaced a developer's registered projects with a
// two-project fixture, and the suite passed while doing it. Per-test t.Setenv
// protects the test that remembers to call it; this protects the one that does
// not.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "deck-ui-state")
	if err != nil {
		fmt.Fprintln(os.Stderr, "test state dir:", err)
		os.Exit(1)
	}
	os.Setenv("XDG_STATE_HOME", dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// startAgent runs bash under a PTY and returns the runner, stopped on cleanup.
//
// An empty script gives a plain interactive bash that stays up. Anything else
// runs under -c, so a test can pick a process that exits on its own or one
// that sits quietly.
func startAgent(t *testing.T, dir, script string) *agent.Runner {
	t.Helper()
	args := []string{"--norc", "--noprofile"}
	if script != "" {
		args = append(args, "-c", script)
	}
	r, err := agent.Start(agent.Config{
		Command: "bash",
		Args:    args,
		Dir:     dir,
		Width:   80, Height: 24,
	})
	if err != nil {
		t.Fatalf("start agent: %v", err)
	}
	t.Cleanup(r.Stop)
	return r
}

// waitExited blocks until the agent is gone, so a test asserting on a dead
// process is not racing its own fixture.
func waitExited(t *testing.T, r *agent.Runner) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for r.Status() != agent.Exited && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if r.Status() != agent.Exited {
		t.Fatal("the agent never exited; the test proves nothing")
	}
}
