package coord

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"sort"
	"strings"
	"testing"
)

func start(t *testing.T) *Coordinator {
	t.Helper()
	c, err := Start(t.TempDir())
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func twoSessions(t *testing.T, c *Coordinator) (a, b Session) {
	t.Helper()
	a = Session{ID: "s1", ProjectID: "p1", Name: "swift-otter-aaaa",
		Title: "parser work", Branch: "session/swift-otter-aaaa", Dir: "/wt/swift-otter-aaaa"}
	b = Session{ID: "s2", ProjectID: "p1", Name: "wily-crane-bbbb",
		Title: "docs", Branch: "session/wily-crane-bbbb", Dir: "/wt/wily-crane-bbbb"}
	c.Register(a)
	c.Register(b)
	return a, b
}

// TestClaimsCollideAcrossWorktrees is the reason this package exists. Two
// sessions work in different worktrees, so the same file has a different
// absolute path in each. Without normalising to repo-relative, they would
// never conflict and the whole mechanism would be decorative.
func TestClaimsCollideAcrossWorktrees(t *testing.T) {
	c := start(t)
	twoSessions(t, c)

	granted, conflicts := c.Claim("s1", []string{"/wt/swift-otter-aaaa/src/parser.go"}, "rewrite")
	if len(granted) != 1 || granted[0] != "src/parser.go" {
		t.Fatalf("granted = %v, want [src/parser.go]", granted)
	}
	if len(conflicts) != 0 {
		t.Fatalf("unexpected conflicts: %v", conflicts)
	}

	// Same file, different worktree, absolute path.
	granted, conflicts = c.Claim("s2", []string{"/wt/wily-crane-bbbb/src/parser.go"}, "docs")
	if len(granted) != 0 {
		t.Errorf("granted %v, want none — the file is taken", granted)
	}
	if len(conflicts) != 1 {
		t.Fatalf("conflicts = %v, want one", conflicts)
	}
	if conflicts[0].Session != "swift-otter-aaaa" {
		t.Errorf("conflict names %q, want swift-otter-aaaa", conflicts[0].Session)
	}
}

func TestClaimIsPerProject(t *testing.T) {
	c := start(t)
	twoSessions(t, c)
	c.Register(Session{ID: "s3", ProjectID: "OTHER", Name: "other-one", Dir: "/wt/other"})

	c.Claim("s1", []string{"src/parser.go"}, "")
	// A different project claiming the same relative path must not conflict.
	_, conflicts := c.Claim("s3", []string{"src/parser.go"}, "")
	if len(conflicts) != 0 {
		t.Errorf("cross-project conflict: %v", conflicts)
	}
}

func TestReleaseFreesForOthers(t *testing.T) {
	c := start(t)
	twoSessions(t, c)

	c.Claim("s1", []string{"a.go", "b.go"}, "")
	if freed := c.Release("s1", []string{"a.go"}); len(freed) != 1 || freed[0] != "a.go" {
		t.Fatalf("freed = %v, want [a.go]", freed)
	}
	if granted, conflicts := c.Claim("s2", []string{"a.go"}, ""); len(granted) != 1 || len(conflicts) != 0 {
		t.Errorf("a.go not reclaimable: granted=%v conflicts=%v", granted, conflicts)
	}
	// b.go is still held.
	if _, conflicts := c.Claim("s2", []string{"b.go"}, ""); len(conflicts) != 1 {
		t.Error("b.go should still be held by s1")
	}
	if freed := c.Release("s1", nil); len(freed) != 1 || freed[0] != "b.go" {
		t.Errorf("release-all freed %v, want [b.go]", freed)
	}
}

// TestUnregisterReleasesClaims covers the reason claims live in memory: a
// claim held by a process that has exited is worse than no claim, because the
// next agent believes someone is working there.
func TestUnregisterReleasesClaims(t *testing.T) {
	c := start(t)
	twoSessions(t, c)

	c.Claim("s1", []string{"src/parser.go"}, "")
	c.Unregister("s1")

	granted, conflicts := c.Claim("s2", []string{"src/parser.go"}, "")
	if len(conflicts) != 0 {
		t.Errorf("a dead session still holds claims: %v", conflicts)
	}
	if len(granted) != 1 {
		t.Errorf("granted = %v, want the path", granted)
	}
}

func TestSiblingsExcludesSelfAndOtherProjects(t *testing.T) {
	c := start(t)
	twoSessions(t, c)
	c.Register(Session{ID: "s3", ProjectID: "OTHER", Name: "elsewhere", Dir: "/wt/other"})
	c.Claim("s2", []string{"docs/readme.md"}, "writing")

	got := c.Siblings("s1")
	if len(got) != 1 {
		t.Fatalf("siblings = %v, want exactly wily-crane-bbbb", got)
	}
	if got[0]["session"] != "wily-crane-bbbb" {
		t.Errorf("sibling = %v", got[0])
	}
	claims, _ := got[0]["claims"].([]string)
	if len(claims) != 1 || claims[0] != "docs/readme.md" {
		t.Errorf("sibling claims = %v", claims)
	}
}

func TestNotesRoundTripAndShare(t *testing.T) {
	c := start(t)
	twoSessions(t, c)

	if err := c.AppendNote("s1", "the parser owns tokenising, not the lexer"); err != nil {
		t.Fatalf("append: %v", err)
	}
	if err := c.AppendNote("s2", "docs regenerate from the schema"); err != nil {
		t.Fatalf("append: %v", err)
	}

	// s2 sees what s1 wrote: that is the whole point.
	notes, err := c.Notes("s2")
	if err != nil {
		t.Fatalf("notes: %v", err)
	}
	if len(notes) != 2 {
		t.Fatalf("got %d notes, want 2", len(notes))
	}
	if notes[0].Session != "swift-otter-aaaa" || !strings.Contains(notes[0].Text, "tokenising") {
		t.Errorf("first note = %+v", notes[0])
	}
}

func TestNotesAreCapped(t *testing.T) {
	c := start(t)
	twoSessions(t, c)
	for i := range maxNotes + 20 {
		if err := c.AppendNote("s1", "note "+string(rune('a'+i%26))); err != nil {
			t.Fatal(err)
		}
	}
	notes, err := c.Notes("s1")
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != maxNotes {
		t.Errorf("got %d notes, want the cap of %d — an uncapped log is billed to every agent that reads it",
			len(notes), maxNotes)
	}
}

func TestEmptyNoteRejected(t *testing.T) {
	c := start(t)
	twoSessions(t, c)
	if err := c.AppendNote("s1", "   "); err == nil {
		t.Error("empty note accepted")
	}
}

// ---- transport ----

func rpc(t *testing.T, c *Coordinator, sessionID, method string, params any) rpcResp {
	t.Helper()
	body := map[string]any{"jsonrpc": "2.0", "id": 1, "method": method}
	if params != nil {
		body["params"] = params
	}
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, c.server.urlFor(sessionID), bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s: %v", method, err)
	}
	defer res.Body.Close()

	var out rpcResp
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatalf("decode %s: %v", method, err)
	}
	return out
}

