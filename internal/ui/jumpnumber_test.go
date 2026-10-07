package ui

// Jump numbers past nine: when ^g and a digit jumps at once, when it waits for
// another, and what ends the wait. Driven through Update, because the number
// spans several keys and only the dispatch sees all of them.

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/tripledownab/deck/internal/store"
)

// manySessions builds n sessions across two projects, so the rows interleave
// headers, and returns the session names in the order the sidebar numbers
// them.
func manySessions(t *testing.T, n int) (Model, []string) {
	t.Helper()
	st := &store.State{}
	a := st.AddProject(store.Project{Name: "alpha", Path: t.TempDir()})
	b := st.AddProject(store.Project{Name: "beta", Path: t.TempDir()})
	for i := range n {
		p := a
		if i%2 == 1 {
			p = b
		}
		st.AddSession(store.Session{ProjectID: p.ID, Name: fmt.Sprintf("s-%02d", i), Dir: t.TempDir()})
	}
	var names []string
	for _, p := range st.Projects {
		for _, s := range st.SessionsFor(p.ID) {
			names = append(names, s.Name)
		}
	}
	m := sized(New(st, "bash", nil), 120, 40)
	return m, names
}

// keys sends each message through Update in turn.
func keys(m Model, msgs ...tea.KeyMsg) Model {
	for _, k := range msgs {
		next, _ := m.Update(k)
		m = next.(Model)
	}
	return m
}

var prefix = tea.KeyMsg{Type: tea.KeyCtrlG}

func selected(m Model) string {
	if s := m.currentSession(); s != nil {
		return s.Name
	}
	return ""
}

// TestJumpCompletesWhenTheNumberCannotGrow is the rule: with 15 sessions a 7
// jumps at once, because there is no 70, and a 1 waits, because 10 to 15 begin
// with it. With nine or fewer every digit jumps at once, as it always did.
func TestJumpCompletesWhenTheNumberCannotGrow(t *testing.T) {
	m, names := manySessions(t, 15)

	if got := keys(m, prefix, typed("7")); selected(got) != names[6] || got.armed {
		t.Errorf("^g 7 selected %q, armed %v; want %s at once", selected(got), got.armed, names[6])
	}
	waiting := keys(m, prefix, typed("1"))
	if !waiting.armed || waiting.jumpDigits != 1 || selected(waiting) != selected(m) {
		t.Fatalf("^g 1 with 15 sessions did not wait: armed %v, pending %d", waiting.armed, waiting.jumpDigits)
	}
	if got := keys(waiting, typed("4")); selected(got) != names[13] || got.screen != screenSession {
		t.Errorf("^g 1 4 selected %q on screen %v, want %s in the session view", selected(got), got.screen, names[13])
	}

	few, fewNames := manySessions(t, 9)
	if got := keys(few, prefix, typed("1")); selected(got) != fewNames[0] || got.armed {
		t.Errorf("with nine sessions ^g 1 waited instead of jumping")
	}
	ten, _ := manySessions(t, 10)
	if got := keys(ten, prefix, typed("1")); got.jumpDigits != 1 {
		t.Errorf("with ten sessions ^g 1 did not wait for a possible 10")
	}
}

// TestWhatEndsAPendingNumber: ↵ jumps to it, esc drops it, and any other key
// drops it and runs as the command it is. Esc after ^g alone detaches the
// pane, so the model is attached to tell dropping the number from that.
func TestWhatEndsAPendingNumber(t *testing.T) {
	m, names := manySessions(t, 15)
	m.jumpToSession(5)
	m.attached = true
	start := selected(m)

	if got := keys(m, prefix, typed("1"), tea.KeyMsg{Type: tea.KeyEnter}); selected(got) != names[0] {
		t.Errorf("^g 1 ↵ selected %q, want %s", selected(got), names[0])
	}
	esc := keys(m, prefix, typed("1"), tea.KeyMsg{Type: tea.KeyEsc})
	if selected(esc) != start || esc.armed || esc.jumpDigits != 0 || !esc.attached {
		t.Errorf("^g 1 esc left %q selected, armed %v, pending %d, attached %v",
			selected(esc), esc.armed, esc.jumpDigits, esc.attached)
	}
	if got := keys(m, prefix, typed("1"), typed("j")); selected(got) != names[5] {
		t.Errorf("^g 1 j selected %q, want j to move down to %s", selected(got), names[5])
	}
}

// TestBatchedDigitsAreOneNumber: over SSH a fast "14" arrives as one message.
func TestBatchedDigitsAreOneNumber(t *testing.T) {
	m, names := manySessions(t, 15)
	if got := keys(m, prefix, typed("14")); selected(got) != names[13] {
		t.Errorf("^g \"14\" in one read selected %q, want %s", selected(got), names[13])
	}
}

