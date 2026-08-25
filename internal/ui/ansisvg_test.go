package ui

// ansiToSVG turns a rendered frame into a standalone SVG at a fixed monospace
// geometry. It is the engine behind assetgen_test.go, kept apart from the
// scenes so each file holds one concern, and it lives in a _test.go file
// because it is tooling rather than part of the binary.
//
// The SGR parser reads 24-bit, 256-colour and basic-16 sequences. Deck's own
// styles emit only 24-bit under the forced TrueColor profile, but a session
// pane replays whatever the hosted program wrote and that program picks its
// own encoding.

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
)

// SVG geometry. The cell size is the advance of a 15px monospace face, which
// is what makes a box-drawing frame join up: any other ratio leaves hairline
// gaps between the rules the pane and the modals draw.
const (
	svgX0    = 16.0 // left margin
	svgY0    = 30.0 // baseline of the first line
	svgCellW = 9.03 // cell advance at font-size 15 — 0.602em, shared by every face in svgFont
	svgLineH = 19.0 // line advance
	svgFont  = `font-family="ui-monospace,'DejaVu Sans Mono',Menlo,Consolas,monospace" font-size="15" xml:space="preserve"`
)

// svgTextTop is how far the background rectangle of a run rises above the
// baseline. A fill drawn from the baseline down would clip every ascender.
const svgTextTop = 14.0

// ansi16 is the conventional xterm RGB for the sixteen base colours. It serves
// both the basic-SGR path and the 0..15 range of the 256-colour cube.
var ansi16 = []string{
	"#000000", "#cd0000", "#00cd00", "#cdcd00", "#0000ee", "#cd00cd", "#00cdcd", "#e5e5e5",
	"#7f7f7f", "#ff0000", "#00ff00", "#ffff00", "#5c5cff", "#ff00ff", "#00ffff", "#ffffff",
}

// svgRun is a maximal stretch of one line that shares a style.
type svgRun struct {
	col    int
	text   string
	fg, bg string
	bold   bool
}

// ansiToSVG renders screen onto a canvas of bg, drawing unstyled text in
// defaultFG. Both are "#rrggbb"; every Deck palette states concrete hex, so
// there is no ANSI index to resolve here.
func ansiToSVG(screen, bg, defaultFG string) string {
	lines := strings.Split(strings.TrimRight(screen, "\n"), "\n")
	cols := 0
	for _, ln := range lines {
		if w := lipgloss.Width(ln); w > cols {
			cols = w
		}
	}
	width := int(svgX0*2 + float64(cols)*svgCellW)
	height := int(svgY0 + svgLineH*float64(len(lines)-1) + svgTextTop)

	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`+"\n",
		width, height, width, height)
	fmt.Fprintf(&b, `<rect width="%d" height="%d" fill="%s"/>`+"\n", width, height, bg)

	for i, ln := range lines {
		y := svgY0 + svgLineH*float64(i)
		runs := parseRuns(ln)
		// Fills first, so the text of the next loop sits on top of them.
		for _, r := range runs {
			if r.bg == "" {
				continue
			}
			fmt.Fprintf(&b, `<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" fill="%s"/>`+"\n",
				svgX0+float64(r.col)*svgCellW, y-svgTextTop,
				float64(lipgloss.Width(r.text))*svgCellW, svgLineH, r.bg)
		}
		fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" %s>`+"\n", svgX0, y, svgFont)
		for _, r := range runs {
			if strings.TrimSpace(r.text) == "" {
				continue // blank cells: any fill already drew them
			}
			fg := r.fg
			if fg == "" {
				fg = defaultFG
			}
			weight := 400
			if r.bold {
				weight = 700
			}
			// textLength pins the run to an exact number of cells. Without it
			// the glyphs advance at whatever the viewer's fallback font uses,
			// and a box-drawing frame 100 cells wide ends up a cell or two
			// short of the verticals it should meet.
			fmt.Fprintf(&b, `<tspan x="%.1f" textLength="%.2f" lengthAdjust="spacingAndGlyphs" fill="%s" font-weight="%d">%s</tspan>`+"\n",
				svgX0+float64(r.col)*svgCellW, float64(lipgloss.Width(r.text))*svgCellW,
				fg, weight, svgEscape(r.text))
		}
		b.WriteString("</text>\n")
	}
	b.WriteString("</svg>")
	return b.String()
}

