package coord

// Tests that drive the real `claude` CLI. Opt-in, because each spends a turn
// on the user's subscription:
//
//	DECK_LIVE=1 go test -run TestLiveClaude ./internal/coord/
//
// They are worth their cost. Every other test in this package posts to our own
// HTTP handler, which proves the server answers correctly and nothing about
// whether claude can reach it — the config shape, the transport, the tool
// naming and the hook event names are only exercised here.

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tripledownab/deck/internal/agent"
)

// liveOnly skips unless the opt-in is set and claude is installed.
func liveOnly(t *testing.T) {
	t.Helper()
	if os.Getenv("DECK_LIVE") == "" {
		t.Skip("set DECK_LIVE=1 to run; this test calls the real claude CLI")
	}
	if _, err := exec.LookPath("claude"); err != nil {
		t.Skip("claude not on PATH")
	}
}

// runClaude runs one non-interactive turn in dir and returns its combined
// output, along with whatever the process exited with — a test that induces a
// failure needs that error rather than a fatal.
//
// The prompt goes over stdin, not as an argument. With stdin closed or empty,
// claude reports "input must be provided" and ignores a positional prompt.
func runClaude(t *testing.T, dir string, args []string, prompt string) (string, error) {
	t.Helper()
	cmd := exec.Command("claude", append([]string{"-p"}, args...)...)
	cmd.Dir = dir
	cmd.Env = agent.ScrubbedEnv()
	cmd.Stdin = strings.NewReader(prompt)

	done := make(chan struct{})
	var out []byte
	var err error
	go func() { out, err = cmd.CombinedOutput(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Minute):
		_ = cmd.Process.Kill()
		t.Fatal("claude did not answer within two minutes")
	}
	return string(out), err
}

// mustRunClaude is runClaude for the turns that are meant to succeed.
func mustRunClaude(t *testing.T, dir string, args []string, prompt string) string {
	t.Helper()
	out, err := runClaude(t, dir, args, prompt)
	if err != nil {
		t.Fatalf("claude: %v\n%s", err, out)
	}
	return out
}

