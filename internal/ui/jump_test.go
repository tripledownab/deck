package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/tripledownab/deck/internal/store"
)

// twoProjects builds a sidebar with sessions split across two projects, so the
// row list interleaves project headers with sessions. It returns the model and
// the session names in the order the sidebar shows them.
//
// That order is read back from the store rather than assumed to be the order
// they were added: SessionsFor sorts newest first, so an insertion-order
// expectation is wrong in a way that looks like a jump bug.
func twoProjects(t *testing.T) (Model, []string) {
	t.Helper()
	st := &store.State{}
	for _, p := range []struct {
		name     string
		sessions []string
	}{
		{"alpha", []string{"swift-otter-aaaa", "wily-crane-bbbb"}},
		{"beta", []string{"brisk-heron-cccc", "quiet-lynx-dddd"}},
	} {
		proj := st.AddProject(store.Project{Name: p.name, Path: t.TempDir()})
		for _, n := range p.sessions {
			st.AddSession(store.Session{ProjectID: proj.ID, Name: n, Dir: t.TempDir()})
		}
	}

	var names []string
	for _, p := range st.Projects {
		for _, s := range st.SessionsFor(p.ID) {
			names = append(names, s.Name)
		}
	}

	m := New(st, "bash", nil)
	m.width, m.height = 100, 40
	m.rebuildRows()
	return m, names
}

// TestJumpCountsSessionsNotRows is the whole point of the feature: m.rows
// interleaves project headers, so with two projects the third session sits at
// row index four. Counting rows would land ^g 3 on the wrong session, or on a
// header, which is not a session at all.
func TestJumpCountsSessionsNotRows(t *testing.T) {
	m, names := twoProjects(t)

	if len(m.rows) != 6 {
		t.Fatalf("rows = %d, want 6 (two headers, four sessions) — the fixture "+
			"does not interleave, so this test cannot catch off-by-header", len(m.rows))
	}

	for n, want := range names {
		if !m.jumpToSession(n + 1) {
			t.Fatalf("jump to %d failed", n+1)
		}
		got := m.currentSession()
		if got == nil || got.Name != want {
			t.Errorf("^g %d selected %v, want %s", n+1, got, want)
		}
	}
}

// TestJumpPastTheEndSaysSoRatherThanMoving keeps an out-of-range digit from
// looking like a broken key.
func TestJumpPastTheEndSaysSoRatherThanMoving(t *testing.T) {
	m, _ := twoProjects(t)
	if !m.jumpToSession(2) {
		t.Fatal("jump to 2 failed")
	}
	before := m.rowIx

	if m.jumpToSession(7) {
		t.Error("jump to 7 reported success with four sessions")
	}
	if m.rowIx != before {
		t.Errorf("cursor moved to row %d on a failed jump, want %d", m.rowIx, before)
	}
	if !strings.Contains(m.notice, "4") {
		t.Errorf("notice %q does not say how many sessions there are", m.notice)
	}
}

// TestJumpKeyIgnoresBatchedRunes is the KeyRunes trap, applied to digits.
//
// Bubble Tea batches the runes of one read into a single message, so a fast or
// pasted "12" arrives as one KeyMsg. Reading Runes[0] would jump to session 1
// on input that meant no such thing — the same class of bug that made typing
// "up" behave as the up arrow.
func TestJumpKeyIgnoresBatchedRunes(t *testing.T) {
	for _, tc := range []struct {
		name   string
		msg    tea.KeyMsg
		want   int
		wantOK bool
	}{
		{"a single digit", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}}, 3, true},
		{"two digits in one read", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1', '2'}}, 0, false},
		{"zero has no session", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'0'}}, 0, false},
		{"a letter", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}}, 0, false},
		{"a named key", tea.KeyMsg{Type: tea.KeyEnter}, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			n, ok := digitKey(tc.msg)
			if n != tc.want || ok != tc.wantOK {
				t.Errorf("digitKey = %d, %v; want %d, %v", n, ok, tc.want, tc.wantOK)
			}
		})
	}
}

// TestJumpThroughThePrefixShowsTheSession drives Update rather than calling
// jumpToSession, because a jump that never reaches the screen is no jump: the
// binding is only reachable through the armed prefix, and the digit has to
// survive the command dispatch.
func TestJumpThroughThePrefixShowsTheSession(t *testing.T) {
	m, names := twoProjects(t)
	m.screen = screenDashboard

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlG})
	next, _ = next.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'4'}})
	got := next.(Model)

	if got.screen != screenSession {
		t.Error("a successful jump left the dashboard up")
	}
	if s := got.currentSession(); s == nil || s.Name != names[3] {
		t.Errorf("^g 4 selected %v, want %s", s, names[3])
	}
}

// TestArmingThePrefixNumbersTheSidebar covers the other half: the digits are
// only useful if you can see which card each one means, and they are drawn
// only while the prefix is armed.
func TestArmingThePrefixNumbersTheSidebar(t *testing.T) {
	m, _ := twoProjects(t)
	sidebarW, bodyH := m.layout()

	// The fixture's names and branches carry no digits, so any digit in the
	// frame is a jump number.
	if got := m.renderSidebar(sidebarW, bodyH); strings.ContainsAny(got, "123456789") {
		t.Errorf("the sidebar is numbered before ^g is pressed:\n%s", got)
	}

	m.armed = true
	armed := m.renderSidebar(sidebarW, bodyH)
	for _, want := range []string{"1 ", "2 ", "3 ", "4 "} {
		if !strings.Contains(armed, want) {
			t.Errorf("armed sidebar has no %q:\n%s", want, armed)
		}
	}
	if lines := strings.Split(armed, "\n"); len(lines) != bodyH {
		t.Errorf("arming changed the sidebar to %d rows, want %d", len(lines), bodyH)
	}
}