func TestMCPHandshakeAndTools(t *testing.T) {
	c := start(t)
	twoSessions(t, c)

	init := rpc(t, c, "s1", "initialize", map[string]any{"protocolVersion": "2025-06-18"})
	if init.Error != nil {
		t.Fatalf("initialize: %v", init.Error)
	}

	list := rpc(t, c, "s1", "tools/list", nil)
	raw, _ := json.Marshal(list.Result)
	for _, want := range []string{"sessions", "claim", "release", "note", "notes",
		"message", "inbox", "work", "analyse", "analysis"} {
		if !strings.Contains(string(raw), `"`+want+`"`) {
			t.Errorf("tools/list is missing %q", want)
		}
	}
}

// TestMCPIdentifiesCallerByURL pins the identification scheme: the session id
// is in the path, so an agent cannot present itself as a different session
// without having been handed that URL.
func TestMCPIdentifiesCallerByURL(t *testing.T) {
	c := start(t)
	twoSessions(t, c)

	call := rpc(t, c, "s1", "tools/call", map[string]any{
		"name":      "claim",
		"arguments": map[string]any{"paths": []string{"src/parser.go"}, "reason": "rewrite"},
	})
	if call.Error != nil {
		t.Fatalf("claim: %v", call.Error)
	}

	// s2 asking for siblings must see s1 holding it.
	got := rpc(t, c, "s2", "tools/call", map[string]any{
		"name": "sessions", "arguments": map[string]any{},
	})
	raw, _ := json.Marshal(got.Result)
	if !strings.Contains(string(raw), "swift-otter-aaaa") || !strings.Contains(string(raw), "src/parser.go") {
		t.Errorf("sessions result did not attribute the claim: %s", raw)
	}
}