// writeConfig writes a generated config next to the work directory and returns
// its path.
func writeConfig(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestLiveClaudeSeesSiblings drives a real claude against the coordination
// server: it must discover the MCP tools, call one, and come back with a
// sibling session it could only have learned from us.
func TestLiveClaudeSeesSiblings(t *testing.T) {
	liveOnly(t)

	c := start(t)
	work := t.TempDir()

	// A sibling that is already holding a file.
	c.Register(Session{
		ID: "sibling", ProjectID: "p1",
		Name: "swift-otter-aaaa", Title: "rewriting the parser",
		Branch: "session/swift-otter-aaaa", Dir: filepath.Join(work, "sibling"),
	})
	c.Claim("sibling", []string{"src/parser.go"}, "rewrite")

	// The session claude will run as.
	c.Register(Session{
		ID: "asker", ProjectID: "p1",
		Name: "wily-crane-bbbb", Title: "asking", Dir: work,
	})

	cfg := writeConfig(t, work, "deck.mcp.json", c.MCPConfigJSON("asker"))
	got := mustRunClaude(t, work, []string{
		"--mcp-config", cfg,
		// Not plan mode: it blocks MCP tool calls outright, so the run proves
		// only that claude can see the tool, not that it can reach us.
		"--allowedTools", "mcp__deck__sessions",
	}, "Call the deck sessions tool. Reply with only the session name it reports, nothing else.")

	t.Logf("claude replied: %s", strings.TrimSpace(got))
	if !strings.Contains(got, "swift-otter-aaaa") {
		t.Errorf("claude did not report the sibling session; it never reached the server.\n%s", got)
	}
}

// TestLiveClaudeMessaging drives two real claude runs through the mailbox: one
// sends, the other collects.
//
// This is the only test that proves an agent can reach another agent, rather
// than just reach the server.
func TestLiveClaudeMessaging(t *testing.T) {
	liveOnly(t)

	c := start(t)
	work := t.TempDir()
	c.Register(Session{ID: "sender", ProjectID: "p1",
		Name: "swift-otter-aaaa", Title: "sending", Dir: work})
	c.Register(Session{ID: "reader", ProjectID: "p1",
		Name: "wily-crane-bbbb", Title: "reading", Dir: work})

	ask := func(sessionID, tools, prompt string) string {
		t.Helper()
		cfg := writeConfig(t, work, sessionID+".mcp.json", c.MCPConfigJSON(sessionID))
		return mustRunClaude(t, work, []string{"--mcp-config", cfg, "--allowedTools", tools}, prompt)
	}

	ask("sender", "mcp__deck__message",
		"Use the deck message tool to send wily-crane-bbbb exactly: TAKE-THE-LEXER. Then reply DONE.")

	if n := c.Unread("reader"); n != 1 {
		t.Fatalf("reader has %d messages, want 1 — the send did not land", n)
	}

	got := ask("reader", "mcp__deck__inbox",
		"Call the deck inbox tool. Reply with only the message text you received.")
	t.Logf("reader replied: %s", strings.TrimSpace(got))
	if !strings.Contains(got, "TAKE-THE-LEXER") {
		t.Errorf("the message did not reach the other agent:\n%s", got)
	}
}

// TestLiveClaudeReportsTurnBoundaries drives a real claude with the generated
// hooks config and asserts it reports the end of a turn that succeeded.
//
// This and the test below are the only ones that prove the hooks fire at all.
// Everything else posts to our own endpoint, which shows the handler works and
// nothing about whether claude was configured correctly — the settings shape,
// the event names and the http handler type are only exercised here.
func TestLiveClaudeReportsTurnBoundaries(t *testing.T) {
	liveOnly(t)

	c := start(t)
	work := t.TempDir()
	c.Register(Session{ID: "hooked", ProjectID: "p1", Name: "swift-otter-aaaa", Dir: work})

	if _, reported := c.StateOf("hooked"); reported {
		t.Fatal("a state existed before claude ran")
	}

	settings := writeConfig(t, work, "hooks.json", c.HooksConfigJSON("hooked"))
	out := mustRunClaude(t, work, []string{"--settings", settings},
		"Reply with the single word: acknowledged.")
	t.Logf("claude replied: %s", strings.TrimSpace(out))

	state, reported := c.StateOf("hooked")
	if !reported {
		t.Fatal("claude ran a whole turn and no hook reached us; the settings were not applied")
	}
	// The last boundary of a completed turn is Stop.
	if state != StateIdle {
		t.Errorf("state after a completed turn = %v, want %v", state, StateIdle)
	}
}

// TestLiveClaudeReportsAnApiError pins the hole this feature shipped with:
// Stop does not fire when a turn dies on an API error, so a config without
// StopFailure leaves the session reading "Working" until the user types again.
//
// The failure is induced rather than waited for. CLAUDE_CODE_MAX_OUTPUT_TOKENS
// caps the response, and a cap of one token ends the turn as a
// max_output_tokens error before the model writes anything. The input side is
// a normal turn's, so this is cheap rather than free.
func TestLiveClaudeReportsAnApiError(t *testing.T) {
	liveOnly(t)

	c := start(t)
	work := t.TempDir()
	c.Register(Session{ID: "doomed", ProjectID: "p1", Name: "swift-otter-aaaa", Dir: work})

	// The generated hooks, plus the cap that makes the turn fail.
	var settings map[string]any
	if err := json.Unmarshal([]byte(c.HooksConfigJSON("doomed")), &settings); err != nil {
		t.Fatalf("generated settings are not valid JSON: %v", err)
	}
	settings["env"] = map[string]string{"CLAUDE_CODE_MAX_OUTPUT_TOKENS": "1"}
	body, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	path := writeConfig(t, work, "hooks.json", string(body))

	// A failing turn exits non-zero, so the error is the expected outcome here.
	out, runErr := runClaude(t, work, []string{"--settings", path},
		"Write a 500 word essay about harbours.")
	if !strings.Contains(out, "API Error") {
		t.Fatalf("the turn did not fail, so this proves nothing about StopFailure "+
			"(exit: %v):\n%s", runErr, out)
	}

	state, reported := c.StateOf("doomed")
	if !reported {
		t.Fatal("a turn died on an API error and no hook reached us")
	}
	if state != StateWaiting {
		t.Errorf("state after a failed turn = %v, want %v — Stop does not fire on an "+
			"API error, so only StopFailure can clear the Working dot", state, StateWaiting)
	}
}
