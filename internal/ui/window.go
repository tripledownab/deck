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
// visible, centred on it once the list is longer than the window. rows[k] is
// the index of the line drawn at row k, or rowMore, or rowBlank.
//
// It is the layout half of window, kept separate because windowIndexes reads
// it too, for a list that draws its own rows.
//
// marked puts a marker on a clipped edge, so a column that continues off
// screen says so. Without it a list that fits and one that is cut look
// identical, and arrowing past the edge is the only way to find out which you
// are looking at.
//
// Nothing is marked below three rows. The markers cost the first and last row,
// and a two-row column would be all marker and no content. Centring keeps
// focus off both edges whenever a marker goes there, so no marker can land on
// the row the cursor is meant to be showing.
func windowed(n, focus, height int, marked bool) []int {
	if height <= 0 {
		return nil
	}
	rows := make([]int, height)
	if n <= height {
		for k := range rows {
			rows[k] = k
			if k >= n {
				rows[k] = rowBlank
			}
		}
		return rows
	}
	start := clamp(focus-height/2, 0, n-height)
	for k := range rows {
		rows[k] = start + k
	}
	if !marked || height < 3 {
		return rows
	}
	if start > 0 {
		rows[0] = rowMore
	}
	if start+height < n {
		rows[height-1] = rowMore
	}
	return rows
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

// window returns the height rows of lines that keep focus visible, with more
// on a clipped edge. Pass "" to mark nothing.
//
// The marker arrives already styled. This file draws and does not know the
// palette.
func window(lines []string, focus, height int, more string) []string {
	return fill(windowed(len(lines), focus, height, more != ""), lines, more)
}

// windowIndexes is the indexes of the lines windowed shows, with no markers
// and no padding, for a list that draws its own rows rather than taking them
// from fill.
func windowIndexes(total, focus, visible int) []int {
	var out []int
	for _, i := range windowed(total, focus, visible, false) {
		if i >= 0 {
			out = append(out, i)
		}
	}
	return out
}