// TestMCPAcceptsSSE covers the framing current claude asks for.
func TestMCPAcceptsSSE(t *testing.T) {
	c := start(t)
	twoSessions(t, c)

	b, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/list"})
	req, _ := http.NewRequest(http.MethodPost, c.server.urlFor("s1"), bytes.NewReader(b))
	req.Header.Set("Accept", "application/json, text/event-stream")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer res.Body.Close()

	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(res.Body)
	if !strings.HasPrefix(buf.String(), "event: message\ndata: ") {
		t.Errorf("SSE framing missing, got: %q", buf.String())
	}
}

func TestMCPRejectsUnknownMethod(t *testing.T) {
	c := start(t)
	twoSessions(t, c)
	got := rpc(t, c, "s1", "nonsense/method", nil)
	if got.Error == nil {
		t.Fatal("unknown method accepted")
	}
}

// ---- messages ----

func TestSendAndCollect(t *testing.T) {
	c := start(t)
	twoSessions(t, c)

	sent, err := c.Send("s1", "wily-crane-bbbb", "I hold the parser, take the lexer")
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if len(sent) != 1 || sent[0] != "wily-crane-bbbb" {
		t.Fatalf("delivered to %v", sent)
	}
	if n := c.Unread("s2"); n != 1 {
		t.Errorf("unread = %d, want 1", n)
	}

	got := c.Collect("s2")
	if len(got) != 1 || got[0].From != "swift-otter-aaaa" {
		t.Fatalf("collected %+v", got)
	}
	if !strings.Contains(got[0].Text, "take the lexer") {
		t.Errorf("text = %q", got[0].Text)
	}
	// Collecting empties the box.
	if n := c.Unread("s2"); n != 0 {
		t.Errorf("unread after collect = %d, want 0", n)
	}
}

func TestBroadcastReachesEverySibling(t *testing.T) {
	c := start(t)
	twoSessions(t, c)
	c.Register(Session{ID: "s3", ProjectID: "p1", Name: "third-one", Dir: "/wt/third"})
	c.Register(Session{ID: "s4", ProjectID: "OTHER", Name: "elsewhere", Dir: "/wt/other"})

	sent, err := c.Send("s1", "", "heads up: regenerating the schema")
	if err != nil {
		t.Fatalf("broadcast: %v", err)
	}
	if len(sent) != 2 {
		t.Fatalf("delivered to %v, want both siblings on p1", sent)
	}
	if c.Unread("s1") != 0 {
		t.Error("sender got its own broadcast")
	}
	if c.Unread("s4") != 0 {
		t.Error("a session on another project received the broadcast")
	}
}

func TestSendToUnknownSessionFails(t *testing.T) {
	c := start(t)
	twoSessions(t, c)
	if _, err := c.Send("s1", "nobody-here", "hello"); err == nil {
		t.Error("sending to a non-existent session succeeded")
	}
}

func TestInboxIsBounded(t *testing.T) {
	c := start(t)
	twoSessions(t, c)
	for range maxInbox + 25 {
		if _, err := c.Send("s1", "wily-crane-bbbb", "spam"); err != nil {
			t.Fatal(err)
		}
	}
	if n := c.Unread("s2"); n != maxInbox {
		t.Errorf("inbox = %d, want the cap of %d", n, maxInbox)
	}
}

func TestUnregisterClearsInbox(t *testing.T) {
	c := start(t)
	twoSessions(t, c)
	c.Send("s1", "wily-crane-bbbb", "hello")
	c.Unregister("s2")
	if n := c.Unread("s2"); n != 0 {
		t.Errorf("gone session still holds %d messages", n)
	}
}