// parseRuns splits one line into same-style runs, tracking the visible column
// each one starts at. The column is counted with lipgloss.Width rather than
// len, so a wide glyph advances the run that follows it by two cells.
func parseRuns(line string) []svgRun {
	var runs []svgRun
	col, start := 0, 0
	var sb strings.Builder
	fg, bg := "", ""
	bold := false

	emit := func() {
		if sb.Len() > 0 {
			runs = append(runs, svgRun{col: start, text: sb.String(), fg: fg, bg: bg, bold: bold})
			col += lipgloss.Width(sb.String())
			sb.Reset()
		}
		start = col
	}

	for i := 0; i < len(line); {
		if line[i] == 0x1b && i+1 < len(line) && line[i+1] == '[' {
			j := i + 2
			for j < len(line) && line[j] != 'm' {
				j++
			}
			emit()
			applySGR(line[i+2:j], &fg, &bg, &bold)
			i = j + 1
			continue
		}
		r, size := utf8.DecodeRuneInString(line[i:])
		sb.WriteRune(r)
		i += size
	}
	emit()
	return runs
}

// applySGR advances the colour and weight state by one "ESC[ ... m" body.
func applySGR(params string, fg, bg *string, bold *bool) {
	if params == "" {
		*fg, *bg, *bold = "", "", false
		return
	}
	toks := strings.Split(params, ";")
	for k := 0; k < len(toks); k++ {
		switch toks[k] {
		case "0", "":
			*fg, *bg, *bold = "", "", false
		case "1":
			*bold = true
		case "22":
			*bold = false
		case "39":
			*fg = ""
		case "49":
			*bg = ""
		case "38", "48":
			target := fg
			if toks[k] == "48" {
				target = bg
			}
			switch {
			case k+4 < len(toks) && toks[k+1] == "2": // 38;2;r;g;b
				*target = rgbHex(toks[k+2], toks[k+3], toks[k+4])
				k += 4
			case k+2 < len(toks) && toks[k+1] == "5": // 38;5;n
				*target = xterm256Hex(toks[k+2])
				k += 2
			}
		default:
			switch n := atoi(toks[k]); {
			case n >= 30 && n <= 37:
				*fg = ansi16[n-30]
			case n >= 90 && n <= 97:
				*fg = ansi16[n-90+8]
			case n >= 40 && n <= 47:
				*bg = ansi16[n-40]
			case n >= 100 && n <= 107:
				*bg = ansi16[n-100+8]
			}
		}
	}
}

func rgbHex(r, g, b string) string {
	return fmt.Sprintf("#%02x%02x%02x", atoi(r), atoi(g), atoi(b))
}

// xterm256Hex maps a 256-colour index to hex: the grey ramp, the 6x6x6 cube,
// then the base sixteen.
func xterm256Hex(s string) string {
	n := atoi(s)
	switch {
	case n >= 232:
		v := 8 + (n-232)*10
		return fmt.Sprintf("#%02x%02x%02x", v, v, v)
	case n >= 16:
		n -= 16
		level := func(c int) int {
			if c == 0 {
				return 0
			}
			return 55 + c*40
		}
		return fmt.Sprintf("#%02x%02x%02x", level(n/36), level((n/6)%6), level(n%6))
	default:
		return ansi16[n%16]
	}
}

func atoi(s string) int { n, _ := strconv.Atoi(strings.TrimSpace(s)); return n }

func svgEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	return strings.ReplaceAll(s, ">", "&gt;")
}
