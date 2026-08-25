package ui

// The twelve themes: their ids, labels, and the five source colours each one
// states. palette.go derives the rest; styles.go turns a palette into styles.

import (
	"github.com/tripledownab/deck/internal/store"
)

// themeDef is one row in the theme picker.
type themeDef struct{ id, label, desc string }

// themes are the same ids, labels and source colours as cathode's, so a theme
// name means the same thing in both tools. They are deliberately *not* shared
// through a common file: the two apps have different role vocabularies —
// cathode needs diff-gutter and "you"-label colours Deck has no use for,
// and Deck's SelectBg is a dark tint where cathode's equivalent is a
// bright fill. A shared schema would have to satisfy both forever to save an
// edit made twice a year.
var themes = []themeDef{
	{"cinder", "Cinder", "warm ash with one live ember"},
	{"bbs", "BBS", "the original neon 16-colour palette"},
	{"dracula", "Dracula", "purple & pink on dark slate"},
	{"nord", "Nord", "muted arctic blues"},
	{"solarized", "Solarized Dark", "low-contrast teal & amber"},
	{"tokyonight", "Tokyo Night", "soft neon on deep blue"},
	{"gruvbox", "Gruvbox Dark", "warm retro earth tones"},
	{"onedark", "One Dark", "Atom's classic dark"},
	{"monokai", "Monokai", "the Sublime Text classic"},
	{"catppuccin", "Catppuccin Mocha", "pastel on dark mauve"},
	{"github", "GitHub Dark", "GitHub's dark mode"},
	{"rosepine", "Rosé Pine", "muted rose & pine"},
}

// defaultThemeID is store.DefaultTheme, not a second copy of the string: the
// value is one rule with two readers — the settings default and the fallback
// for an id that does not resolve — and two constants would let a first run
// and a corrupt-file run disagree about which theme you get.
const defaultThemeID = store.DefaultTheme

// themeLabel is the display name for an id, or the id when unknown.
func themeLabel(id string) string {
	for _, t := range themes {
		if t.id == id {
			return t.label
		}
	}
	return id
}
