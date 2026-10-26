package settings

// Persistent settings for the panel (Phase A, issue #35): the user's theme
// choice at ~/.config/streambeats/settings.json. The store never holds
// secrets or stream tokens (catalog-cache security contract), writes
// atomically like the catalog store, and absent/malformed files are a
// caller-visible condition so the TUI falls back to defaults without any
// special body (robustness contract).

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Settings is the persisted user configuration.
type Settings struct {
	Theme string `json:"theme,omitempty"`
}

// Store persists and reloads settings as JSON.
type Store struct{ Path string }

// NewStore returns a Store at the given path.
func NewStore(path string) *Store { return &Store{Path: path} }

// DefaultStore opens a Store at the streambeats config dir location.
func DefaultStore() (*Store, error) {
	p, err := DefaultPath()
	if err != nil {
		return nil, err
	}
	return NewStore(p), nil
}

// DefaultPath returns the platform config location
// ($XDG_CONFIG_HOME/streambeats/settings.json or the OS equivalent).
func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("user config dir: %w", err)
	}
	return filepath.Join(dir, "streambeats", "settings.json"), nil
}

// Load returns the persisted settings; a missing file yields zero settings
// with no error (defaults apply), malformed content errors so callers can
// report and fall back by their own policy.
func (s *Store) Load() (Settings, error) {
	raw, err := os.ReadFile(s.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return Settings{}, nil
		}
		return Settings{}, fmt.Errorf("read %s: %w", s.Path, err)
	}
	var set Settings
	if err := json.Unmarshal(raw, &set); err != nil {
		return Settings{}, fmt.Errorf("decode %s: %w", s.Path, err)
	}
	return set, nil
}

// Save atomically writes the settings (write .tmp then rename), mirroring
// the catalog store's durability pattern.
func (s *Store) Save(set Settings) error {
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o750); err != nil {
		return fmt.Errorf("mkdir config dir: %w", err)
	}
	raw, err := json.MarshalIndent(set, "", "  ")
	if err != nil {
		return fmt.Errorf("encode settings: %w", err)
	}
	tmp := s.Path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return fmt.Errorf("write settings: %w", err)
	}
	if err := os.Rename(tmp, s.Path); err != nil {
		return fmt.Errorf("commit settings: %w", err)
	}
	return nil
}
