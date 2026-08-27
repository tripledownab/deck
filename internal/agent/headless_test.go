package agent

import (
	"strings"
	"testing"
	"time"
)

// A captured stream, trimmed to the events that matter. The shape is the one
// `claude -p --output-format json` actually emits: an array, with the totals
// in a result event rather than in the last element by position.
const captured = `[
 {"type":"system","subtype":"init","session_id":"abc"},
 {"type":"assistant","message":{"role":"assistant"}},
 {"type":"result","subtype":"success","is_error":false,"result":"ZEPHYR_QUOTA_GUARD",
  "duration_ms":1624,"num_turns":1,"total_cost_usd":0.23709,
  "usage":{"input_tokens":2,"output_tokens":4,
           "cache_read_input_tokens":0,"cache_creation_input_tokens":23698}},
 {"type":"trailing_event_added_later"}
]`

// TestParseClaudeJSONSelectsByType is why the parser does not read the last
// element. A stream with anything appended after the result would otherwise
// report zero cost and no answer.
func TestParseClaudeJSONSelectsByType(t *testing.T) {
	run, err := parseClaudeJSON([]byte(captured))
	if err != nil {
		t.Fatal(err)
	}
	if run.Text != "ZEPHYR_QUOTA_GUARD" {
		t.Errorf("text = %q", run.Text)
	}
	if run.CostUSD != 0.23709 {
		t.Errorf("cost = %v, want 0.23709", run.CostUSD)
	}
	if run.Took != 1624*time.Millisecond {
		t.Errorf("took = %v, want the duration the CLI reported", run.Took)
	}
}

// TestParseClaudeJSONSplitsCacheTokens covers the split that explains a bill.
// Reads and writes of the same context price differently, so collapsing them
// into one number would hide the largest influence on a short run's cost.
func TestParseClaudeJSONSplitsCacheTokens(t *testing.T) {
	run, err := parseClaudeJSON([]byte(captured))
	if err != nil {
		t.Fatal(err)
	}
	want := Tokens{Input: 2, Output: 4, CacheRead: 0, CacheWrite: 23698}
	if run.Tokens != want {
		t.Errorf("tokens = %+v, want %+v", run.Tokens, want)
	}
}

// TestFailedTurnKeepsItsCost is the property the whole shape exists for. A
// turn that ran and refused still spent money, and returning it as a Go error
// would tell every caller to discard the value — putting the bill out of reach
// in the one case where it is surprising.
func TestFailedTurnKeepsItsCost(t *testing.T) {
	stream := `[{"type":"result","subtype":"error_max_turns","is_error":true,
	             "result":"ran out of turns","total_cost_usd":0.4,
	             "usage":{"input_tokens":7,"output_tokens":0}}]`
	run, err := parseClaudeJSON([]byte(stream))
	if err != nil {
		t.Fatalf("a turn that ran was reported as unusable: %v", err)
	}
	if !run.Failed() {
		t.Error("a refused turn does not report itself as failed")
	}
	if run.CostUSD != 0.4 {
		t.Errorf("cost = %v, want 0.4 — a failed run that spent was not counted", run.CostUSD)
	}
	if run.Tokens.Input != 7 {
		t.Errorf("tokens = %+v, want the usage the envelope reported", run.Tokens)
	}
	if run.Text != "" {
		t.Errorf("text = %q; a failed turn has no answer to give", run.Text)
	}
	for _, want := range []string{"error_max_turns", "ran out of turns"} {
		if !strings.Contains(run.Failure, want) {
			t.Errorf("failure %q does not mention %q", run.Failure, want)
		}
	}
}

// TestParseClaudeJSONReportsAMissingResult covers a stream that ends without
// totals — a crash mid-run. Returning a zero-cost success would under-report
// the bill and hand the caller an empty answer as if it were real.
func TestParseClaudeJSONReportsAMissingResult(t *testing.T) {
	if _, err := parseClaudeJSON([]byte(`[{"type":"system","subtype":"init"}]`)); err == nil {
		t.Error("a stream with no result event parsed as a success")
	}
	if _, err := parseClaudeJSON([]byte(`not json at all`)); err == nil {
		t.Error("unparseable output was accepted")
	}
}
