package coord

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tripledownab/deck/internal/agent"
)

// analysable registers a reader and a sibling with real, uncommitted work, and
// substitutes the spawn so the suite neither spends money nor needs the CLI.
//
// run is what a spawned review will report. Without this the tests invoke the
// real claude: a probe measured one at $0.106, and in CI, where it is not
// installed, the spawn would fail while the tests still passed.
func analysable(t *testing.T, run agent.ClaudeRun, spawnErr error) *Coordinator {
	t.Helper()
	return analysableWatching(t, run, spawnErr, nil)
}

// analysableWatching is analysable, recording the prompt each spawn is given.
// The prompt is the only place a caller's question becomes visible, so without
// this nothing proves the question reaches the reviewer at all.
func analysableWatching(t *testing.T, run agent.ClaudeRun, spawnErr error, seen *[]string) *Coordinator {
	t.Helper()
	dir, head := worktreeSession(t)
	if err := os.WriteFile(filepath.Join(dir, "auth.go"),
		[]byte("package gateway\n\nfunc Auth() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	c := startWith(t, func(_ context.Context, _, prompt string) (agent.ClaudeRun, error) {
		if seen != nil {
			mu.Lock()
			*seen = append(*seen, prompt)
			mu.Unlock()
		}
		return run, spawnErr
	})
	c.Register(Session{ID: "me", ProjectID: "p1", Name: "scheming-hawk-jhgk", Dir: t.TempDir()})
	c.Register(Session{ID: "them", ProjectID: "p1", Name: "wily-crane-bbbb",
		Title: "split the auth middleware", Dir: dir,
		Branch: "session/wily-crane-bbbb", Isolated: true, BaseRef: head})
	return c
}

// TestAnalyseRefusesTheSameThingsWorkDoes is why both go through workOf. The
// scoping and the two refusals are stated once; if they drifted, a spawned run
// could read a session the work tool would not show.
func TestAnalyseRefusesTheSameThingsWorkDoes(t *testing.T) {
	c := analysable(t, agent.ClaudeRun{}, nil)
	c.Register(Session{ID: "shared", ProjectID: "p1", Name: "brisk-heron-cccc",
		Dir: t.TempDir(), Isolated: false})
	c.Register(Session{ID: "far", ProjectID: "OTHER", Name: "quiet-lynx-dddd",
		Dir: t.TempDir(), Isolated: true, BaseRef: "abc"})

	for _, tc := range []struct{ target, want string }{
		{"brisk-heron-cccc", "project directory"},
		{"quiet-lynx-dddd", "no live session"},
		{"not-a-session", "no live session"},
	} {
		_, err := c.Analyse("me", tc.target, "")
		if err == nil {
			t.Errorf("%s: spawned a run that work would have refused", tc.target)
			continue
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: error %q does not mention %q", tc.target, err, tc.want)
		}
	}
}

// TestAnalyseReturnsBeforeItFinishes is the whole point of the handle. A
// synchronous answer would outlast the calling agent's tool timeout, which
// belongs to that agent rather than to us.
func TestAnalyseReturnsBeforeItFinishes(t *testing.T) {
	c := analysable(t, agent.ClaudeRun{}, nil)

	start := time.Now()
	job, err := c.Analyse("me", "wily-crane-bbbb", "")
	if err != nil {
		t.Fatal(err)
	}
	if took := time.Since(start); took > 2*time.Second {
		t.Errorf("Analyse blocked for %v; it must hand back a handle", took)
	}
	if job.State != JobRunning {
		t.Errorf("state = %v, want running", job.State)
	}
	if job.Subject != "wily-crane-bbbb" {
		t.Errorf("subject = %q", job.Subject)
	}
}

