package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"context"
	"github.com/tripledownab/deck/internal/agent"
	"github.com/tripledownab/deck/internal/coord"
	"github.com/tripledownab/deck/internal/gittest"
	"github.com/tripledownab/deck/internal/store"
)

// TestStatusOfPrefersTruthOverGuess pins the precedence in statusOf.
//
// internal/coord tests prove the coordinator records what hooks report. None
// of them touch the function that turns a reported state into what a person
// sees, and that is where the order matters: put the coordinator's answer
// above the Exited check and a dead agent reads "Working" forever, with every
// coord test still green. The same wiring-versus-mechanism gap that
// TestSweepReleasesSelfExitedSession exists for.
func TestStatusOfPrefersTruthOverGuess(t *testing.T) {
	for _, tc := range []struct {
		name string
		// script is empty when the session has no runner at all.
		script   string
		exited   bool
		report   coord.State
		reported bool
		// staleAfter overrides how long a "working" report outlives a silent
		// pane. Zero leaves the shipped value.
		staleAfter time.Duration
		wantGlyph  string
		wantLabel  string
	}{{
		name:      "a session that was never opened is closed, not guessed at",
		wantGlyph: glyphClosed,
		wantLabel: "Closed",
	}, {
		name:      "a dead process outranks whatever it last reported",
		script:    "exit 0",
		exited:    true,
		report:    coord.StateWorking,
		reported:  true,
		wantGlyph: glyphExited,
		wantLabel: "Exited",
	}, {
		name:      "a reported wait is the dot the heuristic could never show",
		script:    "sleep 30",
		report:    coord.StateWaiting,
		reported:  true,
		wantGlyph: glyphWaiting,
		wantLabel: "Needs you",
	}, {
		name:      "a reported state outranks a quiet PTY",
		script:    "sleep 30",
		report:    coord.StateWorking,
		reported:  true,
		wantGlyph: glyphWorking,
		wantLabel: "Working",
	}, {
		name:      "with nothing reported the heuristic still answers",
		script:    "sleep 30",
		wantGlyph: glyphIdle,
		wantLabel: "Idle",
	}, {
		// An interrupted turn ends without an event to observe, so the
		// report would otherwise say Working until the next prompt.
		name:       "a working report that outlived the turn gives way to a silent pane",
		script:     "sleep 30",
		report:     coord.StateWorking,
		reported:   true,
		staleAfter: time.Nanosecond,
		wantGlyph:  glyphIdle,
		wantLabel:  "Idle",
	}} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.staleAfter != 0 {
				restore := staleWorkingReport
				staleWorkingReport = tc.staleAfter
				t.Cleanup(func() { staleWorkingReport = restore })
			}
			c, err := coord.Start(t.TempDir())
			if err != nil {
				t.Fatalf("coordinator: %v", err)
			}
			t.Cleanup(func() { _ = c.Close() })

			st := &store.State{}
			p := st.AddProject(store.Project{Name: "demo", Path: t.TempDir()})
			sess := st.AddSession(store.Session{
				ProjectID: p.ID, Name: "swift-otter-aaaa", Title: "work", Dir: t.TempDir(),
			})
			m := New(st, "bash", nil).WithCoordinator(c)

			if tc.script != "" {
				r := startAgent(t, sess.Dir, tc.script)
				m.runners[sess.ID] = r
				if tc.exited {
					waitExited(t, r)
				} else if r.Status() != agent.Idle {
					// The live cases all run a silent command, so the heuristic
					// must say Idle. If it says Working the two cases that assert
					// a reported state stop discriminating — the heuristic would
					// produce the same answer on its own.
					t.Fatalf("the PTY is not quiet (status %v); this case cannot "+
						"tell a reported state from the heuristic", r.Status())
				}
			}
			if tc.reported {
				c.Report(sess.ID, tc.report)
			}

			got := m.statusOf(sess)
			if got.glyph != tc.wantGlyph || got.label != tc.wantLabel {
				t.Errorf("statusOf = %q %q, want %q %q", got.glyph, got.label, tc.wantGlyph, tc.wantLabel)
			}
		})
	}
}

