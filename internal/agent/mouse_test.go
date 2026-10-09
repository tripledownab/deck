package agent

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// TestWheelUsesTheFormTheAgentAskedFor: an agent that set a mouse-tracking
// mode gets the wheel as a mouse report at the cell given, SGR if it also set
// ?1006 and X10 if not. One that never set it, cleared it, or was reset gets
// the ↑ or ↓ key. cathode with mouse capture on reads ↑ as prompt history, so
// sending it the key would recall an old prompt instead of scrolling.
//
// The script prints the bytes it read as decimals, so the test compares bytes
// rather than what a screen makes of them, and then says whether anything
// followed: a report and a key together would still recall cathode's history.
// It reads with dd, one byte at a time, because BSD head reads ahead and would
// swallow whatever followed in the same write.
func TestWheelUsesTheFormTheAgentAskedFor(t *testing.T) {
	const sgr = `\033[?1000h\033[?1006h`
	for name, tc := range map[string]struct {
		modes string // what the agent prints before it reads
		up    bool
		x, y  int
		want  string
	}{
		"sgr, up":             {sgr, true, 3, 4, "\x1b[<64;4;5M"},
		"x10 compatibility":   {`\033[?9h\033[?1006h`, true, 3, 4, "\x1b[<64;4;5M"},
		"highlight tracking":  {`\033[?1001h\033[?1006h`, true, 3, 4, "\x1b[<64;4;5M"},
		"button-event":        {`\033[?1002h\033[?1006h`, true, 3, 4, "\x1b[<64;4;5M"},
		"any-event":           {`\033[?1003h\033[?1006h`, true, 3, 4, "\x1b[<64;4;5M"},
		"sgr, down":           {sgr, false, 3, 4, "\x1b[<65;4;5M"},
		"x10":                 {`\033[?1000h`, true, 3, 4, "\x1b[M`$%"},
		"x10 past its range":  {`\033[?1000h`, true, 300, 4, "\x1b[M`\xff%"},
		"x10 below its range": {`\033[?1000h`, true, 3, 300, "\x1b[M`$\xff"},
		"tracking cleared":    {sgr + `\033[?1000l`, true, 3, 4, "\x1b[A"},
		"reset (RIS)":         {sgr + `\033c`, false, 3, 4, "\x1b[B"},
		"never on":            {``, true, 3, 4, "\x1b[A"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			script := fmt.Sprintf(`stty -echo -icanon; printf '%s'; echo READY; `+
				`dd bs=1 count=%d 2>/dev/null | od -An -tu1 | tr -s ' \n' '  '; echo END; `+
				`if IFS= read -r -t 1 -n 1 rest; then echo EXTRA; else echo CLEAN; fi; sleep 5`,
				tc.modes, len(tc.want))
			r, err := Start(Config{
				Command: "bash",
				Args:    []string{"--norc", "--noprofile", "-c", script},
				Dir:     t.TempDir(),
				Width:   80,
				Height:  24,
			})
			if err != nil {
				t.Fatalf("start: %v", err)
			}
			defer r.Stop()

			if !screenContains(t, r, "READY", 5*time.Second) {
				t.Fatal("the script never got ready")
			}
			if err := r.Wheel(tc.up, tc.x, tc.y); err != nil {
				t.Fatalf("wheel: %v", err)
			}
			var want strings.Builder
			for _, b := range []byte(tc.want) {
				fmt.Fprintf(&want, " %d", b)
			}
			want.WriteString(" END")
			if !screenContains(t, r, want.String(), 5*time.Second) {
				t.Errorf("want%s on screen:\n%s", want.String(), strings.Join(r.Render(false, nil, nil), "\n"))
			}
			if !screenContains(t, r, "CLEAN", 5*time.Second) {
				t.Errorf("the agent got more than the one form:\n%s", strings.Join(r.Render(false, nil, nil), "\n"))
			}
		})
	}
}
