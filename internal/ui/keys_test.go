package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestKeyToBytes(t *testing.T) {
	cases := []struct {
		name string
		msg  tea.KeyMsg
		want string
	}{
		{"letter", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")}, "a"},
		{"word batch", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hello")}, "hello"},
		{"alt letter", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b"), Alt: true}, "\x1bb"},
		{"enter", tea.KeyMsg{Type: tea.KeyEnter}, "\r"},
		{"tab", tea.KeyMsg{Type: tea.KeyTab}, "\t"},
		{"escape", tea.KeyMsg{Type: tea.KeyEsc}, "\x1b"},
		{"backspace", tea.KeyMsg{Type: tea.KeyBackspace}, "\x7f"},
		{"ctrl+c", tea.KeyMsg{Type: tea.KeyCtrlC}, "\x03"},
		{"ctrl+g", tea.KeyMsg{Type: tea.KeyCtrlG}, "\x07"},
		{"up", tea.KeyMsg{Type: tea.KeyUp}, "\x1b[A"},
		{"down", tea.KeyMsg{Type: tea.KeyDown}, "\x1b[B"},
		{"shift+tab", tea.KeyMsg{Type: tea.KeyShiftTab}, "\x1b[Z"},
		{"page up", tea.KeyMsg{Type: tea.KeyPgUp}, "\x1b[5~"},
		{"delete", tea.KeyMsg{Type: tea.KeyDelete}, "\x1b[3~"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := string(keyToBytes(c.msg)); got != c.want {
				t.Errorf("keyToBytes(%s) = %q, want %q", c.name, got, c.want)
			}
		})
	}
}

// TestSpaceReachesTheAgent is the regression for silently dropped spaces.
//
// Bubble Tea puts KeySpace in its negative "Other keys" block, not at ASCII
// 32, so the single-byte fallback in keyToBytes never saw it and every space
// typed into an agent vanished. It cost nothing to miss in review: forms use
// bubbles/textinput, which handles KeySpace itself, so only the PTY path broke.
func TestSpaceReachesTheAgent(t *testing.T) {
	if got := string(keyToBytes(tea.KeyMsg{Type: tea.KeySpace})); got != " " {
		t.Fatalf("space encoded as %q, want a space", got)
	}
}

// TestTypedSentenceRoundTrips reassembles what the agent receives when a user
// types a sentence, the way Bubble Tea delivers it: printable runs as
// KeyRunes, each space as its own KeySpace.
func TestTypedSentenceRoundTrips(t *testing.T) {
	const sentence = "echo hello world"

	var got []byte
	for _, word := range []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune("echo")},
		{Type: tea.KeySpace},
		{Type: tea.KeyRunes, Runes: []rune("hello")},
		{Type: tea.KeySpace},
		{Type: tea.KeyRunes, Runes: []rune("world")},
	} {
		got = append(got, keyToBytes(word)...)
	}
	if string(got) != sentence {
		t.Errorf("agent received %q, want %q", got, sentence)
	}
}

// TestEveryTypableKeyEncodes guards the class rather than the instance. A key
// a user can press while attached must never encode to nothing.
func TestEveryTypableKeyEncodes(t *testing.T) {
	typable := map[string]tea.KeyType{
		"space": tea.KeySpace, "enter": tea.KeyEnter, "tab": tea.KeyTab,
		"backspace": tea.KeyBackspace, "delete": tea.KeyDelete, "escape": tea.KeyEsc,
		"up": tea.KeyUp, "down": tea.KeyDown, "left": tea.KeyLeft, "right": tea.KeyRight,
		"home": tea.KeyHome, "end": tea.KeyEnd, "pgup": tea.KeyPgUp, "pgdown": tea.KeyPgDown,
		"shift+tab": tea.KeyShiftTab, "shift+left": tea.KeyShiftLeft,
		"shift+right": tea.KeyShiftRight, "ctrl+left": tea.KeyCtrlLeft,
		"ctrl+right": tea.KeyCtrlRight, "ctrl+c": tea.KeyCtrlC, "ctrl+d": tea.KeyCtrlD,
	}
	for name, kt := range typable {
		if got := keyToBytes(tea.KeyMsg{Type: kt}); len(got) == 0 {
			t.Errorf("%s encodes to nothing, so the agent never sees it", name)
		}
	}
}

// TestKeyToBytesPaste keeps the bracketed-paste markers, because the agent
// asked for them and uses them to tell a paste from fast typing.
func TestKeyToBytesPaste(t *testing.T) {
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("pasted"), Paste: true}
	want := "\x1b[200~pasted\x1b[201~"
	if got := string(keyToBytes(msg)); got != want {
		t.Errorf("paste = %q, want %q", got, want)
	}
}

// TestKeyToBytesDropsUnknown pins the deliberate choice to send nothing rather
// than guess: a wrong escape sequence reaching an agent mid-turn is worse than
// a dropped key.
func TestKeyToBytesDropsUnknown(t *testing.T) {
	if got := keyToBytes(tea.KeyMsg{Type: tea.KeyCtrlPgUp}); got != nil {
		t.Errorf("unmapped key produced %q, want nothing", got)
	}
}

// TestPrefixIsReachable guards the one key Deck takes from the agent. If
// it stops round-tripping, ^g ^g cannot send a literal ctrl+g either.
func TestPrefixIsReachable(t *testing.T) {
	msg := tea.KeyMsg{Type: tea.KeyCtrlG}
	if msg.String() != PrefixKey {
		t.Fatalf("ctrl+g stringifies as %q, but PrefixKey is %q", msg.String(), PrefixKey)
	}
}
