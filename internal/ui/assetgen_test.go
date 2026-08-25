package ui

// Regeneration of the README images in assets/. Every scene renders through
// Model.View, so an image cannot quietly drift from what the app draws. The
// ANSI-to-SVG engine is in ansisvg_test.go.
//
// Opt-in, so a plain `go test` never rewrites a checked-in file:
//
//	DECK_GENASSETS=1 go test -run TestGenerateAssets ./internal/ui/
//
// Every project, path and session name below is invented. A capture taken from
// a real store would publish the name of every project on the machine that ran
// it, which is the leak the fixture rule for test data exists to prevent.

import (
	"os"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/tripledownab/deck/internal/store"
)

// assetTheme is the palette the images ship in. Cinder is the default a first
// run gets, so the README shows what a reader will actually see.
const assetTheme = "cinder"

// assetW and assetH are the frame every scene renders at, so the images are
// one size and sit together on a page.
//
// The width is set by the project meta row: below about 110 columns "Path" and
// its value wrap onto a second line. The height is set by the new-session
// form, which is the tallest scene — a frame shorter than the modal does not
// clip it, it overflows, and that image alone comes out taller than the rest.
const assetW, assetH = 132, 34

// paneStep is the gap between commands written into the pane, and paneSettle
// is the wait before the capture. The pane is a live process, so a capture
// taken too early shows a half-drawn frame.
//
// paneStep has to outlast the slowest command, not just the echo: a line
// written while the previous one is still running is echoed by the line
// editor above its own output, and the pane reads as two interleaved turns.
const (
	paneStep   = 3 * time.Second
	paneSettle = 2 * time.Second
)

// demoState is the invented store every scene renders. Three projects, because
// two do not show that the list scrolls and four crowd the column.
func demoState() *store.State {
	st := &store.State{}
	// Ages are set back deliberately. A store built a moment ago reports "0s"
	// against every row, which makes each image look like a first run.
	now := time.Now()
	gw := st.AddProject(store.Project{
		Name: "api-gateway", Path: "/Users/you/code/api-gateway",
		Description: "request routing and auth",
		CreatedAt:   now.Add(-11 * 24 * time.Hour),
	})
	st.AddProject(store.Project{
		Name: "billing-service", Path: "/Users/you/code/billing-service",
		Description: "invoices and dunning",
		CreatedAt:   now.Add(-4 * 24 * time.Hour),
	})
	st.AddProject(store.Project{
		Name: "web-frontend", Path: "/Users/you/code/web-frontend",
		CreatedAt: now.Add(-2 * time.Hour),
	})

	// The newest session is the one the scenes attach to, and its agent is the
	// program the pane really runs. Naming it anything else would caption a
	// bash prompt as claude.
	for _, s := range []struct {
		name, title, agent string
		age                time.Duration
	}{
		{"scheming-hawk-jhgk", "rate limit the public endpoints", "claude", 3 * time.Hour},
		{"wily-crane-bbbb", "split the auth middleware", "cathode", 95 * time.Minute},
		{"brisk-heron-cccc", "retry budget for upstream calls", "bash", 12 * time.Minute},
	} {
		st.AddSession(store.Session{
			ProjectID: gw.ID, Name: s.name, Title: s.title, Agent: s.agent,
			Isolated: true, Branch: "session/" + s.name,
			Dir:       "/Users/you/.local/state/deck/worktrees/" + s.name,
			CreatedAt: now.Add(-s.age),
		})
	}
	return st
}

// demoModel is the state above, sized and themed, with the cursor on the
// project that owns the sessions.
func demoModel() Model {
	m := New(demoState(), "claude", nil).WithTheme(assetTheme)
	m.width, m.height = assetW, assetH
	m.rebuildRows()
	return m
}

