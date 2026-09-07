package coord

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// connectedPair registers one session on each of two projects and links them.
// The pair is the case connections exist for: an API changing in one
// repository while its consumer changes in another.
func connectedPair(t *testing.T, c *Coordinator, theirDir, head string) {
	t.Helper()
	c.Register(Session{ID: "me", ProjectID: "p1", Project: "gateway",
		Name: "scheming-hawk-jhgk", Dir: t.TempDir()})
	c.Register(Session{ID: "them", ProjectID: "p2", Project: "billing-service",
		Name: "wily-crane-bbbb", Title: "split the auth middleware",
		Dir: theirDir, Branch: "session/wily-crane-bbbb", Isolated: true, BaseRef: head})
	c.SetConnections([]Connection{{A: "me", B: "them"}})
}

// TestAConnectionReachesAcrossProjects is the whole point of the feature.
// TestWorkStaysInsideTheProject is its other half: the same two sessions
// without a connection cannot see each other at all.
func TestAConnectionReachesAcrossProjects(t *testing.T) {
	dir, head := worktreeSession(t)
	c := start(t)
	connectedPair(t, c, dir, head)

	if err := os.WriteFile(filepath.Join(dir, "auth.go"),
		[]byte("package gateway\n\nfunc Auth() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := c.Work("me", "wily-crane-bbbb")
	if err != nil {
		t.Fatalf("a connected session is unreachable: %v", err)
	}
	if !strings.Contains(out["patch"].(string), "func Auth()") {
		t.Errorf("patch missing the change: %v", out["patch"])
	}
	// Named because it crosses. Without it the reader resolves the paths in
	// that patch against its own tree, where they mean something else or
	// nothing.
	if got := out["project"]; got != "billing-service" {
		t.Errorf("project = %v, want billing-service", got)
	}
}

// TestWorkOnTheSameProjectDoesNotNameAProject is the other side of that rule:
// a result with no project is one from your own, which is what makes the
// field's presence meaningful.
func TestWorkOnTheSameProjectDoesNotNameAProject(t *testing.T) {
	dir, head := worktreeSession(t)
	c := start(t)
	c.Register(Session{ID: "me", ProjectID: "p1", Project: "gateway",
		Name: "scheming-hawk-jhgk", Dir: t.TempDir()})
	c.Register(Session{ID: "them", ProjectID: "p1", Project: "gateway",
		Name: "wily-crane-bbbb", Dir: dir, Isolated: true, BaseRef: head})

	out, err := c.Work("me", "wily-crane-bbbb")
	if err != nil {
		t.Fatal(err)
	}
	if _, named := out["project"]; named {
		t.Errorf("a same-project result names a project: %v", out["project"])
	}
}

// TestAConnectionDoesNotWidenClaims is the deliberate exception, and the one
// most likely to be "fixed" by a later reader who sees claims scoping to the
// project while everything beside it scopes to what a session can see.
//
// A claim is a repo-relative path. Two sessions in different repositories both
// working on internal/api/client.go are not in each other's way, and reporting
// a conflict would teach agents to ignore the mechanism.
func TestAConnectionDoesNotWidenClaims(t *testing.T) {
	c := start(t)
	c.Register(Session{ID: "me", ProjectID: "p1", Name: "scheming-hawk-jhgk", Dir: "/wt/mine"})
	c.Register(Session{ID: "them", ProjectID: "p2", Name: "wily-crane-bbbb", Dir: "/wt/theirs"})
	c.SetConnections([]Connection{{A: "me", B: "them"}})

	if granted, _ := c.Claim("me", []string{"internal/api/client.go"}, "rewrite"); len(granted) != 1 {
		t.Fatalf("granted = %v, want the path", granted)
	}
	granted, conflicts := c.Claim("them", []string{"internal/api/client.go"}, "consume")
	if len(conflicts) != 0 {
		t.Errorf("a connection made two repositories collide on one path: %v", conflicts)
	}
	if len(granted) != 1 {
		t.Errorf("granted = %v, want the path", granted)
	}
}

// TestSessionsNamesTheProjectOfAConnectedRow. A connected row's branch and
// claims belong to another repository, and read wrongly without saying so.
func TestSessionsNamesTheProjectOfAConnectedRow(t *testing.T) {
	dir, head := worktreeSession(t)
	c := start(t)
	connectedPair(t, c, dir, head)

	rows := c.Siblings("me")
	if len(rows) != 1 {
		t.Fatalf("rows = %v, want the connected session", rows)
	}
	if rows[0]["session"] != "wily-crane-bbbb" {
		t.Fatalf("wrong row: %v", rows[0])
	}
	if rows[0]["project"] != "billing-service" {
		t.Errorf("project = %v, want billing-service", rows[0]["project"])
	}
}

// TestMessagesReachAConnectedSession keeps the mailbox on the same rule as the
// listing. A session named by the sessions tool that cannot be written to would
// be worse than not listing it.
func TestMessagesReachAConnectedSession(t *testing.T) {
	dir, head := worktreeSession(t)
	c := start(t)
	connectedPair(t, c, dir, head)

	sent, err := c.Send("me", "wily-crane-bbbb", "the response shape changed")
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	if len(sent) != 1 || sent[0] != "wily-crane-bbbb" {
		t.Fatalf("sent = %v", sent)
	}
	if got := c.Collect("them"); len(got) != 1 || got[0].From != "scheming-hawk-jhgk" {
		t.Errorf("inbox = %v", got)
	}
}

// TestABroadcastReachesAConnectedSession guards the rule Send's doc comment
// states: an empty target reaches everyone the sessions tool lists, which now
// includes a connected session on another project.
//
// Without this the rule is unguarded, and worse than unguarded.
// TestBroadcastReachesEverySibling asserts that an unconnected session on
// another project receives nothing, so the suite reads as "a broadcast never
// leaves the project". Reverting Send to a project test would leave every test
// green while the documented behaviour was gone.
func TestABroadcastReachesAConnectedSession(t *testing.T) {
	dir, head := worktreeSession(t)
	c := start(t)
	connectedPair(t, c, dir, head)
	// A second session on the connected project, joined to nobody. It is the
	// half that proves the broadcast follows the connection rather than simply
	// going everywhere.
	c.Register(Session{ID: "other", ProjectID: "p2", Project: "billing-service",
		Name: "quiet-vole-cccc", Dir: t.TempDir()})

	sent, err := c.Send("me", "", "regenerating the schema")
	if err != nil {
		t.Fatalf("broadcast: %v", err)
	}
	if len(sent) != 1 || sent[0] != "wily-crane-bbbb" {
		t.Fatalf("delivered to %v, want only the connected session", sent)
	}
	if c.Unread("them") != 1 {
		t.Error("the connected session did not receive the broadcast")
	}
	if c.Unread("other") != 0 {
		t.Error("an unconnected session on the connected project received it")
	}
}

// TestNotesFromAConnectedSessionAreVisible covers the read half of the shared
// log, and TestNotesOfAStrangerOnThatProjectStayHidden covers the filter that
// keeps it from being the other project's whole log.
func TestNotesFromAConnectedSessionAreVisible(t *testing.T) {
	dir, head := worktreeSession(t)
	c := start(t)
	connectedPair(t, c, dir, head)

	if err := c.AppendNote("me", "starting on the client"); err != nil {
		t.Fatal(err)
	}
	if err := c.AppendNote("them", "the response shape changed"); err != nil {
		t.Fatal(err)
	}

	notes, err := c.Notes("me")
	if err != nil {
		t.Fatal(err)
	}
	var texts []string
	for _, n := range notes {
		texts = append(texts, n.Text)
	}
	if len(texts) != 2 {
		t.Fatalf("notes = %v, want mine and the connected one", texts)
	}
	// Oldest first, merged across two files rather than concatenated.
	if texts[0] != "starting on the client" || texts[1] != "the response shape changed" {
		t.Errorf("notes out of order: %v", texts)
	}
}

func TestNotesOfAStrangerOnThatProjectStayHidden(t *testing.T) {
	dir, head := worktreeSession(t)
	c := start(t)
	connectedPair(t, c, dir, head)
	// A third session on the connected project that nobody linked to.
	c.Register(Session{ID: "other", ProjectID: "p2", Project: "billing-service",
		Name: "quiet-vole-cccc", Dir: t.TempDir()})

	if err := c.AppendNote("other", "unrelated refactor in billing"); err != nil {
		t.Fatal(err)
	}
	notes, err := c.Notes("me")
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range notes {
		if n.Session == "quiet-vole-cccc" {
			t.Fatalf("a connection published the whole project log: %v", notes)
		}
	}
}

// TestAnUnreadableConnectedLogIsReported keeps one os.ReadFile failure to one
// answer. Notes already returns the error when the caller's own log will not
// read, and skipping the far one would tell an agent that a connected session
// had written nothing — the single conclusion a connection exists to stop it
// drawing.
//
// The log is made unreadable by putting a directory where the file goes, which
// fails for the same reason whatever user the tests run as. A missing file is
// not this case: readNotes turns that into an empty log on purpose.
func TestAnUnreadableConnectedLogIsReported(t *testing.T) {
	dir, head := worktreeSession(t)
	notes := t.TempDir()
	c, err := Start(notes)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	connectedPair(t, c, dir, head)

	if err := os.MkdirAll(filepath.Join(notes, "p2.jsonl"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Notes("me"); err == nil {
		t.Error("an unreadable connected log was skipped rather than reported")
	}
}

// TestSetConnectionsReplacesRatherThanAdds. The coordinator holds a copy of
// state the store owns, so the only safe update is the whole set: a stale link
// that survives a replace is one the user disconnected and the agents kept.
func TestSetConnectionsReplacesRatherThanAdds(t *testing.T) {
	dir, head := worktreeSession(t)
	c := start(t)
	connectedPair(t, c, dir, head)

	if rows := c.Siblings("me"); len(rows) != 1 {
		t.Fatalf("the fixture is not connected: %v", rows)
	}
	c.SetConnections(nil)
	if rows := c.Siblings("me"); len(rows) != 0 {
		t.Errorf("a disconnected session is still visible: %v", rows)
	}
}
