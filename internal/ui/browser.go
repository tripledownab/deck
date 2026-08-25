package ui

// The directory explorer used when adding a project, wrapped around
// bubbles/filepicker with two of its defaults changed.

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/filepicker"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// browser is the directory explorer used to pick a repository to register.
//
// It wraps bubbles/filepicker in directory mode. With DirAllowed set and
// FileAllowed clear, the picker resolves its two enter-bound actions in our
// favour: enter selects the highlighted directory, while → and l descend into
// it. That is the whole interaction.
type browser struct {
	fp      filepicker.Model
	problem string
}

func newBrowser(start string, height int) (*browser, tea.Cmd) {
	fp := filepicker.New()
	fp.DirAllowed = true
	fp.FileAllowed = false
	fp.ShowHidden = false
	fp.ShowPermissions = false
	fp.ShowSize = false
	fp.AutoHeight = false
	fp.CurrentDirectory = start

	// esc has to cancel the modal, so it cannot also mean "up one directory".
	// The filepicker binds both by default; the other Back keys stay.
	fp.KeyMap.Back = key.NewBinding(
		key.WithKeys("h", "backspace", "left"),
		key.WithHelp("←/h", "up"),
	)
	fp.SetHeight(height)

	b := &browser{fp: fp}
	return b, fp.Init()
}

// update advances the picker. The returned path is non-empty only when the
// user selected a directory this tick.
func (b *browser) update(msg tea.Msg) (tea.Cmd, string) {
	var cmd tea.Cmd
	b.fp, cmd = b.fp.Update(msg)
	if ok, path := b.fp.DidSelectFile(msg); ok {
		return cmd, path
	}
	return cmd, ""
}

func (b *browser) view(s styleSet, width, height int) string {
	boxWidth := min(width-8, 76)
	if boxWidth < 30 {
		boxWidth = max(width-4, 20)
	}
	inner := boxWidth - 4

	var body string
	body += s.Title.Render("Add project") + "\n"
	body += s.Faint.Render(truncate(b.fp.CurrentDirectory, inner)) + "\n"
	body += s.Rule.Render(strings.Repeat("─", inner)) + "\n\n"
	body += b.fp.View() + "\n"

	if b.problem != "" {
		body += "\n" + s.Error.Render("! "+wrap(b.problem, inner-2)) + "\n"
	}
	body += "\n" + s.Footer.Render("↑/↓ move · →/l open · ←/h up · ↵ pick this directory · esc cancel")

	box := s.Modal.Width(boxWidth).Padding(1, 2).Render(body)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}

// browserStart picks the directory the explorer opens in: the parent of the
// selected project, so the sibling repositories are already on screen. It
// falls back to the working directory, then home.
func browserStart(current string) string {
	for _, candidate := range []string{filepath.Dir(current), cwdOrEmpty(), homeOrEmpty()} {
		if candidate == "" || candidate == "." {
			continue
		}
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}
	return "/"
}

func cwdOrEmpty() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	return dir
}

func homeOrEmpty() string {
	dir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return dir
}