// TestSidebarSurvivesALoadedBadgeLine keeps the card's third line inside the
// column.
//
// It is the one card line whose length the caller does not control: it grows
// with the claim and mail counts and with an exit error, where the title and
// branch are truncated against a width the caller passes in. An over-long line
// does not widen the sidebar, it wraps — the card gains a row, the remainder
// starts at column 0 with no bar, and the column ends up taller than the pane
// it is joined to. So the assertion is on the row count as much as the width;
// checking width alone misses it entirely, because every wrapped row is narrow.
func TestSidebarSurvivesALoadedBadgeLine(t *testing.T) {
	c, err := coord.Start(t.TempDir())
	if err != nil {
		t.Fatalf("coordinator: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	st := &store.State{}
	p := st.AddProject(store.Project{Name: "demo", Path: t.TempDir()})
	sess := st.AddSession(store.Session{
		ProjectID: p.ID, Name: "swift-otter-aaaa", Title: "a fairly long session title", Dir: t.TempDir(),
	})

	m := New(st, "bash", nil).WithCoordinator(c)
	m.width, m.height = 80, 30 // the narrowest sidebar the layout allows: 24
	m.rows = []sidebarRow{{project: p}, {session: sess}}
	m.runners[sess.ID] = startAgent(t, sess.Dir, "sleep 30")

	c.Register(coord.Session{ID: sess.ID, ProjectID: p.ID, Name: sess.Name, Dir: sess.Dir})
	c.Register(coord.Session{ID: "sib", ProjectID: p.ID, Name: "wily-crane-bbbb", Dir: sess.Dir})
	c.Report(sess.ID, coord.StateWaiting) // the longest label
	c.Claim(sess.ID, []string{"a.go", "b.go", "c.go", "d.go", "e.go",
		"f.go", "g.go", "h.go", "i.go", "j.go", "k.go", "l.go"}, "refactor")
	for i := 0; i < 12; i++ {
		if _, err := c.Send("sib", sess.Name, "hi"); err != nil {
			t.Fatalf("send: %v", err)
		}
	}
	if n, u := c.ClaimCount(sess.ID), c.Unread(sess.ID); n != 12 || u != 12 {
		t.Fatalf("fixture has %d claims and %d unread, want 12 and 12 — "+
			"the badges are what make the line long, so this proves nothing", n, u)
	}

	sidebarW, bodyH := m.layout()
	lines := strings.Split(m.renderSidebar(sidebarW, bodyH), "\n")
	if len(lines) != bodyH {
		t.Errorf("sidebar is %d rows, want %d — a wrapped card makes the column "+
			"taller than the pane beside it", len(lines), bodyH)
	}
	for i, line := range lines {
		if w := lipgloss.Width(line); w > sidebarW {
			t.Errorf("row %d is %d cells, sidebar is %d: %q", i, w, sidebarW, line)
		}
	}
}

// TestSidebarShowsWhatAnalysesCost is the visibility a spawned review needs.
// It has no pane and the session that asked for it has moved on, so the
// sidebar is the only place a running one is visible and the only place its
// cost is reported at all.
//
// It drives the real path — Analyse, through a reviewer the test supplies —
// rather than seeding records, so the badge is asserted against what the
// coordinator actually produces.
func TestSidebarShowsWhatAnalysesCost(t *testing.T) {
	release := make(chan struct{})
	c, err := coord.Start(t.TempDir(), coord.WithReviewer(
		func(ctx context.Context, _, _ string, onUsage func(agent.Tokens)) (agent.ClaudeRun, error) {
			// Reported before blocking, so the assertions below run against a
			// review that is genuinely mid-flight. A stub that never reports
			// leaves the live figure at zero and the badge's token path
			// unexercised, which is how it went untested the first time.
			onUsage(agent.Tokens{Input: 2, CacheWrite: 7954, Output: 1500})
			<-release
			return agent.ClaudeRun{Text: "fine", CostUSD: 0.37}, nil
		}))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })

	repo, head := gittest.RepoWith(t, "router.go", "package gateway\n")
	st := &store.State{}
	p := st.AddProject(store.Project{Name: "api-gateway", Path: t.TempDir()})
	asker := st.AddSession(store.Session{ProjectID: p.ID, Name: "swift-otter-aaaa",
		Title: "rate limiting", Dir: t.TempDir(), Agent: "bash"})
	worker := st.AddSession(store.Session{ProjectID: p.ID, Name: "wily-crane-bbbb",
		Title: "auth", Dir: repo, Agent: "bash", Isolated: true, BaseRef: head})

	m := New(st, "bash", nil).WithCoordinator(c)
	m.width, m.height = 100, 30
	m.screen = screenSession
	m.rebuildRows()

	// Nothing spawned: no badge, so an idle session is not decorated with a
	// figure that means nothing.
	if got := m.renderSidebar(34, 14); strings.Contains(got, "⚗") {
		t.Fatalf("a session with no analyses shows the badge:\n%s", got)
	}

	c.Register(coord.Session{ID: asker.ID, ProjectID: p.ID, Name: asker.Name, Dir: asker.Dir})
	c.Register(coord.Session{ID: worker.ID, ProjectID: p.ID, Name: worker.Name,
		Dir: worker.Dir, Isolated: true, BaseRef: head})

	if _, err := c.Analyse(asker.ID, worker.Name, ""); err != nil {
		t.Fatal(err)
	}
	// Tokens while it runs: the CLI reports a cost only when a turn ends, so a
	// dollar figure here would sit at zero and read as a review costing
	// nothing.
	waitForBadge(t, m, "⚗ 1 · 1.5k")

	close(release)
	waitForBadge(t, m, "$0.37")
}

