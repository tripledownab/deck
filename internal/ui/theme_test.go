package ui

import (
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/tripledownab/deck/internal/store"
)

// TestEveryThemeHasASource stops a row appearing in the picker that resolves
// to the default palette — the user picks "Nord" and nothing changes.
func TestEveryThemeHasASource(t *testing.T) {
	for _, th := range themes {
		if _, ok := sources[th.id]; !ok {
			t.Errorf("theme %q is listed in the picker but has no colours", th.id)
		}
	}
	if len(sources) != len(themes) {
		t.Errorf("%d palettes for %d themes — one is unreachable", len(sources), len(themes))
	}
}

// TestThemesAreDistinct guards against a copy-paste palette: two rows that
// look different in the list and identical on screen.
func TestThemesAreDistinct(t *testing.T) {
	seen := map[lipglossColor]string{}
	for _, th := range themes {
		p := paletteFor(th.id)
		key := lipglossColor{string(p.Fg), string(p.Accent), string(p.Border)}
		if other, dup := seen[key]; dup {
			t.Errorf("themes %q and %q render identically", other, th.id)
		}
		seen[key] = th.id
	}
}

type lipglossColor struct{ fg, accent, border string }

// TestPaletteForUnknownFallsBack pins the choice to degrade rather than fail:
// a hand-edited settings file should cost colours, not the session.
func TestPaletteForUnknownFallsBack(t *testing.T) {
	got := paletteFor("no-such-theme")
	want := paletteFor(defaultThemeID)
	if got.Accent != want.Accent {
		t.Errorf("unknown theme gave accent %q, want the default %q", got.Accent, want.Accent)
	}
}

// TestDerivedDimLevelsAreOrdered checks the three dim tones stay in order for
// every theme. Border must recede behind Faint, and Faint behind Muted; if the
// blend ever inverts, rules would shout and labels would vanish.
func TestDerivedDimLevelsAreOrdered(t *testing.T) {
	for _, th := range themes {
		p := paletteFor(th.id)
		src := sources[th.id]
		bgLum := luminance(src.bg)
		border, faint, muted := luminance(string(p.Border)), luminance(string(p.Faint)), luminance(string(p.Muted))

		// Ordering is by distance from the background, which works for a dark
		// theme and for a light one alike.
		d := func(l float64) float64 {
			if l > bgLum {
				return l - bgLum
			}
			return bgLum - l
		}
		if !(d(border) < d(faint) && d(faint) < d(muted)) {
			t.Errorf("%s: dim levels out of order — border %.3f, faint %.3f, muted %.3f (bg %.3f)",
				th.id, d(border), d(faint), d(muted), bgLum)
		}
	}
}

func luminance(hex string) float64 {
	r, g, b := hexToRGB(hex)
	return 0.2126*float64(r) + 0.7152*float64(g) + 0.0722*float64(b)
}

func TestBlendEndpoints(t *testing.T) {
	if got := blend("#000000", "#FFFFFF", 0); got != "#000000" {
		t.Errorf("blend at 0 = %q, want the first colour", got)
	}
	if got := blend("#000000", "#FFFFFF", 1); got != "#FFFFFF" {
		t.Errorf("blend at 1 = %q, want the second colour", got)
	}
	if got := blend("#000000", "#FFFFFF", 0.5); got != "#7F7F7F" {
		t.Errorf("blend at 0.5 = %q, want the midpoint", got)
	}
}

// ---- picker ----

func themeModel(t *testing.T, active string) Model {
	t.Helper()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	return New(&store.State{}, "bash", nil).WithTheme(active)
}

func TestPickerOpensOnTheActiveTheme(t *testing.T) {
	m := themeModel(t, "nord")
	mm, _ := m.openThemePicker()
	m = mm.(Model)
	if got := m.picker.selected(); got != "nord" {
		t.Errorf("picker opened on %q, want the active theme", got)
	}
}

