package agent

// A headless agent run: one turn, no pseudo-terminal, and a structured report
// of what it cost. Used for work Deck starts on an agent's behalf rather than
// on a person's, where nobody is watching a pane.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Tokens is the usage a run reported, split the way billing splits it.
//
// Cache writes and reads are separated because they price differently and are
// the largest single influence on what a short run costs: the same one-word
// reply measured at $0.012 when its context was read from cache and $0.237
// when the same context was written to it.
type Tokens struct {
	Input      int `json:"input_tokens"`
	Output     int `json:"output_tokens"`
	CacheRead  int `json:"cache_read_input_tokens"`
	CacheWrite int `json:"cache_creation_input_tokens"`
}

// ClaudeRun is what one headless turn reported.
//
// A turn that ran and failed is a result, not an error: it has a cost, and
// that cost is the one most worth noticing. So Failure is a field rather than
// a returned error. RunClaude returns an error only when there is no
// accounting at all — the process would not start, or produced nothing we can
// read — because Go's convention tells a caller to discard the value alongside
// an error, which would put the bill out of reach exactly when it is
// surprising.
type ClaudeRun struct {
	Text    string        // the model's final answer, empty when it failed
	Failure string        // why the turn produced no answer; empty on success
	CostUSD float64       // what the turn cost, whether or not it answered
	Tokens  Tokens        // usage, split by kind
	Took    time.Duration // how long the turn took, as the CLI measured it
}

// Failed reports whether the turn ran without producing an answer.
func (r ClaudeRun) Failed() bool { return r.Failure != "" }

// resultEnvelope is the one event in the stream that carries the totals. The
// field names are claude's; another agent would need its own parser, which is
// why this function is named for the one it understands.
type resultEnvelope struct {
	Type       string  `json:"type"`
	Subtype    string  `json:"subtype"`
	IsError    bool    `json:"is_error"`
	Result     string  `json:"result"`
	DurationMS int     `json:"duration_ms"`
	CostUSD    float64 `json:"total_cost_usd"`
	Usage      Tokens  `json:"usage"`
}

// RunClaude runs one non-interactive turn in dir and reports what it cost,
// with the answer when there is one.
//
// A turn that ends in a refusal or an API error comes back as a ClaudeRun with
// Failure set and its cost intact, not as an error. See ClaudeRun.
//
// The prompt goes over stdin rather than as an argument: with stdin empty
// claude reports "input must be provided" and ignores a positional prompt.
//
// The environment is scrubbed exactly as an interactive session's is, so a
// spawned run cannot inherit credentials or the child-session marker from
// whatever started Deck.
func RunClaude(ctx context.Context, dir, prompt string, args ...string) (ClaudeRun, error) {
	full := append([]string{"-p", "--output-format", "json"}, args...)
	cmd := exec.CommandContext(ctx, "claude", full...)
	cmd.Dir = dir
	cmd.Env = ScrubbedEnv()
	cmd.Stdin = strings.NewReader(prompt)

	out, err := cmd.Output()
	if err != nil {
		// The CLI reports its own diagnosis on stderr; a bare "exit status 1"
		// tells the caller nothing about whether it was auth, a bad flag or a
		// refusal.
		var ee *exec.ExitError
		if errors.As(err, &ee) && len(ee.Stderr) > 0 {
			return ClaudeRun{}, fmt.Errorf("claude: %s", strings.TrimSpace(string(ee.Stderr)))
		}
		return ClaudeRun{}, fmt.Errorf("claude: %w", err)
	}
	return parseClaudeJSON(out)
}

// parseClaudeJSON pulls the totals out of a --output-format json stream.
//
// The stream is an array of events, not a single object: an init event, the
// assistant turns, and one result event carrying the totals. Reading the last
// element would work today and break the moment anything is appended after
// it, so this selects by type.
func parseClaudeJSON(out []byte) (ClaudeRun, error) {
	var events []resultEnvelope
	if err := json.Unmarshal(out, &events); err != nil {
		// A single object rather than an array is also valid JSON output; try
		// it before giving up, so a change of shape degrades to one parse
		// failure rather than to a wrong answer.
		var one resultEnvelope
		if json.Unmarshal(out, &one) != nil {
			return ClaudeRun{}, fmt.Errorf("claude produced no readable result: %w", err)
		}
		events = []resultEnvelope{one}
	}
	for _, e := range events {
		if e.Type != "result" {
			continue
		}
		run := ClaudeRun{
			Text:    e.Result,
			CostUSD: e.CostUSD,
			Tokens:  e.Usage,
			Took:    time.Duration(e.DurationMS) * time.Millisecond,
		}
		if e.IsError {
			run.Text = ""
			run.Failure = strings.TrimSpace(e.Subtype + ": " + e.Result)
		}
		return run, nil
	}
	// No result event means no accounting: whatever it spent, we cannot say.
	return ClaudeRun{}, fmt.Errorf("claude produced no result event")
}