// waitForBadge polls the rendered sidebar, because the analysis runs in its
// own goroutine and the UI reads whatever the coordinator has at frame time.
func waitForBadge(t *testing.T, m Model, want string) {
	t.Helper()
	for range 100 {
		if got := m.renderSidebar(34, 14); strings.Contains(got, want) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("sidebar never showed %q:\n%s", want, m.renderSidebar(34, 14))
}

// TestStatusGlyphsAreDistinct is the regression for the state the card cannot
// spell out any more.
//
// Idle and working were both ◉, separated by accent against muted. The sidebar
// carried the word beside the dot, so the colour was a hint rather than the
// whole answer. It is the whole answer now, and a colour is not one: a
// low-contrast theme flattens it, and a reader who cannot separate two hues
// never had it.
func TestStatusGlyphsAreDistinct(t *testing.T) {
	seen := map[string]string{}
	for _, g := range []struct{ name, glyph string }{
		{"closed", glyphClosed},
		{"idle", glyphIdle},
		{"working", glyphWorking},
		{"waiting", glyphWaiting},
		{"exited", glyphExited},
	} {
		if first, dup := seen[g.glyph]; dup {
			t.Errorf("%s and %s are both %q, so the card cannot tell them apart", first, g.name, g.glyph)
		}
		seen[g.glyph] = g.name
		// One cell each. The sidebar budgets the title line by subtracting a
		// fixed width for the glyph, so a two-cell one silently eats a column
		// of every session name.
		if w := ansi.StringWidth(g.glyph); w != 1 {
			t.Errorf("%s glyph %q is %d cells wide, want 1", g.name, g.glyph, w)
		}
	}
}

// TestSessionCardSpendsTwoLinesOnAQuietSession is the saving this shape exists
// for. A third of every card went on a line that said "Idle" beside a dot
// already saying it, and most sessions in a sidebar are quiet ones.
func TestSessionCardSpendsTwoLinesOnAQuietSession(t *testing.T) {
	st := &store.State{}
	p := st.AddProject(store.Project{Name: "demo", Path: "/d"})
	sess := st.AddSession(store.Session{
		ProjectID: p.ID, Name: "swift-otter-aaaa", Title: "a session", Branch: "session/x",
	})
	m := New(st, "bash", nil)

	card := m.sessionCard(sess, false, 30, 0)

	if len(card) != 2 {
		t.Fatalf("card is %d lines, want 2:\n%s", len(card), strings.Join(card, "\n"))
	}
	// The glyph has to be on the title line, or the state is not visible at all
	// now that the word is gone.
	if !strings.Contains(card[0], glyphClosed) {
		t.Errorf("the status glyph is not on the title line: %q", card[0])
	}
	if !strings.Contains(card[0], "a session") {
		t.Errorf("the title is not on the title line: %q", card[0])
	}
}

// TestSessionCardGrowsALineForWhatAGlyphCannotSay is the other half. Badges and
// an exit error are facts about a session rather than a restatement of its dot,
// so they still earn the row the state no longer does.
func TestSessionCardGrowsALineForWhatAGlyphCannotSay(t *testing.T) {
	c, err := coord.Start(t.TempDir())
	if err != nil {
		t.Fatalf("coordinator: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	st := &store.State{}
	p := st.AddProject(store.Project{Name: "demo", Path: t.TempDir()})
	sess := st.AddSession(store.Session{
		ProjectID: p.ID, Name: "swift-otter-aaaa", Title: "a session", Dir: t.TempDir(),
	})
	m := New(st, "bash", nil).WithCoordinator(c)

	if n := len(m.sessionCard(sess, false, 30, 0)); n != 2 {
		t.Fatalf("card is %d lines before any badge, so this proves nothing", n)
	}

	c.Register(coord.Session{ID: sess.ID, ProjectID: p.ID, Name: sess.Name, Dir: sess.Dir})
	c.Register(coord.Session{ID: "sib", ProjectID: p.ID, Name: "wily-crane-bbbb", Dir: sess.Dir})
	if _, err := c.Send("sib", sess.Name, "hi"); err != nil {
		t.Fatalf("send: %v", err)
	}

	card := m.sessionCard(sess, false, 30, 0)
	if len(card) != 3 {
		t.Fatalf("card is %d lines with unread mail, want 3:\n%s", len(card), strings.Join(card, "\n"))
	}
	if !strings.Contains(card[2], "✉") {
		t.Errorf("the third line does not carry the badge that earned it: %q", card[2])
	}
	// No leading gap: the badges are joined, not concatenated with the two
	// spaces that used to separate them from a label in front.
	if strings.HasPrefix(strings.TrimPrefix(card[2], "  "), " ") {
		t.Errorf("the badge line is indented past the bar: %q", card[2])
	}
}