// TestAnalysisIsPrivateToTheSessionThatPaid keeps one agent from collecting
// another's answer. A review is work someone spent money on.
func TestAnalysisIsPrivateToTheSessionThatPaid(t *testing.T) {
	c := analysable(t, agent.ClaudeRun{}, nil)
	c.Register(Session{ID: "other", ProjectID: "p1", Name: "brisk-heron-cccc", Dir: t.TempDir()})

	job, err := c.Analyse("me", "wily-crane-bbbb", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Analysis("other", job.ID); err == nil {
		t.Error("a different session collected an analysis it did not start")
	}
	if _, err := c.Analysis("me", job.ID); err != nil {
		t.Errorf("the session that started it cannot collect it: %v", err)
	}
}

// TestRunningJobReportsHowLongItHasWaited covers the field that is only
// written on completion. A job still in flight would otherwise say 0s, which
// is the one moment the caller most wants the number.
func TestRunningJobReportsHowLongItHasWaited(t *testing.T) {
	c := analysable(t, agent.ClaudeRun{}, nil)
	job, err := c.Analyse("me", "wily-crane-bbbb", "")
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(1100 * time.Millisecond)

	got, err := c.Analysis("me", job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State == JobRunning && got.Elapsed < time.Second {
		t.Errorf("a job running for over a second reports %v", got.Elapsed)
	}
}

// TestAnUnusableRunRecordsNoCost is the other half. An error from RunClaude
// means no envelope came back, so there is no figure to add — inventing one
// would be worse than the gap it fills.
func TestAnUnusableRunRecordsNoCost(t *testing.T) {
	c := analysable(t, agent.ClaudeRun{}, errors.New("claude: command not found"))

	job, err := c.Analyse("me", "wily-crane-bbbb", "")
	if err != nil {
		t.Fatal(err)
	}
	got := waitFor(t, c, job.ID)

	if got.State != JobFailed {
		t.Errorf("state = %v, want failed", got.State)
	}
	if !strings.Contains(got.Err, "command not found") {
		t.Errorf("error = %q", got.Err)
	}
	if c.Spend("me") != 0 {
		t.Errorf("spend = %v; a run that never happened was billed", c.Spend("me"))
	}
}

// TestJobsAreBounded matches how the inbox and the log are bounded. Each
// record holds a full review, so a session that keeps asking would otherwise
// grow without limit.
func TestJobsAreBounded(t *testing.T) {
	c := analysable(t, agent.ClaudeRun{Text: "ok", CostUSD: 0.01}, nil)

	var first string
	for i := range maxJobs + 5 {
		job, err := c.Analyse("me", "wily-crane-bbbb", "")
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			first = job.ID
		}
	}
	c.mu.Lock()
	kept := 0
	for _, j := range c.jobs {
		if j.From == "me" {
			kept++
		}
	}
	c.mu.Unlock()
	if kept != maxJobs {
		t.Errorf("kept %d analyses, want the bound of %d", kept, maxJobs)
	}
	if _, err := c.Analysis("me", first); err == nil {
		t.Error("the oldest analysis survived the bound")
	}
}

// TestUnregisterDropsJobsAndSpend matches how claims and the inbox behave: a
// session's record goes with the session.
func TestUnregisterDropsJobsAndSpend(t *testing.T) {
	c := analysable(t, agent.ClaudeRun{}, nil)
	job, err := c.Analyse("me", "wily-crane-bbbb", "")
	if err != nil {
		t.Fatal(err)
	}
	c.Unregister("me")

	if _, err := c.Analysis("me", job.ID); err == nil {
		t.Error("an unregistered session's analysis is still readable")
	}
	if got := c.Spend("me"); got != 0 {
		t.Errorf("spend = %v after unregister, want 0", got)
	}
}

// TestSpendAccumulatesAcrossRuns covers the number the UI will show. A per-run
// figure alone is hard to read: the same short turn measured at $0.012 and
// $0.237 depending on whether its context was read from cache or written to
// it, so the running total is what makes a pattern visible.
func TestSpendAccumulatesAcrossRuns(t *testing.T) {
	c := analysable(t, agent.ClaudeRun{Text: "looks fine", CostUSD: 0.05,
		Tokens: agent.Tokens{Input: 10, Output: 20, CacheWrite: 23698}}, nil)

	for range 3 {
		job, err := c.Analyse("me", "wily-crane-bbbb", "")
		if err != nil {
			t.Fatal(err)
		}
		waitFor(t, c, job.ID)
	}
	if got := c.Spend("me"); got < 0.149 || got > 0.151 {
		t.Errorf("spend = %v, want three runs at 0.05", got)
	}
}

// TestAFailedRunStillCountsAgainstSpend keeps the total honest. A turn that
// refused after spending is still spending, and a total counting only
// successes would understate the bill in exactly the case worth noticing.
func TestAFailedRunStillCountsAgainstSpend(t *testing.T) {
	// The shape RunClaude really returns for a refused turn: a result that
	// carries its cost, with Failure set. The earlier version of this test
	// paired a cost with a Go error, which no production path produces, so it
	// asserted a property nothing implemented.
	c := analysable(t, agent.ClaudeRun{CostUSD: 0.09, Failure: "error_max_turns: ran out of turns"}, nil)

	job, err := c.Analyse("me", "wily-crane-bbbb", "")
	if err != nil {
		t.Fatal(err)
	}
	got := waitFor(t, c, job.ID)

	if got.State != JobFailed {
		t.Errorf("state = %v, want failed", got.State)
	}
	if !strings.Contains(got.Err, "ran out of turns") {
		t.Errorf("error = %q", got.Err)
	}
	if c.Spend("me") != 0.09 {
		t.Errorf("spend = %v; a failed run that cost money was not counted", c.Spend("me"))
	}
}

// TestFinishedJobCarriesItsUsage pins the split the bill is explained by.
func TestFinishedJobCarriesItsUsage(t *testing.T) {
	want := agent.Tokens{Input: 10, Output: 20, CacheRead: 5, CacheWrite: 23698}
	c := analysable(t, agent.ClaudeRun{Text: "ok", CostUSD: 0.05, Tokens: want}, nil)

	job, err := c.Analyse("me", "wily-crane-bbbb", "")
	if err != nil {
		t.Fatal(err)
	}
	got := waitFor(t, c, job.ID)
	if got.Tokens != want {
		t.Errorf("tokens = %+v, want %+v", got.Tokens, want)
	}
	if got.Answer != "ok" {
		t.Errorf("answer = %q", got.Answer)
	}
}

// waitFor blocks until a job leaves the running state.
func waitFor(t *testing.T, c *Coordinator, id string) *Job {
	t.Helper()
	for range 100 {
		job, err := c.Analysis("me", id)
		if err != nil {
			t.Fatal(err)
		}
		if job.State != JobRunning {
			return job
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("job %s never finished", id)
	return nil
}

// toolPayload decodes what a tool returned. An MCP result carries the tool's
// JSON as text inside its content, so a test that string-matches the outer
// envelope is matching escaped JSON and breaks on any reformatting.
func toolPayload(t *testing.T, result any) map[string]any {
	t.Helper()
	var env struct {
		Content []struct{ Text string } `json:"content"`
	}
	raw, _ := json.Marshal(result)
	if err := json.Unmarshal(raw, &env); err != nil || len(env.Content) == 0 {
		t.Fatalf("not a tool result: %s", raw)
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(env.Content[0].Text), &out); err != nil {
		t.Fatalf("tool payload is not JSON: %s", env.Content[0].Text)
	}
	return out
}

// TestAnalyseIsCallableOverMCP covers the wiring rather than the method, and
// exists because the same gap was found in the work tool and then repeated
// here: renaming either case name left the whole suite green while both tools
// appeared in tools/list and failed for a real agent.
//
// It drives the full round trip — start through tools/call, collect through
// tools/call — with the spawn stubbed, so it proves the dispatch and the
// argument names without costing anything.
func TestAnalyseIsCallableOverMCP(t *testing.T) {
	c := analysable(t, agent.ClaudeRun{
		Text: "the retry budget is unbounded", CostUSD: 0.05,
		Tokens: agent.Tokens{Input: 10, Output: 20}, Took: 3 * time.Second,
	}, nil)

	started := rpc(t, c, "me", "tools/call", map[string]any{
		"name":      "analyse",
		"arguments": map[string]any{"session": "wily-crane-bbbb", "question": "anything risky?"},
	})
	if started.Error != nil {
		t.Fatalf("analyse: %v", started.Error)
	}
	begun := toolPayload(t, started.Result)
	if begun["session"] != "wily-crane-bbbb" {
		t.Fatalf("analyse did not report the subject: %v", begun)
	}
	id, _ := begun["id"].(string)
	if id == "" {
		t.Fatalf("analyse returned no id: %v", begun)
	}

	var done map[string]any
	for range 100 {
		got := rpc(t, c, "me", "tools/call", map[string]any{
			"name": "analysis", "arguments": map[string]any{"id": id},
		})
		if got.Error != nil {
			t.Fatalf("analysis: %v", got.Error)
		}
		done = toolPayload(t, got.Result)
		if done["state"] == "done" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if done["state"] != "done" {
		t.Fatalf("analysis never reported done: %v", done)
	}
	if done["answer"] != "the retry budget is unbounded" {
		t.Errorf("answer = %v", done["answer"])
	}
	if done["cost_usd"] != 0.05 {
		t.Errorf("cost_usd = %v, want 0.05", done["cost_usd"])
	}
	if done["session_spend_usd"] != 0.05 {
		t.Errorf("session_spend_usd = %v, want the running total", done["session_spend_usd"])
	}
	if done["took"] != "3s" {
		t.Errorf("took = %v, want the duration the run reported", done["took"])
	}
}

// TestTheQuestionReachesTheReviewer covers the argument the round-trip test
// cannot see. The question only becomes visible in the prompt, so renaming its
// JSON tag left every other assertion green while the reviewer silently got
// the general review instead of what was asked.
func TestTheQuestionReachesTheReviewer(t *testing.T) {
	var prompts []string
	c := analysableWatching(t, agent.ClaudeRun{Text: "ok"}, nil, &prompts)

	const asked = "does the retry budget have an upper bound?"
	got := rpc(t, c, "me", "tools/call", map[string]any{
		"name":      "analyse",
		"arguments": map[string]any{"session": "wily-crane-bbbb", "question": asked},
	})
	if got.Error != nil {
		t.Fatal(got.Error)
	}
	waitFor(t, c, toolPayload(t, got.Result)["id"].(string))

	if len(prompts) != 1 {
		t.Fatalf("spawned %d reviewers, want 1", len(prompts))
	}
	if !strings.Contains(prompts[0], asked) {
		t.Errorf("the question never reached the reviewer:\n%s", prompts[0])
	}
	// The diff has to be there too: the reviewer is given no tools, so
	// anything missing from the prompt is unavailable to it.
	for _, want := range []string{"auth.go", "wily-crane-bbbb"} {
		if !strings.Contains(prompts[0], want) {
			t.Errorf("prompt is missing %q, which the reviewer cannot look up", want)
		}
	}
}

// TestAGeneralReviewGetsTheDefaultQuestion covers the other branch: omitting
// the question must still ask something.
func TestAGeneralReviewGetsTheDefaultQuestion(t *testing.T) {
	var prompts []string
	c := analysableWatching(t, agent.ClaudeRun{Text: "ok"}, nil, &prompts)

	job, err := c.Analyse("me", "wily-crane-bbbb", "   ")
	if err != nil {
		t.Fatal(err)
	}
	waitFor(t, c, job.ID)

	if len(prompts) != 1 || !strings.Contains(prompts[0], defaultQuestion) {
		t.Errorf("a blank question did not fall back to the default:\n%v", prompts)
	}
}

// TestCloseStopsARunningAnalysis ties spawned work to the app that asked for
// it. Without it, quitting Deck mid-review left a claude process running and
// billing for as long as the job timeout allowed, with no surface left to
// show it on — the opposite of the visibility the cost reporting exists for.
func TestCloseStopsARunningAnalysis(t *testing.T) {
	cancelled := make(chan bool, 1)
	c := analysableWith(t, func(ctx context.Context, _, _ string) (agent.ClaudeRun, error) {
		select {
		case <-ctx.Done():
			cancelled <- true
		case <-time.After(5 * time.Second):
			cancelled <- false
		}
		return agent.ClaudeRun{}, ctx.Err()
	})

	if _, err := c.Analyse("me", "wily-crane-bbbb", ""); err != nil {
		t.Fatal(err)
	}
	// Let the goroutine reach the spawn before closing, so this tests
	// cancellation rather than a race to start.
	time.Sleep(100 * time.Millisecond)
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}

	select {
	case got := <-cancelled:
		if !got {
			t.Error("the spawned run survived Close; it will keep billing unattended")
		}
	case <-time.After(3 * time.Second):
		t.Error("the spawned run never reported; Close did not reach it")
	}
}

// startWith is start with a substituted reviewer.
func startWith(t *testing.T, r Reviewer) *Coordinator {
	t.Helper()
	c, err := Start(t.TempDir(), WithReviewer(r))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// analysableWith is analysable with a reviewer the caller controls, for tests
// about the run's lifetime rather than its result.
func analysableWith(t *testing.T, r Reviewer) *Coordinator {
	t.Helper()
	dir, head := worktreeSession(t)
	c := startWith(t, r)
	c.Register(Session{ID: "me", ProjectID: "p1", Name: "scheming-hawk-jhgk", Dir: t.TempDir()})
	c.Register(Session{ID: "them", ProjectID: "p1", Name: "wily-crane-bbbb",
		Title: "split the auth middleware", Dir: dir,
		Branch: "session/wily-crane-bbbb", Isolated: true, BaseRef: head})
	return c
}
