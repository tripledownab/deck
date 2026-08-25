package ui

// Encoding a Bubble Tea key back into the bytes a terminal would have sent,
// for the pane hosting another terminal program.

import (
	tea "github.com/charmbracelet/bubbletea"
)

// ptySequences maps the keys Bubble Tea names to the bytes a terminal would
// have sent. Bubble Tea parses stdin into KeyMsg values, so a pane hosting
// another terminal program has to encode them back.
//
// Keys absent from this table and outside the ASCII range below are dropped
// rather than guessed: sending a wrong escape sequence to an agent mid-turn is
// worse than sending nothing.
var ptySequences = map[tea.KeyType]string{
	// Space is first because it is the one that hurts. Bubble Tea puts
	// KeySpace in its *negative* "Other keys" block alongside the arrows, not
	// at ASCII 32 where the name suggests — so the single-byte fallback below
	// never sees it. Without this entry every space a user types is dropped
	// and "echo hello world" reaches the agent as "echohelloworld".
	tea.KeySpace: " ",

	tea.KeyUp:         "\x1b[A",
	tea.KeyDown:       "\x1b[B",
	tea.KeyRight:      "\x1b[C",
	tea.KeyLeft:       "\x1b[D",
	tea.KeyShiftUp:    "\x1b[1;2A",
	tea.KeyShiftDown:  "\x1b[1;2B",
	tea.KeyShiftRight: "\x1b[1;2C",
	tea.KeyShiftLeft:  "\x1b[1;2D",
	tea.KeyCtrlUp:     "\x1b[1;5A",
	tea.KeyCtrlDown:   "\x1b[1;5B",
	tea.KeyCtrlRight:  "\x1b[1;5C",
	tea.KeyCtrlLeft:   "\x1b[1;5D",
	tea.KeyHome:       "\x1b[H",
	tea.KeyEnd:        "\x1b[F",
	tea.KeyShiftHome:  "\x1b[1;2H",
	tea.KeyShiftEnd:   "\x1b[1;2F",
	tea.KeyCtrlHome:   "\x1b[1;5H",
	tea.KeyCtrlEnd:    "\x1b[1;5F",
	tea.KeyPgUp:       "\x1b[5~",
	tea.KeyPgDown:     "\x1b[6~",
	tea.KeyDelete:     "\x1b[3~",
	tea.KeyInsert:     "\x1b[2~",
	tea.KeyShiftTab:   "\x1b[Z",
	tea.KeyF1:         "\x1bOP",
	tea.KeyF2:         "\x1bOQ",
	tea.KeyF3:         "\x1bOR",
	tea.KeyF4:         "\x1bOS",
	tea.KeyF5:         "\x1b[15~",
	tea.KeyF6:         "\x1b[17~",
	tea.KeyF7:         "\x1b[18~",
	tea.KeyF8:         "\x1b[19~",
	tea.KeyF9:         "\x1b[20~",
	tea.KeyF10:        "\x1b[21~",
	tea.KeyF11:        "\x1b[23~",
	tea.KeyF12:        "\x1b[24~",
}

// keyToBytes encodes one KeyMsg for the PTY.
func keyToBytes(k tea.KeyMsg) []byte {
	if k.Paste {
		// Bracketed paste: the agent asked for the markers, so keep them.
		return []byte("\x1b[200~" + string(k.Runes) + "\x1b[201~")
	}

	if k.Type == tea.KeyRunes {
		b := []byte(string(k.Runes))
		if k.Alt {
			return append([]byte{0x1b}, b...)
		}
		return b
	}

	if seq, ok := ptySequences[k.Type]; ok {
		if k.Alt {
			return append([]byte{0x1b}, seq...)
		}
		return []byte(seq)
	}

	// Bubble Tea numbers the control keys by their byte value: KeyCtrlA is 1,
	// KeyTab is 9, KeyEnter is 13, KeyEsc is 27, KeyBackspace is 127.
	//
	// KeySpace is NOT 32 despite the name — it lives in the negative "Other
	// keys" block with the arrows, which is why it needs an entry in the table
	// above. Do not assume a key is in this range because it maps to one byte;
	// check the constant block.
	if k.Type >= 0 && k.Type <= 127 {
		if k.Alt {
			return []byte{0x1b, byte(k.Type)}
		}
		return []byte{byte(k.Type)}
	}

	return nil
}