// TestAFreshPrefixStartsAFreshNumber: a click disarms the prefix with a
// number pending, and the next ^g 3 must mean 3, not 13.
func TestAFreshPrefixStartsAFreshNumber(t *testing.T) {
	m, names := manySessions(t, 15)
	m = keys(m, prefix, typed("1"))
	m = press(m, tea.MouseButtonWheelDown, 60, 10)
	if got := keys(m, prefix, typed("3")); selected(got) != names[2] {
		t.Errorf("^g 1, a click, ^g 3 selected %q, want %s", selected(got), names[2])
	}
}

// TestAJumpPastTheEndSaysSo: a two-digit number past the last session moves
// nothing and names the count, and a 0 alone is no session at all.
func TestAJumpPastTheEndSaysSo(t *testing.T) {
	m, _ := manySessions(t, 15)
	start := selected(m)
	got := keys(m, prefix, typed("1"), typed("6"))
	if selected(got) != start || !strings.Contains(got.notice, "16") || !strings.Contains(got.notice, "15") {
		t.Errorf("^g 16 with 15 sessions: selected %q, notice %q", selected(got), got.notice)
	}
	if got := keys(m, prefix, typed("0")); selected(got) != start || got.armed {
		t.Errorf("^g 0 moved the cursor or stayed armed")
	}
}

// TestAPendingNumberIsShown: the footer says Deck is waiting and how to end
// it, and the armed sidebar numbers every card, padded so titles line up.
func TestAPendingNumberIsShown(t *testing.T) {
	m, names := manySessions(t, 15)
	m.screen = screenSession
	m = keys(m, prefix, typed("1"))
	if foot := m.sessionFooter(); !strings.Contains(foot, "^g 1…") || !strings.Contains(foot, "↵ jumps to 1") {
		t.Errorf("footer while ^g 1 is pending: %q", foot)
	}
	sidebarW, bodyH := m.layout()
	side := m.renderSidebar(sidebarW, bodyH)
	for _, want := range []string{" 1 ", " 9 ", "10 ", "15 "} {
		if !strings.Contains(side, want) {
			t.Errorf("armed sidebar has no %q", want)
		}
	}
	column := func(name string) int {
		for _, l := range strings.Split(ansi.Strip(side), "\n") {
			if i := strings.Index(l, name); i >= 0 {
				return ansi.StringWidth(l[:i])
			}
		}
		t.Fatalf("%s is not in the sidebar", name)
		return 0
	}
	if first, last := column(names[0]), column(names[14]); first != last {
		t.Errorf("titles of cards 1 and 15 start in columns %d and %d", first, last)
	}
}

// TestAPrefixWhilePendingStartsAgain: ^g 1 then ^g 3 is 3. Taken as a command,
// the second ^g was the literal prefix, which sent ^g and every key after it
// to the agent.
func TestAPrefixWhilePendingStartsAgain(t *testing.T) {
	m, names := manySessions(t, 15)
	got := keys(m, prefix, typed("1"), prefix)
	if !got.armed || got.jumpDigits != 0 {
		t.Fatalf("^g 1 ^g left armed %v, pending %d; want armed afresh", got.armed, got.jumpDigits)
	}
	if got = keys(got, typed("3")); selected(got) != names[2] {
		t.Errorf("^g 1 ^g 3 selected %q, want %s", selected(got), names[2])
	}
}

// TestALongPasteCannotWrapAround: digits past any session count stay a number
// past any session count. Unchecked, 2^64+10 wraps to 10 and jumps there.
func TestALongPasteCannotWrapAround(t *testing.T) {
	m, _ := manySessions(t, 15)
	start := selected(m)
	for _, paste := range []string{"18446744073709551626", strings.Repeat("9", 60)} {
		got := keys(m, prefix, typed(paste))
		if selected(got) != start || got.armed || got.jumpDigits != 0 || !strings.Contains(got.notice, "no session") {
			t.Errorf("pasting %s: selected %q, armed %v, pending %d, notice %q",
				paste, selected(got), got.armed, got.jumpDigits, got.notice)
		}
	}
}

// TestAPressDropsAPendingNumber: a press disarms the prefix, and the number
// pending behind it goes too.
func TestAPressDropsAPendingNumber(t *testing.T) {
	m, _ := manySessions(t, 15)
	got := press(keys(m, prefix, typed("1")), tea.MouseButtonWheelDown, 60, 10)
	if got.armed || got.jumpDigits != 0 {
		t.Errorf("after a press: armed %v, pending %d", got.armed, got.jumpDigits)
	}
}
