package termquery

import (
	"strings"
	"testing"
)

func TestAnswersEachQuery(t *testing.T) {
	cases := map[string]struct{ ask, want string }{
		"foreground":       {"\x1b]10;?\x1b\\", "\x1b]10;rgb:"},
		"background":       {"\x1b]11;?\x1b\\", "\x1b]11;rgb:"},
		"cursor position":  {"\x1b[6n", "\x1b[1;1R"},
		"device attrs":     {"\x1b[c", "\x1b[?62;c"},
		"device attrs (0)": {"\x1b[0c", "\x1b[?62;c"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			got := string(Answer([]byte(c.ask)))
			if !strings.Contains(got, c.want) {
				t.Errorf("Answer(%q) = %q, want it to contain %q", c.ask, got, c.want)
			}
		})
	}
}

// TestDeviceAttrsAnsweredOnce guards the shape of the table. DA has two
// spellings, and a naive one-entry-per-spelling table would answer twice when
// a chunk happened to carry both — two replies to one question is worse than
// none, because the extra one arrives as input to whatever asked next.
func TestDeviceAttrsAnsweredOnce(t *testing.T) {
	got := string(Answer([]byte("\x1b[c and also \x1b[0c")))
	if n := strings.Count(got, "\x1b[?62;c"); n != 1 {
		t.Errorf("answered device attributes %d times, want 1 (got %q)", n, got)
	}
}

// TestNoQueryNoReply matters because the harnesses write whatever this returns
// straight back into the PTY: a spurious reply becomes keystrokes the program
// never asked for.
func TestNoQueryNoReply(t *testing.T) {
	for _, chunk := range []string{"", "plain output", "\x1b[2J\x1b[H", "\x1b[31mred\x1b[0m"} {
		if got := Answer([]byte(chunk)); len(got) != 0 {
			t.Errorf("Answer(%q) = %q, want nothing", chunk, got)
		}
	}
}

// TestAnswersSeveralInOneChunk covers the real case: a program emits its whole
// startup handshake in a single write.
func TestAnswersSeveralInOneChunk(t *testing.T) {
	got := string(Answer([]byte("\x1b]11;?\x1b\\\x1b[6n")))
	for _, want := range []string{"\x1b]11;rgb:", "\x1b[1;1R"} {
		if !strings.Contains(got, want) {
			t.Errorf("reply %q is missing %q", got, want)
		}
	}
}
