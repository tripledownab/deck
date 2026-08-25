package store

// Persisted user preferences, kept apart from the projects and sessions so a
// corrupt one cannot cost the other.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Settings is the persisted user preference set. Kept in its own file rather
// than inside State: projects and sessions are working data that changes as
// you use the app, preferences are chosen once, and a corrupt one should not
// cost you the other.
//
// Forward-compatible in the same way as State — unknown fields are ignored on
// load and a missing one takes its default, so an older binary reading a newer
// file degrades to defaults instead of failing.
type Settings struct {
	Theme string `json:"theme"`

	// Agent is the program new sessions run, remembered from the last one
	// opened. It is a default and not a lock: each session records what it
	// actually started, so changing this never rewrites a session that exists.
	Agent string `json:"agent"`
}

// DefaultTheme is the id used when nothing is persisted. It matches cathode's
// default so the two tools open looking the same.
const DefaultTheme = "cinder"

// DefaultAgent is the program a session runs when nothing has been chosen.
const DefaultAgent = "claude"

func DefaultSettings() Settings {
	return Settings{Theme: DefaultTheme, Agent: DefaultAgent}
}

func settingsPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "settings.json"), nil
}

// LoadSettings reads the settings file, falling back to defaults for anything
// missing or unreadable.
//
// Unlike State, a settings failure is not worth stopping for: the worst case
// is the wrong colours, where the worst case for a corrupt State is losing
// track of running sessions. So this returns defaults rather than an error.
func LoadSettings() Settings {
	s := DefaultSettings()
	path, err := settingsPath()
	if err != nil {
		return s
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return s
	}
	_ = json.Unmarshal(data, &s) // keep defaults on a parse error
	if s.Theme == "" {
		s.Theme = DefaultTheme
	}
	if s.Agent == "" {
		s.Agent = DefaultAgent
	}
	return s
}

// Save writes the settings file atomically.
func (s Settings) Save() error {
	path, err := settingsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("encode settings: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("replace %s: %w", path, err)
	}
	return nil
}
