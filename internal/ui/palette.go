package ui

// Colour: the eleven palette roles, the five source colours each theme
// states, and the blending that derives one from the other.

import (
	"image/color"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// palette is Deck's whole colour vocabulary. Every style is built from it,
// so a new theme is a new palette and nothing else.
//
// Colours are concrete rather than lipgloss.AdaptiveColor. Every theme here is
// a dark theme, and an AdaptiveColor whose light and dark values are identical
// promises an adaptation it does not perform — a claim in the type that the
// data does not back. If a light theme ever lands, this is the decision to
// revisit, not to work around.
//
// Nothing sets a window background: the pane hosts another program's output,
// and painting behind it would fight whatever that program chose.
type palette struct {
	Fg       lipgloss.Color // primary text
	Muted    lipgloss.Color // secondary text: labels, counts
	Faint    lipgloss.Color // hints and timestamps
	Border   lipgloss.Color // rules and inactive frames
	Accent   lipgloss.Color // the active thing: cursor, focused border, working dot
	Dead     lipgloss.Color // an agent that exited, and error text
	SelectBg lipgloss.Color // behind a selected row

	// The cursor colours are written straight into an emulated terminal cell
	// rather than handed to lipgloss.
	CursorBg color.Color
	CursorFg color.Color
}

// source is the handful of values each upstream theme actually defines. The
// Deck palette is derived from these rather than hand-picking eleven
// colours twelve times, so every theme separates its three dim levels by the
// same amount and none of them can drift into an unreadable combination.
type source struct {
	fg     string // primary text
	dim    string // the theme's comment / dim grey
	accent string // its signature colour
	dead   string // its red
	bg     string // its background
}

var sources = map[string]source{
	// Cinder's values are tuned, not derived — see the contrast notes below.
	"cinder":     {fg: "#E4E2DD", dim: "#7D7A72", accent: "#E8552F", dead: "#E24C6B", bg: "#14130F"},
	"bbs":        {fg: "#FFFFFF", dim: "#7F7F7F", accent: "#FFB000", dead: "#FF0000", bg: "#000000"},
	"dracula":    {fg: "#F8F8F2", dim: "#6272A4", accent: "#FFB86C", dead: "#FF5555", bg: "#282A36"},
	"nord":       {fg: "#ECEFF4", dim: "#4C566A", accent: "#D08770", dead: "#BF616A", bg: "#2E3440"},
	"solarized":  {fg: "#EEE8D5", dim: "#586E75", accent: "#CB4B16", dead: "#DC322F", bg: "#002B36"},
	"tokyonight": {fg: "#C0CAF5", dim: "#565F89", accent: "#FF9E64", dead: "#F7768E", bg: "#1A1B26"},
	"gruvbox":    {fg: "#EBDBB2", dim: "#928374", accent: "#FE8019", dead: "#FB4934", bg: "#282828"},
	"onedark":    {fg: "#ABB2BF", dim: "#5C6370", accent: "#D19A66", dead: "#E06C75", bg: "#282C34"},
	"monokai":    {fg: "#F8F8F2", dim: "#75715E", accent: "#FD971F", dead: "#F92672", bg: "#272822"},
	"catppuccin": {fg: "#CDD6F4", dim: "#6C7086", accent: "#FAB387", dead: "#F38BA8", bg: "#1E1E2E"},
	"github":     {fg: "#C9D1D9", dim: "#8B949E", accent: "#F0883E", dead: "#F85149", bg: "#0D1117"},
	"rosepine":   {fg: "#E0DEF4", dim: "#6E6A86", accent: "#EBBCBA", dead: "#EB6F92", bg: "#191724"},
}

// paletteFor builds a theme's palette. An unknown id falls back to the default
// rather than returning an error: a bad id in a settings file should cost the
// user their colours, not their session.
func paletteFor(id string) palette {
	src, ok := sources[id]
	if !ok {
		src = sources[defaultThemeID]
	}
	return palette{
		Fg:    lipgloss.Color(src.fg),
		Faint: lipgloss.Color(src.dim),
		// Three dim levels from one: Muted sits between the dim tone and the
		// foreground so labels stay readable, Border sits between it and the
		// background so a rule recedes. Deriving keeps the separation equal
		// across every theme.
		Muted:    lipgloss.Color(blend(src.dim, src.fg, 0.35)),
		Border:   lipgloss.Color(blend(src.dim, src.bg, 0.55)),
		Accent:   lipgloss.Color(src.accent),
		Dead:     lipgloss.Color(src.dead),
		SelectBg: lipgloss.Color(blend(src.accent, src.bg, 0.86)),
		CursorBg: mustRGBA(src.accent),
		CursorFg: mustRGBA(src.bg),
	}
}

// blend mixes two hex colours, t of the way from a to b, and returns hex.
// Linear in sRGB: not perceptually uniform, but consistent, dependency-free,
// and only ever used to step between two tones of the same family.
func blend(a, b string, t float64) string {
	ar, ag, ab := hexToRGB(a)
	br, bg, bb := hexToRGB(b)
	mix := func(x, y uint8) uint8 { return uint8(float64(x) + (float64(y)-float64(x))*t) }
	return rgbToHex(mix(ar, br), mix(ag, bg), mix(ab, bb))
}

func hexToRGB(h string) (r, g, b uint8) {
	h = strings.TrimPrefix(h, "#")
	if len(h) != 6 {
		return 0, 0, 0
	}
	v, err := strconv.ParseUint(h, 16, 32)
	if err != nil {
		return 0, 0, 0
	}
	return uint8(v >> 16), uint8(v >> 8), uint8(v)
}

func rgbToHex(r, g, b uint8) string {
	const digits = "0123456789ABCDEF"
	out := []byte{'#', 0, 0, 0, 0, 0, 0}
	for i, v := range []uint8{r, g, b} {
		out[1+i*2] = digits[v>>4]
		out[2+i*2] = digits[v&0x0F]
	}
	return string(out)
}

func mustRGBA(hex string) color.Color {
	r, g, b := hexToRGB(hex)
	return color.RGBA{R: r, G: g, B: b, A: 0xFF}
}
