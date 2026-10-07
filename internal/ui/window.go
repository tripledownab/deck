package ui

// Scroll windowing: which lines of a list a fixed-height column or modal
// shows, and the marker on an edge that has more past it.

// moreGlyph marks a column edge with content past it.
//
// The same glyph truncate appends, and deliberately so: it already means "cut
// off, there is more" everywhere else in the frame, and a second symbol for the
// same fact is a second thing to learn.
//
// One character, so it fits the narrowest column Deck will draw, and it carries
// no count: the project list's lines are projects, while the sidebar's are
// thirds of a session card, so any number would be right in one column and
// wrong in the other. Which edge it sits on is the direction.
const moreGlyph = "…"

// What windowed puts in a row that shows none of the lines.
const (
	rowMore  = -1 // the marker on a clipped edge
	rowBlank = -2 // padding under a list shorter than the window
)

// windowed lays n lines into a window of height rows that keeps line focus
// visible, scrolling only when it has to. rows[k] is the index of the line
// drawn at row k, or rowMore, or rowBlank, and start is the first line shown.
//
// The mouse reads it as well as the renderer. A click on row k acts on the
// line windowed put there, so the renderer and the mouse cannot disagree about
// which line is on a row.
//
// from is where the window started on the last frame, or -1 for none. The
// window stays there while focus is on a row that shows a line, and otherwise
// moves only as far as it takes to bring focus onto one. With no previous
// frame it centres on focus. A window that moved under a click would put a
// different line under the pointer before the second press, so a double-click
// would select a line instead of opening one.
//
// marked puts a marker on a clipped edge, so a column that continues off
// screen says so. Without it a list that fits and one that is cut look
// identical, and arrowing past the edge is the only way to find out which you
// are looking at.
//
// Nothing is marked below three rows. The markers cost the first and last row,
// and a two-row column would be all marker and no content. A marker row does
// not count as showing a line, so focus reaching one moves the window, and no
// marker can land on the row the cursor is meant to be showing.
func windowed(n, focus, height int, marked bool, from int) (rows []int, start int) {
	if height <= 0 {
		return nil, 0
	}
	rows = make([]int, height)
	if n <= height {
		for k := range rows {
			rows[k] = k
			if k >= n {
				rows[k] = rowBlank
			}
		}
		return rows, 0
	}
	edges := marked && height >= 3
	// shows is the first and last line a window starting at s shows, markers
	// excluded.
	shows := func(s int) (first, last int) {
		first, last = s, s+height-1
		if edges && s > 0 {
			first++
		}
		if edges && s+height < n {
			last--
		}
		return first, last
	}
	start = clamp(focus-height/2, 0, n-height)
	if from >= 0 && from <= n-height {
		// Stay, or move only as far as it takes to bring focus onto a line.
		// Moving by that distance never puts focus behind a marker the move
		// itself adds; TestWindowedStaysUntilItHasToMove checks it exhaustively
		// over small lists.
		first, last := shows(from)
		start = from
		if focus > last {
			start = from + focus - last
		} else if focus < first {
			start = from - (first - focus)
		}
		start = clamp(start, 0, n-height)
	}
	for k := range rows {
		rows[k] = start + k
	}
	if !edges {
		return rows, start
	}
	if start > 0 {
		rows[0] = rowMore
	}
	if start+height < n {
		rows[height-1] = rowMore
	}
	return rows, start
}

// fill draws the rows windowed laid out: each row's line, more where a row is
// a marker, and blank padding.
func fill(rows []int, lines []string, more string) []string {
	if rows == nil {
		return nil
	}
	out := make([]string, len(rows))
	for k, i := range rows {
		switch {
		case i >= 0:
			out[k] = lines[i]
		case i == rowMore:
			out[k] = more
		}
	}
	return out
}

// windowIndexes is the indexes of the lines windowed shows, with no markers
// and no padding, for a list that draws its own rows rather than taking them
// from fill.
func windowIndexes(total, focus, visible int) []int {
	var out []int
	rows, _ := windowed(total, focus, visible, false, -1)
	for _, i := range rows {
		if i >= 0 {
			out = append(out, i)
		}
	}
	return out
}
