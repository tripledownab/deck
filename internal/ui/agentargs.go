package ui

// The argv a session's agent is started with: the coordination config, and
// whether to resume the agent's own prior conversation.

import (
	"os"
	"path/filepath"

	"github.com/tripledownab/deck/internal/store"
)

// coordArgs wires the session's agent to the coordination server.
//
// The flag differs per agent, which is the one place Deck knows anything
// about the program it hosts: claude takes --mcp-config, cathode forwards a
// second config with -mcp. Anything else gets nothing, and the config path is
// reported so it can be wired by hand through -agent-args.
func (m Model) coordArgs(sess *store.Session) []string {
	if m.coord == nil {
		return nil
	}
	// Decide the flag before writing anything: an agent we cannot wire up gets
	// no config file either.
	var flag string
	switch filepath.Base(sess.Agent) {
	case "claude":
		flag = "--mcp-config"
	case "cathode":
		flag = "-mcp"
	default:
		return nil
	}

	dir, err := store.SessionConfigDir()
	if err != nil {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil
	}
	mcpPath := filepath.Join(dir, sess.ID+".mcp.json")
	if err := os.WriteFile(mcpPath, []byte(m.coord.MCPConfigJSON(sess.ID)), 0o600); err != nil {
		return nil
	}
	args := []string{flag, mcpPath}

	// Status hooks are claude-only. cathode spawns its own claude with its own
	// argv, so a --settings we passed to cathode would never reach the process
	// the hooks describe; those sessions keep the activity heuristic.
	if flag == "--mcp-config" {
		hooksPath := filepath.Join(dir, sess.ID+".hooks.json")
		if err := os.WriteFile(hooksPath, []byte(m.coord.HooksConfigJSON(sess.ID)), 0o600); err == nil {
			args = append(args, "--settings", hooksPath)
		}
	}
	return args
}

// agentArgsFor decides whether to resume the agent's own prior conversation.
//
// claude --continue resumes the most recent session recorded for the working
// directory. For an isolated session that directory is a worktree only this
// session ever used, so continuing is exactly right. For a session running in
// the project directory it is not: another tool, or another Deck session,
// may have been the last to run there, and we would silently attach to
// someone else's conversation. Those always start fresh.
func (m Model) agentArgsFor(sess *store.Session) []string {
	args := append([]string(nil), m.agentArgs...)
	args = append(args, m.coordArgs(sess)...)
	if willResume(sess) {
		args = append(args, "--continue")
	}
	return args
}

// willResume reports whether starting sess picks its conversation back up.
//
// The pane promises this to the user and agentArgsFor delivers it, so the two
// read one predicate rather than each testing the same pair of conditions. A
// hint that says "resume" where no --continue is passed is worse than no hint.
func willResume(sess *store.Session) bool {
	return sess.Isolated && dirHasHistory(sess.Dir)
}

// dirHasHistory reports whether claude has a transcript for a directory. It
// is a hint for --continue, so a missing or unreadable state directory simply
// means "no history", not a failure worth reporting.
func dirHasHistory(dir string) bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	entries, err := os.ReadDir(filepath.Join(home, ".claude", "projects"))
	if err != nil {
		return false
	}
	slug := claudeSlug(dir)
	for _, e := range entries {
		if e.Name() == slug {
			return true
		}
	}
	return false
}

// claudeSlug mirrors how claude partitions its per-project transcripts: every
// character outside [A-Za-z0-9-] becomes a dash, one dash per character, with
// no run collapsing. Cathode documents the same rule in projectdir.go.
func claudeSlug(path string) string {
	out := []rune(path)
	for i, r := range out {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-':
		default:
			out[i] = '-'
		}
	}
	return string(out)
}
