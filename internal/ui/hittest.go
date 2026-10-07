package ui

// What a cell of the dashboard or the session sidebar stands for. The
// builders record it on each line they build, and the renderer and the mouse
// both read those lines, so a click never resolves against a second copy of
// the layout arithmetic.

import "math"

type targetKind int

const (
	hitNone targetKind = iota
	hitProject
	hitSession
	hitTab
	hitRow
)

// target is what a click on a cell acts on. index is the project's position in
// the store, the session's in the project's list, the tab's, or for hitRow the
// session sidebar's row in Model.rows.
type target struct {
	kind  targetKind
	index int
}

// span is a run of cells on one line that stands for one target, counted from
// the line's first cell. to is exclusive.
type span struct {
	from, to int
	target   target
}

// drawnLine is one line of a column as it is drawn, with what each part of it
// stands for. A line with no spans stands for nothing.
type drawnLine struct {
	text  string
	spans []span
}

// wholeLine is a line that stands for one target across its full width,
// including the margin cells either side of its text.
func wholeLine(text string, t target) drawnLine {
	return drawnLine{text: text, spans: []span{{math.MinInt, math.MaxInt, t}}}
}

// at is the target under cell x of the line, counted from its first cell.
func (l drawnLine) at(x int) target {
	for _, s := range l.spans {
		if x >= s.from && x < s.to {
			return s.target
		}
	}
	return target{}
}

// texts is the drawn text of lines, one string per line.
func texts(lines []drawnLine) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = l.text
	}
	return out
}
