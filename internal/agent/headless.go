package agent

// A headless agent run: one turn, no pseudo-terminal, and a structured report
// of what it cost. Used for work Deck starts on an agent's behalf rather than
// on a person's, where nobody is watching a pane.

import (
	"context"
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

// streamArgs put the CLI in line-delimited mode.
//
// stream-json rather than json because it is the only format that reports
// usage before the turn ends, and one format is kept rather than two: a second
// parser for the non-live path would be a second place for the field names to
// drift. --verbose is required alongside it, and --include-partial-messages is
// what makes the output count move during a turn rather than once at the end.
var streamArgs = []string{
	"-p", "--output-format", "stream-json", "--verbose", "--include-partial-messages",
}

// RunClaude runs one non-interactive turn in dir and reports what it cost,
// with the answer when there is one.
//
// onUsage, when not nil, is called with each accounting the run reports as it
// goes, on the reading goroutine. See readClaudeStream for what actually moves.
//
// A turn that ends in a refusal or an API error comes back as a ClaudeRun with
// Failure set and its cost intact, not as an error. That now holds for a
// non-zero exit status too: the result event is the accounting, so if one
// arrived it is returned whatever the process did afterwards. Reading stdout to
// the end before waiting is what makes that possible.
//
// The prompt goes over stdin rather than as an argument: with stdin empty
// claude reports "input must be provided" and ignores a positional prompt.
//
// The environment is scrubbed exactly as an interactive session's is, so a
// spawned run cannot inherit credentials or the child-session marker from
// whatever started Deck.
func RunClaude(ctx context.Context, dir, prompt string, onUsage func(Tokens), args ...string) (ClaudeRun, error) {
	cmd := exec.CommandContext(ctx, "claude", append(append([]string{}, streamArgs...), args...)...)
	cmd.Dir = dir
	cmd.Env = ScrubbedEnv()
	cmd.Stdin = strings.NewReader(prompt)

	// Captured by hand because StdoutPipe rules out cmd.Output, which is what
	// used to collect it. The CLI reports its own diagnosis here, and a bare
	// "exit status 1" tells the caller nothing about whether it was auth, a bad
	// flag or a refusal.
	var errOut strings.Builder
	cmd.Stderr = &errOut

	out, err := cmd.StdoutPipe()
	if err != nil {
		return ClaudeRun{}, fmt.Errorf("claude: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return ClaudeRun{}, fmt.Errorf("claude: %w", err)
	}
	run, readErr := readClaudeStream(out, onUsage)
	waitErr := cmd.Wait()

	if readErr != nil {
		// Nothing to report but the failure, so the exit status and whatever
		// the CLI said about it are worth more than the parse error.
		if waitErr != nil {
			return ClaudeRun{}, claudeFailure(waitErr, errOut.String())
		}
		return ClaudeRun{}, readErr
	}
	return run, nil
}

// claudeFailure turns a failed run into the most informative error available.
func claudeFailure(err error, stderr string) error {
	if s := strings.TrimSpace(stderr); s != "" {
		return fmt.Errorf("claude: %s", s)
	}
	return fmt.Errorf("claude: %w", err)
}
