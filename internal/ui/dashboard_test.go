package ui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tripledownab/deck/internal/store"
)

// TestDashboardFitsTheTerminal pins the frame to exactly the terminal's height.
//
// A line wider than its column used to wrap, and a frame one line too tall
// loses its top row: Bubble Tea keeps the bottom of an over-tall frame. The
// header went, and every row under it was drawn a line higher than the code
// placed it, so a click selected the project above the one under the pointer.
// A running session is the ordinary trigger, because "Active · 1 running" makes
// the metadata row wider than "Idle" does. A long name, a newline or a tab in a
// name or a description, a multi-line error in the footer, and the placeholder
// an empty store shows in a narrow or short column are the others.
func TestDashboardFitsTheTerminal(t *testing.T) {
	st := &store.State{}
	p := st.AddProject(store.Project{
		Name:        "a-proj\nect-name-that-is-quite-long-indeed-for-a-sidebar-row",
		Path:        "/code/some/fairly/long/path/to/the/repository",
		Description: "line one\nline two",
	})
	st.AddProject(store.Project{Name: "a\tb\tc\td\te\tf\tg", Path: "/code/tabs"})
	sess := st.AddSession(store.Session{ProjectID: p.ID, Name: "n", Title: "t", Dir: t.TempDir(), Agent: "bash"})
	running := New(st, "bash", nil)
	running.runners[sess.ID] = startAgent(t, sess.Dir, "")
	tabbed := New(st, "bash", nil)
	tabbed.projectIx = 1
	failed := New(st, "bash", nil)
	failed.fault = errors.New("cannot remove a locked working tree\nuse -f -f to override")

	// Four rows is the least that holds the header, a column border, one line
	// of content and the footer.
	for name, m := range map[string]Model{
		"a running session": running,
		"a tab in a name":   tabbed,
		"a two-line fault":  failed,
		"an empty store":    New(&store.State{}, "bash", nil),
	} {
		for _, h := range []int{4, 5, 24} {
			for w := 1; w <= 200; w++ {
				m.width, m.height = w, h
				if got := len(strings.Split(m.View(), "\n")); got != h {
					t.Errorf("%s at %dx%d: the frame is %d lines", name, w, h, got)
				}
			}
		}
	}
}

// TestFirstFrameCentresTheProjectList: before any frame there is no window to
// keep still, so the list centres on the cursor, as it always has at launch.
// Starting from line 0 instead would show a cursor deep in the list pressed
// against the bottom edge. The size reaches the model through Update, as it
// does at launch, and a frame tick may get there first.
func TestFirstFrameCentresTheProjectList(t *testing.T) {
	for name, before := range map[string][]tea.Msg{
		"size first":       nil,
		"a tick before it": {frameMsg{}},
	} {
		var m tea.Model = New(mouseState(40, 0), "bash", nil).FocusProject("/code/p20")
		for _, msg := range append(before, tea.WindowSizeMsg{Width: 100, Height: 20}) {
			m, _ = m.Update(msg)
		}
		got := m.(Model)
		_, y := cellOf(t, got, "project-20", 0, got.dashboardLayout().navW)
		if mid := got.height / 2; y < mid-2 || y > mid+2 {
			t.Errorf("%s: project-20 is on row %d of %d, not near the middle", name, y, got.height)
		}
	}
}

// TestPlaceholderStaysCentred: the placeholder is cut to its space, and only
// cut. A version that padded the text to the full height left nothing to
// centre, so the empty dashboard and a closed pane drew their message on the
// top row, and every frame was still the right height.
func TestPlaceholderStaysCentred(t *testing.T) {
	m := New(&store.State{}, "bash", nil)
	for _, h := range []int{9, 20} {
		lines := strings.Split(m.placeholder(60, h, "the title", "the hint"), "\n")
		if len(lines) != h {
			t.Fatalf("height %d: the placeholder is %d lines", h, len(lines))
		}
		for y, l := range lines {
			if strings.Contains(l, "the title") {
				if mid := h / 2; y < mid-2 || y > mid {
					t.Errorf("height %d: the title is on row %d, not near the middle", h, y)
				}
			}
		}
	}
}