// TestUnreadHintRidesOtherResults covers how a recipient finds out it has
// mail at all: delivery is pull-only, so the count is appended to every other
// tool result.
func TestUnreadHintRidesOtherResults(t *testing.T) {
	c := start(t)
	twoSessions(t, c)
	c.Send("s1", "wily-crane-bbbb", "look at the lexer")

	got := rpc(t, c, "s2", "tools/call", map[string]any{
		"name": "sessions", "arguments": map[string]any{},
	})
	raw, _ := json.Marshal(got.Result)
	if !strings.Contains(string(raw), "inbox tool") {
		t.Errorf("no unread hint on the sessions result: %s", raw)
	}

	// The inbox's own result must not carry the hint about itself.
	got = rpc(t, c, "s2", "tools/call", map[string]any{
		"name": "inbox", "arguments": map[string]any{},
	})
	raw, _ = json.Marshal(got.Result)
	if strings.Contains(string(raw), "inbox tool") {
		t.Errorf("inbox result hints at itself: %s", raw)
	}
}

// ---- note pruning ----

// TestNotesCompactOnDisk is the pruning guard. Reading was already capped, but
// without trimming the file itself an append-only log grows for the life of
// the project.
func TestNotesCompactOnDisk(t *testing.T) {
	c := start(t)
	twoSessions(t, c)

	for range compactAbove + 30 {
		if err := c.AppendNote("s1", "a decision worth keeping"); err != nil {
			t.Fatal(err)
		}
	}

	onDisk := countLines(c.notesPath("p1"))
	if onDisk > compactAbove {
		t.Errorf("log holds %d lines, want it trimmed below %d", onDisk, compactAbove)
	}
	if onDisk < keepNotes {
		t.Errorf("log holds %d lines, want at least the %d most recent kept", onDisk, keepNotes)
	}
	// Trimming must not break reading.
	notes, err := c.Notes("s1")
	if err != nil {
		t.Fatalf("notes after compaction: %v", err)
	}
	if len(notes) != maxNotes {
		t.Errorf("read %d notes, want %d", len(notes), maxNotes)
	}
}

// TestCompactKeepsTheNewest checks it trims the right end.
func TestCompactKeepsTheNewest(t *testing.T) {
	c := start(t)
	twoSessions(t, c)

	for i := range compactAbove + 5 {
		if err := c.AppendNote("s1", fmt.Sprintf("note-%d", i)); err != nil {
			t.Fatal(err)
		}
	}
	notes, err := c.Notes("s1")
	if err != nil {
		t.Fatal(err)
	}
	last := notes[len(notes)-1].Text
	if want := fmt.Sprintf("note-%d", compactAbove+4); last != want {
		t.Errorf("newest note = %q, want %q — compaction kept the wrong end", last, want)
	}
}

// ---- reported status ----