// TestPickerPreviewsAsItMoves covers the point of the modal: you judge a theme
// by the app behind it, not by its name.
func TestPickerPreviewsAsItMoves(t *testing.T) {
	m := themeModel(t, "cinder")
	mm, _ := m.openThemePicker()
	m = mm.(Model)

	before := m.styles.P.Accent
	mm, _ = m.pickerKey(tea.KeyMsg{Type: tea.KeyDown})
	m = mm.(Model)

	if m.styles.P.Accent == before {
		t.Error("moving the cursor did not restyle the frame")
	}
	if m.theme == "cinder" {
		t.Error("the previewed theme was not applied to the model")
	}
}

// TestPickerCancelRestoresTheOriginal is the regression for a real bug: the
// original id was read from the model on the cancelling keypress, but preview
// had already overwritten it, so esc kept whatever was last previewed.
func TestPickerCancelRestoresTheOriginal(t *testing.T) {
	m := themeModel(t, "dracula")
	want := m.styles.P.Accent

	mm, _ := m.openThemePicker()
	m = mm.(Model)
	for range 3 { // wander well away from dracula
		mm, _ = m.pickerKey(tea.KeyMsg{Type: tea.KeyDown})
		m = mm.(Model)
	}
	mm, _ = m.pickerKey(tea.KeyMsg{Type: tea.KeyEsc})
	m = mm.(Model)

	if m.picker != nil {
		t.Fatal("esc left the picker open")
	}
	if m.theme != "dracula" {
		t.Errorf("theme = %q after esc, want dracula restored", m.theme)
	}
	if m.styles.P.Accent != want {
		t.Errorf("accent = %q after esc, want %q", m.styles.P.Accent, want)
	}
}

func TestPickerCommitPersists(t *testing.T) {
	m := themeModel(t, "cinder")
	mm, _ := m.openThemePicker()
	m = mm.(Model)

	mm, _ = m.pickerKey(tea.KeyMsg{Type: tea.KeyDown})
	m = mm.(Model)
	chosen := m.theme

	mm, _ = m.pickerKey(tea.KeyMsg{Type: tea.KeyEnter})
	m = mm.(Model)

	if m.picker != nil {
		t.Fatal("enter left the picker open")
	}
	if m.fault != nil {
		t.Fatalf("commit reported: %v", m.fault)
	}
	if got := store.LoadSettings().Theme; got != chosen {
		t.Errorf("persisted theme = %q, want %q", got, chosen)
	}
}

// TestHelpCoversEveryBinding is the regression for the help modal falling
// behind. It was a hand-written table beside the bindings, so adding the theme
// key updated one and silently left the other stale.
//
// Generating the modal is the fix; this asserts the generation actually covers
// what the app dispatches, rather than a subset someone remembered.
func TestHelpCoversEveryBinding(t *testing.T) {
	m := New(&store.State{}, "bash", nil)

	documented := map[string]bool{}
	for _, g := range m.keys.helpGroups() {
		if g.title == "" {
			t.Error("a help group has no title")
		}
		for _, k := range g.keys {
			h := k.Help()
			if h.Key == "" || h.Desc == "" {
				t.Errorf("a binding in %q has no help text", g.title)
			}
			documented[h.Desc] = true
		}
	}

	// Keys the user can press that must appear somewhere in the modal.
	for _, want := range []string{"theme", "new session", "add project", "quit"} {
		if !documented[want] {
			t.Errorf("the help modal never mentions %q", want)
		}
	}
}

// TestHelpMentionsTheThemeKey pins the specific gap that prompted the fix.
func TestHelpMentionsTheThemeKey(t *testing.T) {
	m := New(&store.State{}, "bash", nil)
	m.width, m.height = 100, 44

	// Both spellings of the key, and the description beside them. Matching on
	// a bare "t " would be satisfied by "next " or "output " and assert
	// nothing.
	view := m.helpView()
	for _, want := range []string{"^g t", "theme"} {
		if !strings.Contains(view, want) {
			t.Errorf("help modal is missing %q", want)
		}
	}
	if !regexp.MustCompile(`\s{2,}t\s{2,}theme`).MatchString(ansi.Strip(view)) {
		t.Error("help modal has no chrome row binding a bare t to the theme picker")
	}
}