// TestGenerateAssets writes one SVG per scene. It is a test only because that
// is how a package's own unexported state is reachable; nothing here asserts.
func TestGenerateAssets(t *testing.T) {
	if os.Getenv("DECK_GENASSETS") == "" {
		t.Skip("set DECK_GENASSETS=1 to regenerate assets/*.svg")
	}
	// Force 24-bit colour: under a narrower profile lipgloss quantises every
	// style to an ANSI index and the images lose the palette they exist to show.
	lipgloss.SetColorProfile(termenv.TrueColor)

	if err := os.MkdirAll("../../assets", 0o755); err != nil {
		t.Fatalf("create assets dir: %v", err)
	}
	p := paletteFor(assetTheme)
	canvas, text := sources[assetTheme].bg, string(p.Fg)

	for _, a := range []struct {
		name  string
		scene func(*testing.T) string
	}{
		{"deck-dashboard", sceneDashboard},
		{"deck-new-session", sceneNewSession},
		{"deck-session", sceneSession},
	} {
		path := "../../assets/" + a.name + ".svg"
		if err := os.WriteFile(path, []byte(ansiToSVG(a.scene(t), canvas, text)), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
		t.Logf("wrote assets/%s.svg", a.name)
	}
}

// sceneDashboard is the Overview: the project column, and the sessions of the
// selected project beside it. One session is live, because a dashboard whose
// every row reads Closed shows none of what the status column is for.
func sceneDashboard(t *testing.T) string {
	m := demoModel()
	m.focus = colContent
	return openDemoPane(t, m).View()
}

// sceneNewSession is the new-session form. It is here because the Agent field
// is the one part of the form a reader cannot guess from the key table.
func sceneNewSession(*testing.T) string {
	m := demoModel()
	projects := []choice{
		{label: "api-gateway", value: "api-gateway"},
		{label: "billing-service", value: "billing-service"},
		{label: "web-frontend", value: "web-frontend"},
	}
	m.form = newSessionForm(projects, 0, true, "claude")
	m.form.fields[sessionFieldTitle].input.SetValue("rate limit the public endpoints")
	// Focus the Agent field: it is the part of the form the key table cannot
	// describe, and an unfocused choice row does not show its help line.
	m.form.index = sessionFieldAgent
	return m.View()
}

// sceneSession is the session view with a live pane. The agent is bash running
// a real command: Deck hosts any terminal program, and a scripted transcript
// would be a drawing of an agent rather than a capture of one.
func sceneSession(t *testing.T) string {
	m := demoModel()
	m.screen = screenSession
	return openDemoPane(t, m).View()
}

// openDemoPane starts the newest session's agent and drives it through a real
// command, then returns the model with that runner attached. Both scenes use
// it: the dashboard reads its status column off a live runner exactly as the
// session view reads its pane.
func openDemoPane(t *testing.T, m Model) Model {
	m.rebuildRows()
	m.landOn(sessionRow(m))
	m.attached = true

	sess := m.currentSession()
	if sess == nil {
		t.Fatal("the demo state has no session to attach")
	}
	// startAgent opens at its own default size; resizePane is what the app
	// itself calls to fit a runner to the frame, so the pane in the image is
	// sized the way a real one is.
	r := startAgent(t, "../..", "")
	m.runners[sess.ID] = r
	m.resizePane()

	// PS1 first: bash's default prompt states its version, which dates the
	// image for no benefit. clear drops the prompt drawn before it.
	for _, line := range []string{
		"PS1='$ '",
		"clear",
		"go build ./... && echo build ok",
		"go test ./internal/naming/ ./internal/store/",
	} {
		if err := r.Write([]byte(line + "\r")); err != nil {
			t.Fatalf("write to the pane: %v", err)
		}
		time.Sleep(paneStep)
	}
	time.Sleep(paneSettle)
	return m
}

// sessionRow is the index of the first row that is a session rather than a
// project heading. Model.rows interleaves the two, so row 0 is a heading.
func sessionRow(m Model) int {
	for i, r := range m.rows {
		if r.session != nil {
			return i
		}
	}
	return 0
}
