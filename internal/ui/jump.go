package ui

// Jumping to a session by its number after ^g: reading the digits, and
// deciding when the number is complete. Where the cursor lands is
// jumpToSession, in selection.go.

import (
	"math"

	tea "github.com/charmbracelet/bubbletea"
)

// maxJump caps a pasted run of digits so the number cannot overflow, even
// where int is 32 bits. It is far past any session count, so a capped number
// still reports "no session", naming the cap rather than what was pasted.
const maxJump = math.MaxInt32

// digitKey reads the digits of one key message, most significant first.
//
// Bubble Tea batches the runes of one read into a single KeyRunes message, so
// a fast "12" can arrive as one message. Every rune has to be a digit: a batch
// with a letter in it is text, not a number.
func digitKey(msg tea.KeyMsg) ([]int, bool) {
	if msg.Type != tea.KeyRunes || len(msg.Runes) == 0 {
		return nil, false
	}
	ds := make([]int, len(msg.Runes))
	for i, r := range msg.Runes {
		if r < '0' || r > '9' {
			return nil, false
		}
		ds[i] = int(r - '0')
	}
	return ds, true
}

// typeDigits extends the pending number with ds and jumps as soon as no
// session number could start with it. With 15 sessions, 2 jumps at once
// because there is no 20, and 1 waits, because 10 to 15 begin with it. So
// nothing is timed: a number is complete when it cannot grow, or on ↵.
//
// A number that could grow keeps the prefix armed, so the next key comes back
// here through continueJump. There is no session 0, so a 0 with nothing before
// it is ignored, as it always was.
func (m Model) typeDigits(ds []int) (tea.Model, tea.Cmd) {
	n := m.jumpDigits
	for _, d := range ds {
		if n > (maxJump-d)/10 {
			n = maxJump
			continue
		}
		n = n*10 + d
	}
	m.jumpDigits = 0
	if n == 0 {
		return m, nil
	}
	// n*10 <= count, written so n*10 cannot overflow.
	if n <= m.sessionCount()/10 {
		m.jumpDigits, m.armed = n, true
		return m, nil
	}
	return m.jumpTo(n)
}

// continueJump handles the key after a number that could still grow. ↵ ends
// the number and esc drops it. The prefix drops it and arms afresh, so ^g 1
// then ^g 3 is 3: taken as a command it would be the literal prefix, and send
// ^g and every key after it to the agent. Any other key drops the number and
// is taken as the command it is, because the prefix is still armed.
func (m Model) continueJump(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if ds, ok := digitKey(msg); ok {
		return m.typeDigits(ds)
	}
	n := m.jumpDigits
	m.jumpDigits = 0
	switch {
	case msg.Type == tea.KeyEnter:
		return m.jumpTo(n)
	case msg.Type == tea.KeyEsc:
		return m, nil
	case msg.String() == PrefixKey:
		m.armed = true
		return m, nil
	}
	return m.command(msg)
}

// jumpTo moves to session n. Only on a successful jump does the screen
// follow: a number past the end leaves you where you are, with the notice
// jumpToSession set.
func (m Model) jumpTo(n int) (tea.Model, tea.Cmd) {
	if m.jumpToSession(n) {
		m.screen = screenSession
	}
	return m, nil
}

// sessionCount is how many sessions the sidebar lists, which is what a jump
// number counts.
func (m Model) sessionCount() int {
	n := 0
	for _, row := range m.rows {
		if row.session != nil {
			n++
		}
	}
	return n
}
