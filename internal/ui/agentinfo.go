package ui

// What the footer says about the program a session started: the file its
// name resolved to on PATH, and for an agent that runs claude, which claude
// answers and its version.
//
// Deck starts an agent by name, and two installs of one program on PATH are
// easy to end up with. The shell you type in may find a different one from
// the environment Deck inherited — a tmux server started without nvm loaded
// finds a stale system-wide install, and a session fails in a way the same
// command typed by hand does not. Naming the file and the version is what
// makes that visible.

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/tripledownab/deck/internal/agent"
)

// versionTimeout bounds `claude --version`. It runs off the UI's goroutine,
// so a slow one costs nothing but a late notice; this only stops a wedged one
// from leaving a process behind.
const versionTimeout = 5 * time.Second

// agentVersionMsg carries what a version check found back to the footer.
// started is the notice it was asked for, so a notice that has since been
// replaced is left alone.
type agentVersionMsg struct {
	started, agentPath, dir, info string
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
// itself, or cathode, which starts claude by name from the PATH it inherits
// from Deck. coordArgs and willResume switch on the same base name.
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
// Start has already resolved the command, so LookPath answers here; a failure
// would mean PATH changed between the two, and the name is shown instead.
func (m *Model) announceStart(command, dir string) tea.Cmd {
	agentPath, err := exec.LookPath(command)
	if err != nil {
		agentPath = command
	}
	m.notice = startedNotice(agentPath, dir, "")
	if !runsClaude(command) {
		return nil
	}
	started := m.notice
	return func() tea.Msg {
		return agentVersionMsg{started: started, agentPath: agentPath, dir: dir,
			info: claudeInfo(filepath.Base(command) == "claude", agentPath)}
	}
}

// claudeInfo is the claude part of the notice. For claude itself the path is
// already shown, so only the version is; for cathode it is the claude cathode
// will start, which is the one that answers.
//
// A failure is reported in place of the version rather than dropped: "could
// not read" is the notice that should send someone to look.
func claudeInfo(isClaude bool, agentPath string) string {
	path := agentPath
	if !isClaude {
		p, err := exec.LookPath("claude")
		if err != nil {
			return "claude not found on PATH"
		}
		path = p
	}
	ctx, cancel := context.WithTimeout(context.Background(), versionTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, "--version")
	cmd.Env = agent.ScrubbedEnv()
	out, err := cmd.Output()
	if err != nil {
		return "could not read the version of " + path + ": " + err.Error()
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
	if m.notice == msg.started {
		m.notice = startedNotice(msg.agentPath, msg.dir, msg.info)
	}
}