// TestHookReportsArriveByURL covers the identification scheme for status: the
// event's meaning is the last path segment, so the body is never parsed and a
// Notification subtype is distinguished by which endpoint its matcher points
// at rather than by an undocumented field.
func TestHookReportsArriveByURL(t *testing.T) {
	c := start(t)
	twoSessions(t, c)

	if _, reported := c.StateOf("s1"); reported {
		t.Fatal("a state was reported before any hook fired")
	}

	for _, tc := range []struct {
		kind string
		want State
	}{{"working", StateWorking}, {"waiting", StateWaiting}, {"idle", StateIdle}} {
		res, err := http.Post(c.server.hookURL("s1", tc.kind), "application/json",
			strings.NewReader(`{"hook_event_name":"whatever"}`))
		if err != nil {
			t.Fatalf("post %s: %v", tc.kind, err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusOK {
			t.Errorf("%s returned %d, want 200", tc.kind, res.StatusCode)
		}
		got, reported := c.StateOf("s1")
		if !reported || got != tc.want {
			t.Errorf("after %s: state = %v (reported=%v), want %v", tc.kind, got, reported, tc.want)
		}
	}
}

// TestHookRepliesEmpty matters because several hook events accept a JSON
// decision object. A status ping that answered with one could block a tool
// call or a prompt.
func TestHookRepliesEmpty(t *testing.T) {
	c := start(t)
	twoSessions(t, c)

	res, err := http.Post(c.server.hookURL("s1", "idle"), "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if len(body) != 0 {
		t.Errorf("hook reply carried a body %q; anything parseable is a decision", body)
	}
}

func TestHookRejectsUnknownKind(t *testing.T) {
	c := start(t)
	twoSessions(t, c)
	res, err := http.Post(c.server.hookURL("s1", "nonsense"), "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Errorf("unknown hook kind returned %d, want 404", res.StatusCode)
	}
	if _, reported := c.StateOf("s1"); reported {
		t.Error("an unknown kind still recorded a state")
	}
}

// TestUnregisterClearsReportedState keeps a dead session from leaving a stale
// dot behind, the same rule claims and mail already follow.
func TestUnregisterClearsReportedState(t *testing.T) {
	c := start(t)
	twoSessions(t, c)
	c.status.set("s1", StateWorking)
	c.Unregister("s1")
	if _, reported := c.StateOf("s1"); reported {
		t.Error("a gone session still reports a state")
	}
}

// TestHooksConfigCoversTurnBoundaries pins the generated --settings payload.
//
// Every way a turn can open or close needs an event, and three are easy to
// forget: the one that clears the "needs you" dot after a permission is
// granted, the one that reports a turn ending in failure rather than normally,
// and the elicitation notifications, which block on a person exactly like a
// permission prompt does.
func TestHooksConfigCoversTurnBoundaries(t *testing.T) {
	c := start(t)
	twoSessions(t, c)

	var cfg struct {
		Hooks map[string][]struct {
			Matcher string `json:"matcher"`
			Hooks   []struct {
				Type    string `json:"type"`
				URL     string `json:"url"`
				Timeout int    `json:"timeout"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	raw := c.HooksConfigJSON("s1")
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatalf("generated settings are not valid JSON: %v\n%s", err, raw)
	}

	// Each event must exist and post to the endpoint whose meaning matches it.
	// Asserting the endpoint and not just the presence is what catches
	// StopFailure wired to /idle, which would report a rate-limited turn as a
	// finished one.
	wantKind := map[string]string{
		"UserPromptSubmit": "working",
		"PostToolBatch":    "working",
		"Stop":             "idle",
		"StopFailure":      "waiting",
		"Notification":     "waiting",
	}
	for ev, kind := range wantKind {
		entries := cfg.Hooks[ev]
		if len(entries) == 0 {
			t.Errorf("no handler for %s", ev)
			continue
		}
		for _, e := range entries {
			for _, h := range e.Hooks {
				if !strings.HasSuffix(h.URL, "/"+kind) {
					t.Errorf("%s posts to %q, want the %s endpoint", ev, h.URL, kind)
				}
			}
		}
	}
	if n := len(cfg.Hooks); n != len(wantKind) {
		t.Errorf("config carries %d events, want %d — an untested event is an unreported state",
			n, len(wantKind))
	}

	var matchers []string
	for _, e := range cfg.Hooks["Notification"] {
		matchers = append(matchers, e.Matcher)
	}
	sort.Strings(matchers)
	// Spelled out rather than read from waitingNotifications: comparing the
	// generated config against the list that generates it asserts nothing, and
	// passes just as happily with an empty list.
	want := []string{
		"agent_needs_input",
		"elicitation_dialog",
		"elicitation_url_dialog",
		"idle_prompt",
		"permission_prompt",
	}
	if !slices.Equal(matchers, want) {
		t.Errorf("Notification matchers = %v, want %v", matchers, want)
	}

	// Guarded, not indexed: a missing event here is the regression this test
	// exists to catch, and an index panic aborts the whole package run rather
	// than printing the diagnosis above.
	for _, ev := range []string{"Stop", "StopFailure"} {
		for _, e := range cfg.Hooks[ev] {
			if e.Matcher != "" {
				t.Errorf("%s carries matcher %q; the event takes none", ev, e.Matcher)
			}
		}
	}

	for ev, entries := range cfg.Hooks {
		for _, e := range entries {
			for _, h := range e.Hooks {
				if h.Type != "http" {
					t.Errorf("%s handler type = %q, want http", ev, h.Type)
				}
				if h.Timeout <= 0 || h.Timeout > 30 {
					t.Errorf("%s timeout = %d; the 600s default would stall a turn when Deck is gone",
						ev, h.Timeout)
				}
				if !strings.Contains(h.URL, "/hooks/s1/") {
					t.Errorf("%s url %q does not identify the session", ev, h.URL)
				}
			}
		}
	}
}
