package ui

// The directory listing behind the explorer: which entries a project may be
// chosen from, how they are ordered, and how one row is drawn.

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// A project is always a directory, so the listing shows directories only.
// Files cannot be selected and cannot be descended into, and a list that
// offers neither is a list of things to scroll past.
//
// A symlink counts when it resolves to a directory. os.ReadDir reports the
// link itself, not its target — DirEntry.IsDir is false for every symlink —
// so deciding this needs a second call per entry, and that is why readDirList
// stats rather than trusting the entry.
type dirRow struct {
	name   string // as it appears in the parent directory
	target string // where a symlink points; empty for a real directory
}

// dirList is one directory on screen: its selectable entries and the cursor.
//
// The read is synchronous. A directory listing is a stat per entry, the same
// cost resolveProject already pays on every pick, and doing it inline keeps
// the explorer free of the message round-trip an async read would need.
type dirList struct {
	dir     string
	rows    []dirRow
	cursor  int
	visible int
	err     error
}

// readDirList lists the directories inside dir, following symlinks.
//
// An unreadable directory is recorded rather than returned: the explorer has
// to keep rendering, and a caller that cannot draw the error cannot report it.
func readDirList(dir string, visible int) *dirList {
	d := &dirList{dir: dir, visible: max(visible, 1)}
	entries, err := os.ReadDir(dir)
	if err != nil {
		d.err = err
		return d
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue // hidden entries stay hidden, as the old explorer did
		}
		if e.IsDir() {
			d.rows = append(d.rows, dirRow{name: e.Name()})
			continue
		}
		if e.Type()&os.ModeSymlink == 0 {
			continue // an ordinary file
		}
		// Stat, not Lstat: the question is what the link points at. A broken
		// link fails here and is dropped, which is the right answer — it
		// cannot be entered and cannot be registered.
		full := filepath.Join(dir, e.Name())
		info, err := os.Stat(full)
		if err != nil || !info.IsDir() {
			continue
		}
		target, err := filepath.EvalSymlinks(full)
		if err != nil {
			target = ""
		}
		d.rows = append(d.rows, dirRow{name: e.Name(), target: target})
	}
	// One alphabetical run. The old explorer sorted on DirEntry.IsDir, which
	// is false for a symlink, so a linked directory sank below every real one
	// and read as missing.
	sort.Slice(d.rows, func(i, j int) bool { return d.rows[i].name < d.rows[j].name })
	return d
}

// path is the full path of the row under the cursor, or "" when the directory
// has no selectable entries.
func (d *dirList) path() string {
	if len(d.rows) == 0 {
		return ""
	}
	return filepath.Join(d.dir, d.rows[d.cursor].name)
}

// move steps the cursor by delta, stopping at either end rather than wrapping:
// in a list long enough to scroll, wrapping reads as a jump to nowhere.
func (d *dirList) move(delta int) {
	if len(d.rows) == 0 {
		return
	}
	d.cursor = clamp(d.cursor+delta, 0, len(d.rows)-1)
}

// into descends into the highlighted entry. A symlink is followed to its
// target, so the path shown after descending is where the files actually are.
func (d *dirList) into() {
	if len(d.rows) == 0 {
		return
	}
	next := d.path()
	if t := d.rows[d.cursor].target; t != "" {
		next = t
	}
	*d = *readDirList(next, d.visible)
}

// parent climbs one level. At the root there is nowhere to go, and doing
// nothing is better than re-reading the same directory and losing the cursor.
func (d *dirList) parent() {
	up := filepath.Dir(d.dir)
	if up == d.dir {
		return
	}
	*d = *readDirList(up, d.visible)
}

// view renders the rows, scrolling only far enough to keep the cursor on
// screen. inner is the width available inside the modal's padding.
func (d *dirList) view(s styleSet, inner int) string {
	if d.err != nil {
		return s.Error.Render(truncate("! "+d.err.Error(), inner))
	}
	if len(d.rows) == 0 {
		return s.Faint.Render("No directories here.")
	}

	var b strings.Builder
	for n, i := range windowIndexes(len(d.rows), d.cursor, d.visible) {
		if n > 0 {
			b.WriteString("\n")
		}
		row := d.rows[i]
		// A symlink keeps its own colour on the cursor row too. Losing it
		// there would hide the distinction exactly when it is being acted on.
		marker, style := "  ", s.Muted
		if row.target != "" {
			style = s.Link
		}
		if i == d.cursor {
			marker = s.Accent.Render("▸ ")
			if row.target == "" {
				style = s.Value
			}
			style = style.Bold(true)
		}
		line := marker + style.Render(row.name)
		// The target distinguishes two links of the same name in different
		// parents, so it earns the space whenever there is room to spare.
		if rest := inner - len([]rune(row.name)) - 5; row.target != "" && rest > 8 {
			line += s.Faint.Render(" → " + truncate(row.target, rest))
		}
		b.WriteString(truncateStyled(line, inner))
	}
	return b.String()
}
