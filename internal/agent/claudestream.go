package agent

// Reading what `claude -p --output-format stream-json` emits: one JSON object
// per line, and the totals in the result event at the end.

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// maxEventLine bounds one line of the stream.
//
// The result event carries the model's whole answer, so the longest line is as
// long as a review. Four megabytes is far above any answer measured and far
// below a size that would matter; a line past it is reported rather than
// truncated, because half a JSON object parses as nothing and would otherwise
// read as "the run produced no result".
const maxEventLine = 4 << 20

// claudeEvent is one line of the stream.
//
// Usage arrives in four different places depending on the event, which is why
// this carries the shape rather than a flat set of fields: the result event
// holds it at the top level, an assistant event under message, and a
// stream_event under event or under event.message.
type claudeEvent struct {
	Type string `json:"type"`

	// The result event's own fields. Absent everywhere else.
	Subtype    string  `json:"subtype"`
	IsError    bool    `json:"is_error"`
	Result     string  `json:"result"`
	DurationMS int     `json:"duration_ms"`
	CostUSD    float64 `json:"total_cost_usd"`
	Usage      *Tokens `json:"usage"`

	Message *usageHolder `json:"message"`
	Event   *struct {
		Type    string       `json:"type"`
		Usage   *Tokens      `json:"usage"`
		Message *usageHolder `json:"message"`
	} `json:"event"`
}

type usageHolder struct {
	Usage *Tokens `json:"usage"`
}

// usage is the accounting this event carries, or nil.
func (e claudeEvent) usage() *Tokens {
	switch {
	case e.Usage != nil:
		return e.Usage
	case e.Message != nil && e.Message.Usage != nil:
		return e.Message.Usage
	case e.Event == nil:
		return nil
	case e.Event.Usage != nil:
		return e.Event.Usage
	case e.Event.Message != nil && e.Event.Message.Usage != nil:
		return e.Event.Message.Usage
	}
	return nil
}

// readClaudeStream consumes the stream and returns what the run reported,
// calling onUsage with each accounting it passes.
//
// Only Output moves once the run has started: the first event of a turn
// already carries the final input, cache-read and cache-write counts, which is
// why a live display fills in almost at once and then creeps. Cost is not in
// any of them — it appears only in the result — so a run in flight can report
// how much it is using and not what it will cost.
//
// A line that does not parse is skipped rather than fatal. The stream is
// several event kinds wide and gains more over time, and refusing a run
// because one line was unfamiliar would throw away the accounting on the next.
func readClaudeStream(r io.Reader, onUsage func(Tokens)) (ClaudeRun, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64<<10), maxEventLine)

	var run ClaudeRun
	var got bool
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var e claudeEvent
		if json.Unmarshal(line, &e) != nil {
			continue
		}
		if u := e.usage(); u != nil && onUsage != nil {
			onUsage(*u)
		}
		if e.Type != "result" {
			continue
		}
		run = ClaudeRun{
			Text:    e.Result,
			CostUSD: e.CostUSD,
			Took:    time.Duration(e.DurationMS) * time.Millisecond,
		}
		if e.Usage != nil {
			run.Tokens = *e.Usage
		}
		if e.IsError {
			run.Text = ""
			run.Failure = strings.TrimSpace(e.Subtype + ": " + e.Result)
		}
		got = true
	}
	// The read error is subordinate to the result, not the other way round. got
	// is set only by a fully parsed result event, so once one has arrived the
	// accounting is in hand and a later oversized or unreadable line cannot
	// take it back. Returning the error here regardless would lose the cost,
	// the tokens and the answer of a run that had already reported all three —
	// the exact case ClaudeRun's shape exists to prevent.
	if err := sc.Err(); err != nil && !got {
		return ClaudeRun{}, fmt.Errorf("read claude output: %w", err)
	}
	if !got {
		// No result event means no accounting: whatever it spent, we cannot say.
		return ClaudeRun{}, fmt.Errorf("claude produced no result event")
	}
	return run, nil
}
