package coord

// The per-project shared log: append-only, capped on read, trimmed on disk.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Note is one entry in a project's shared log.
type Note struct {
	At      time.Time `json:"at"`
	Session string    `json:"session"`
	Text    string    `json:"text"`
}

// AppendNote adds a line to the project's shared log.
//
// The log is append-only and one file per project. Append-only because
// several agents write concurrently and a read-modify-write would silently
// drop entries; one file per project because that is the sharing boundary.
// The log is trimmed when it passes compactAbove, back down to keepNotes.
// Trimming is what stops an append-only file becoming permanent: without it
// the notes grow for the life of the project even though only the tail is ever
// read.
const (
	compactAbove = 400
	keepNotes    = 100
)

func (c *Coordinator) AppendNote(sessionID, text string) error {
	// Held across the write and the compaction below. Appends from other
	// agents are serialised through here, so the rewrite cannot lose a line
	// that landed between counting and renaming.
	c.mu.Lock()
	defer c.mu.Unlock()

	me, ok := c.sessions[sessionID]
	if !ok {
		return fmt.Errorf("unknown session")
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return fmt.Errorf("note is empty")
	}

	line, err := json.Marshal(Note{At: time.Now(), Session: me.Name, Text: text})
	if err != nil {
		return err
	}
	path := c.notesPath(me.ProjectID)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open notes: %w", err)
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}

	// Count once per project, then track it, so the common append does not
	// re-read the whole file.
	if _, counted := c.noteLines[me.ProjectID]; !counted {
		c.noteLines[me.ProjectID] = countLines(path)
	} else {
		c.noteLines[me.ProjectID]++
	}
	if c.noteLines[me.ProjectID] > compactAbove {
		if n, err := compact(path, keepNotes); err == nil {
			c.noteLines[me.ProjectID] = n
		}
	}
	return nil
}

// compact rewrites the log keeping only its newest keep lines, and returns how
// many remain. The rewrite goes through a temp file and a rename so a crash
// cannot leave a half-written log.
func compact(path string, keep int) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	lines := make([]string, 0, keep)
	for _, l := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(l) != "" {
			lines = append(lines, l)
		}
	}
	if len(lines) <= keep {
		return len(lines), nil
	}
	lines = lines[len(lines)-keep:]

	tmp := path + ".tmp"
	body := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(tmp, []byte(body), 0o644); err != nil {
		return 0, err
	}
	if err := os.Rename(tmp, path); err != nil {
		return 0, err
	}
	return len(lines), nil
}

func countLines(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	n := 0
	for _, l := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(l) != "" {
			n++
		}
	}
	return n
}

// maxNotes caps what a reader gets back. A shared log is re-read by every
// agent that asks, and an uncapped one quietly becomes the most expensive file
// in the project.
const maxNotes = 50

// Notes returns the most recent notes for the session's project, oldest first.
func (c *Coordinator) Notes(sessionID string) ([]Note, error) {
	c.mu.Lock()
	me, ok := c.sessions[sessionID]
	c.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("unknown session")
	}

	data, err := os.ReadFile(c.notesPath(me.ProjectID))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read notes: %w", err)
	}

	var out []Note
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var n Note
		if json.Unmarshal([]byte(line), &n) != nil {
			continue // a torn line from a crash is skipped, not fatal
		}
		out = append(out, n)
	}
	if len(out) > maxNotes {
		out = out[len(out)-maxNotes:]
	}
	return out, nil
}

func (c *Coordinator) notesPath(projectID string) string {
	return filepath.Join(c.notesDir, projectID+".jsonl")
}
