// Package termquery answers the questions a terminal program asks its
// terminal, for harnesses that stand in for one.
//
// Deck's own tests and the probe tool both drive a real program through a
// pseudo-terminal with no terminal behind it. Programs block waiting for these
// replies — termenv waits five seconds for an OSC 11 answer before falling
// back to $COLORFGBG — so a harness that stays silent looks like a hang or a
// flaky test rather than a missing reply.
//
// This lives in one place because it was briefly implemented twice, and the
// two copies had already drifted by one query within a single sitting: the
// probe answered Device Attributes and the smoke test did not, so the two
// harnesses tolerated different programs.
package termquery

import "bytes"

// Replies to the queries in chunk, or nil if it contains none.
//
// The colours are Deck's own foreground and background. Nothing depends on
// the exact values — a program only needs *an* answer — but returning the real
// palette keeps a captured frame looking like the app.
func Answer(chunk []byte) []byte {
	var out []byte
	for _, q := range queries {
		// One reply per query even when a chunk carries two spellings of it.
		for _, ask := range q.asks {
			if bytes.Contains(chunk, ask) {
				out = append(out, q.reply...)
				break
			}
		}
	}
	return out
}

// A query can be spelled more than one way — DA is both the bare form and the
// explicit zero parameter — so each entry carries every spelling that means
// the same question.
var queries = []struct {
	asks  [][]byte
	reply []byte
}{
	// OSC 10/11: foreground and background colour.
	{[][]byte{[]byte("\x1b]10;?")}, []byte("\x1b]10;rgb:e4e4/e2e2/dddd\x1b\\")},
	{[][]byte{[]byte("\x1b]11;?")}, []byte("\x1b]11;rgb:1414/1313/0f0f\x1b\\")},
	// DSR: cursor position. Reported as home; nothing in these harnesses
	// depends on where the cursor actually is.
	{[][]byte{[]byte("\x1b[6n")}, []byte("\x1b[1;1R")},
	// DA: device attributes. Answered as a VT220-class terminal.
	{[][]byte{[]byte("\x1b[c"), []byte("\x1b[0c")}, []byte("\x1b[?62;c")},
}
