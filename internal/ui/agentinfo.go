package ui

// What the footer says about the program a session started: the file it ran,
// and for an agent that runs claude, which claude that PATH finds and its
// version.
//
// Deck starts an agent by name, and two installs of one program on PATH are
// easy to end up with. The shell you type in may find a different one from
// the environment Deck inherited — a tmux server started without nvm loaded
// finds a stale system-wide install, and a session fails in a way the same
// command typed by hand does not. Naming the file and the version is what
// makes that visible.

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/tripledownab/deck/internal/agent"
)

// versionTimeout bounds `claude --version`. It runs off the UI's goroutine,
// so a slow one costs nothing but a late notice; this bounds a wedged one,
// plus versionWaitDelay when a child of it holds the output pipe. The timeout
// kills the process it started, not that process's own children. A var so a
// test can shorten it rather than wait five seconds.
var versionTimeout = 5 * time.Second

// versionWaitDelay is how long the check waits for the output pipe to close
// once the process it started has gone. A wrapper or shim whose own child
// inherits stdout keeps the pipe open after the wrapper exits, or after the
// timeout kills it, and without this the read waits for that child instead.
const versionWaitDelay = time.Second

// agentVersionMsg carries what a version check found back to the footer.
// started is the notice it was asked for and seq the start it belongs to, so
// a notice that has since been replaced is left alone — even by an identical
// one, which two starts in the same directory produce.
type agentVersionMsg struct {
	started, agentPath, dir, info string
	seq                           int
}

// startedNotice is the footer line for an agent that just started. info is
// what the version check found, or "" before it has answered.
func startedNotice(agentPath, dir, info string) string {
	if info != "" {
		agentPath += " (" + info + ")"
	}
	return "started " + agentPath + " in " + dir
}

// runsClaude reports whether an agent answers through the claude CLI: claude
// itself, or cathode, whose default backend starts claude by name from the
// PATH it inherits from Deck. cathode run with another backend, through
// -agent-args, starts no claude, and the notice then names one that is not
// answering. coordArgs and willResume switch on the same base name.
func runsClaude(command string) bool {
	switch filepath.Base(command) {
	case "claude", "cathode":
		return true
	}
	return false
}

// announceStart sets the notice for an agent that just started and returns
// the check that completes it, or nil when there is nothing to ask.
//
// agentPath is the file the runner started (agent.Runner.Path), not a second
// lookup of the command, which could find a different one. A relative path is
// shown as given, beside the directory it ran in, and the check runs in that
// directory so it finds the same file.
func (m *Model) announceStart(agentPath, command, dir string) tea.Cmd {
	m.notice = startedNotice(agentPath, dir, "")
	m.starts++
	if !runsClaude(command) {
		return nil
	}
	started, seq := m.notice, m.starts
	return func() tea.Msg {
		return agentVersionMsg{started: started, agentPath: agentPath, dir: dir, seq: seq,
			info: claudeInfo(filepath.Base(command) == "claude", agentPath, dir)}
	}
}

// claudeInfo is the claude part of the notice. For claude itself the path is
// already shown, so only the version is; for cathode it is the claude its
// default backend will start.
//
// A failure is reported in place of the version rather than dropped: "could
// not read" is the notice that should send someone to look.
func claudeInfo(isClaude bool, agentPath, dir string) string {
	path := agentPath
	if !isClaude {
		p, err := exec.LookPath("claude")
		if err != nil {
			return "claude " + agent.LookupReason(err)
		}
		path = p
	}
	ctx, cancel := context.WithTimeout(context.Background(), versionTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, "--version")
	cmd.Dir = dir
	cmd.Env = agent.ScrubbedEnv()
	cmd.WaitDelay = versionWaitDelay
	out, err := cmd.Output()
	// A pipe a child still held is not a failure when the version arrived.
	if err != nil && !(errors.Is(err, exec.ErrWaitDelay) && len(out) > 0) {
		reason := err.Error()
		var exit *exec.ExitError
		switch {
		case ctx.Err() != nil:
			// Killed by the timeout above, which "signal: killed" would
			// report as claude crashing.
			reason = "no answer within " + versionTimeout.String()
		case errors.As(err, &exit):
			if line, _, _ := strings.Cut(strings.TrimSpace(string(exit.Stderr)), "\n"); line != "" {
				reason += ": " + printable(line)
			}
		}
		return "could not read the version of " + path + ": " + reason
	}
	// "2.1.322 (Claude Code)": the first word is the version, and the rest
	// only repeats the program's name, which the notice already gives.
	version := "(no version printed)"
	if fields := strings.Fields(string(out)); len(fields) > 0 {
		version = fields[0]
	}
	if isClaude {
		return version
	}
	return "claude " + version + " at " + path
}

// applyVersion completes the start notice, unless something has replaced it
// since — a later notice is newer news than this one.
func (m *Model) applyVersion(msg agentVersionMsg) {
	if m.notice == msg.started && m.starts == msg.seq {
		m.notice = startedNotice(msg.agentPath, msg.dir, msg.info)
	}
}

// printable drops control characters from text bound for the footer. stderr is
// a pipe here, so colour codes are unlikely, but an escape sequence that did
// arrive would be interpreted by the terminal rather than shown.
func printable(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, s)
}
