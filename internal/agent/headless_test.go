package agent

import (
	"bufio"
	"strings"
	"testing"
	"time"
)

// A captured stream, trimmed to the events that matter. The shape is the one
// `claude -p --output-format stream-json --include-partial-messages` actually
// emits: one JSON object per line, usage in four different places, and the
// totals in a result event rather than in the last line by position.
const captured = `{"type":"system","subtype":"init","session_id":"abc"}
{"type":"stream_event","event":{"type":"message_start","message":{"usage":{"input_tokens":2,"cache_creation_input_tokens":23698,"cache_read_input_tokens":0,"output_tokens":1}}}}
{"type":"stream_event","event":{"type":"message_delta","usage":{"input_tokens":2,"cache_creation_input_tokens":23698,"cache_read_input_tokens":0,"output_tokens":4}}}
{"type":"assistant","message":{"usage":{"input_tokens":2,"cache_creation_input_tokens":23698,"cache_read_input_tokens":0,"output_tokens":4}}}
{"type":"result","subtype":"success","is_error":false,"result":"ZEPHYR_QUOTA_GUARD","duration_ms":1624,"num_turns":1,"total_cost_usd":0.23709,"usage":{"input_tokens":2,"output_tokens":4,"cache_read_input_tokens":0,"cache_creation_input_tokens":23698}}
{"type":"trailing_event_added_later"}`

func parse(t *testing.T, stream string) ClaudeRun {
	t.Helper()
	run, err := readClaudeStream(strings.NewReader(stream), nil)
	if err != nil {
		t.Fatal(err)
	}
	return run
}

// TestReadClaudeStreamSelectsByType is why the parser does not read the last
// line. A stream with anything appended after the result would otherwise
// report zero cost and no answer.
func TestReadClaudeStreamSelectsByType(t *testing.T) {
	run := parse(t, captured)
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

// TestReadClaudeStreamSplitsCacheTokens covers the split that explains a bill.
// Reads and writes of the same context price differently, so collapsing them
// into one number would hide the largest influence on a short run's cost.
func TestReadClaudeStreamSplitsCacheTokens(t *testing.T) {
	want := Tokens{Input: 2, Output: 4, CacheRead: 0, CacheWrite: 23698}
	if got := parse(t, captured).Tokens; got != want {
		t.Errorf("tokens = %+v, want %+v", got, want)
	}
}

// TestUsageIsReportedBeforeTheResult is what a live figure depends on. The
// callback has to fire on events other than the result — otherwise the number
// arrives at the same moment as the answer and there is nothing to watch.
//
// It also pins where usage is read from. Each of the three events below holds
// it in a different place, and a parser that understood only the result would
// pass every other test in this file.
func TestUsageIsReportedBeforeTheResult(t *testing.T) {
	var seen []Tokens
	run, err := readClaudeStream(strings.NewReader(captured), func(tk Tokens) {
		seen = append(seen, tk)
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(seen) < 3 {
		t.Fatalf("usage reported %d times, want one per event that carries it: %+v", len(seen), seen)
	}
	// message_start, then message_delta: the output count is the only figure
	// that moves, which is why the badge shows it and not the input.
	if seen[0].Output != 1 || seen[1].Output != 4 {
		t.Errorf("output did not climb across events: %+v", seen)
	}
	if seen[0].CacheWrite != 23698 {
		t.Errorf("the first event did not carry the full cache cost: %+v", seen[0])
	}
	if run.Tokens.Output != 4 {
		t.Errorf("final tokens = %+v", run.Tokens)
	}
}

// TestFailedTurnKeepsItsCost is the property the whole shape exists for. A
// turn that ran and refused still spent money, and returning it as a Go error
// would tell every caller to discard the value — putting the bill out of reach
// in the one case where it is surprising.
func TestFailedTurnKeepsItsCost(t *testing.T) {
	stream := `{"type":"result","subtype":"error_max_turns","is_error":true,` +
		`"result":"ran out of turns","total_cost_usd":0.4,` +
		`"usage":{"input_tokens":7,"output_tokens":0}}`
	run, err := readClaudeStream(strings.NewReader(stream), nil)
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

// TestReadClaudeStreamReportsAMissingResult covers a stream that ends without
// totals — a crash mid-run. Returning a zero-cost success would under-report
// the bill and hand the caller an empty answer as if it were real.
func TestReadClaudeStreamReportsAMissingResult(t *testing.T) {
	if _, err := readClaudeStream(strings.NewReader(`{"type":"system","subtype":"init"}`), nil); err == nil {
		t.Error("a stream with no result event parsed as a success")
	}
	if _, err := readClaudeStream(strings.NewReader("not json at all"), nil); err == nil {
		t.Error("unparseable output was accepted")
	}
}

// failAfter reads a string and then fails, standing in for a stream that dies
// after the result event: an oversized trailing line, or a pipe that breaks
// once the process is killed.
type failAfter struct {
	rest string
	err  error
}

func (f *failAfter) Read(p []byte) (int, error) {
	if f.rest == "" {
		return 0, f.err
	}
	n := copy(p, f.rest)
	f.rest = f.rest[n:]
	return n, nil
}

// TestAReadFailureAfterTheResultKeepsTheAccounting is the property ClaudeRun's
// whole shape exists for, at the one boundary that used to break it. The
// result event *is* the accounting, so once it has been parsed a later read
// error cannot take back the cost, the tokens and the answer — otherwise a run
// that spent money reports JobFailed with nothing added to the session total.
func TestAReadFailureAfterTheResultKeepsTheAccounting(t *testing.T) {
	r := &failAfter{
		rest: `{"type":"result","subtype":"success","result":"ZEPHYR_QUOTA_GUARD",` +
			`"total_cost_usd":0.42,"usage":{"output_tokens":9}}` + "\n",
		err: bufio.ErrTooLong,
	}
	run, err := readClaudeStream(r, nil)
	if err != nil {
		t.Fatalf("a read failure after the result discarded it: %v", err)
	}
	if run.CostUSD != 0.42 || run.Text != "ZEPHYR_QUOTA_GUARD" || run.Tokens.Output != 9 {
		t.Errorf("run = %+v, want the totals the result reported", run)
	}
}

// And the other side of it: a read failure with no result is still a failure,
// because there is genuinely nothing to report.
func TestAReadFailureWithNoResultIsStillAFailure(t *testing.T) {
	r := &failAfter{rest: `{"type":"system","subtype":"init"}` + "\n", err: bufio.ErrTooLong}
	if _, err := readClaudeStream(r, nil); err == nil {
		t.Error("a broken stream with no accounting was reported as a success")
	}
}

// TestAnUnreadableLineDoesNotLoseTheRun. The stream is several event kinds
// wide and gains more over time. Refusing the whole run because one line was
// unfamiliar would throw away the accounting carried by the next.
func TestAnUnreadableLineDoesNotLoseTheRun(t *testing.T) {
	stream := "{ this is not json\n" +
		`{"type":"result","subtype":"success","result":"ok","total_cost_usd":0.01}`
	run, err := readClaudeStream(strings.NewReader(stream), nil)
	if err != nil {
		t.Fatalf("one bad line lost a run that reported its totals: %v", err)
	}
	if run.Text != "ok" || run.CostUSD != 0.01 {
		t.Errorf("run = %+v", run)
	}
}
