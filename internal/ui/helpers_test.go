package ui

// Shared fixtures for the tests that need a real agent. Everything here drives
// bash under a PTY rather than faking a Runner: the UI reads Status(), Err()
// and Render() off a live process, and a stub would prove only that the stub
// behaves as written.

import (
	"testing"
	"time"

	"github.com/tripledownab/deck/internal/agent"
)

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
