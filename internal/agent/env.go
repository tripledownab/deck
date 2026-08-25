package agent

// The environment a hosted agent is spawned with.

import (
	"os"
	"strings"
)

// dropFromEnv are the variables a hosted agent must not inherit.
//
// ANTHROPIC_API_KEY and ANTHROPIC_AUTH_TOKEN are the billing rule carried over
// from cathode: claude treats either as a bearer token, so one present makes
// it bill the API instead of the logged-in plan. Do not remove them to "fix"
// an auth problem.
//
// CLAUDE_CODE_CHILD_SESSION is set inside a running Claude Code session and
// marks its children as subsessions, which turns transcript saving off. Deck
// is a plausible thing to launch from inside such a session, and every agent it
// then spawned wrote no transcript — so `dirHasHistory` found nothing, --continue
// never applied, and reopening an isolated session silently started fresh. The
// sessions Deck hosts are top-level ones; the marker is not true of them.
var dropFromEnv = map[string]bool{
	"ANTHROPIC_API_KEY":         true,
	"ANTHROPIC_AUTH_TOKEN":      true,
	"CLAUDE_CODE_CHILD_SESSION": true,
}

// ScrubbedEnv returns the environment a hosted agent should run with.
//
// Exported so anything else that spawns an agent uses this rule rather than
// restating it — a second copy of a billing rule is a second thing to drift.
func ScrubbedEnv() []string {
	env := os.Environ()
	out := make([]string, 0, len(env))
	for _, kv := range env {
		// Match the whole name. This was a substring test against the names
		// concatenated together, which also dropped any variable whose name
		// was a tail of one of them: a plain TOKEN or KEY in the environment
		// vanished from every agent, silently.
		name, _, ok := strings.Cut(kv, "=")
		if ok && dropFromEnv[name] {
			continue
		}
		out = append(out, kv)
	}
	return out
}
