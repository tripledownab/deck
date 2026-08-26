package ui

// The modal wrapper around the directory listing: the frame, the keys, and
// where the explorer opens. The listing itself is in dirlist.go.

import (
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// browser is the directory explorer used to pick a project to register.
//
// It shows directories only, following symlinks. A project is a directory, so
// listing files offered rows that could be neither entered nor chosen. Enter
// selects the highlighted directory; → and l descend into it. That split is
// the whole interaction, and it is the reason the explorer needs no notion of
// a "current file".
type browser struct {
	list    *dirList
	problem string
}

func newBrowser(start string, height int) *browser {
	return &browser{list: readDirList(start, height)}
}

// dir is the directory being shown, which a caller needs in order to reopen
// the explorer where it was after refusing a selection.
func (b *browser) dir() string { return b.list.dir }

// update advances the explorer. The returned path is non-empty only when the
// user selected a directory on this key.
//
// esc is not handled here. It closes the modal, which is the caller's business
// — binding it to "up one directory" as well would leave the modal with no way
// out.
func (b *browser) update(msg tea.KeyMsg) string {
	switch msg.String() {
	case "up", "k":
		b.list.move(-1)
	case "down", "j":
		b.list.move(1)
	case "right", "l":
		b.list.into()
	case "left", "h", "backspace":
		b.list.parent()
	case "enter":
		return b.list.path()
	}
	return ""
}

func (b *browser) view(s styleSet, width, height int) string {
	boxWidth := min(width-8, 76)
	if boxWidth < 30 {
		boxWidth = max(width-4, 20)
	}
	inner := boxWidth - 4

	var body strings.Builder
	body.WriteString(s.Title.Render("Add project") + "\n")
	body.WriteString(s.Faint.Render(truncate(b.list.dir, inner)) + "\n")
	body.WriteString(s.Rule.Render(strings.Repeat("─", inner)) + "\n\n")
	body.WriteString(b.list.view(s, inner) + "\n")

	if b.problem != "" {
		body.WriteString("\n" + s.Error.Render("! "+wrap(b.problem, inner-2)) + "\n")
	}
	body.WriteString("\n" + s.Footer.Render(
		"↑/↓ move · →/l open · ←/h up · ↵ pick this directory · esc cancel"))

	box := s.Modal.Width(boxWidth).Padding(1, 2).Render(body.String())
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
