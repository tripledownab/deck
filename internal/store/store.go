// Package store holds Deck's persisted model: the projects you registered
// and the sessions opened against them.
//
// The store is deliberately small. It records what a session *is* (which
// project, which branch, which directory on disk) and never what a session
// *said* — the agent process owns the conversation, and claude already
// persists its own transcript under ~/.claude/projects. Keeping the two
// separate means a corrupt Deck state file costs you a sidebar, not work.
package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// State is the whole persisted document.
type State struct {
	Projects []Project `json:"projects"`
	Sessions []Session `json:"sessions"`
}

// Load reads the state file. A missing file is an empty store, which is the
// first-run case. Any other read or parse failure is returned: a state file
// that exists but does not parse means the user has sessions we cannot see,
// and silently starting empty would invite Deck to create duplicates.
func Load() (*State, error) {
	path, err := statePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &State{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	// Sessions recorded before Agent existed name no program. Filling them in
	// here, once, is what lets every reader treat the field as always set
	// instead of guessing at the point of use.
	for i := range s.Sessions {
		if s.Sessions[i].Agent == "" {
			s.Sessions[i].Agent = DefaultAgent
		}
	}
	return &s, nil
}

// Save writes the state file atomically, so an interrupted write cannot leave
// a half-written document that the next Load refuses to parse.
func (s *State) Save() error {
	path, err := statePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("encode state: %w", err)
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
