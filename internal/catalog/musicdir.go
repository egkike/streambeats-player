package catalog

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// XDG music-dir resolution (issue #76): the library root default follows the
// user's real music folder from ~/.config/user-dirs.dirs (es systems: Música),
// falling back to $HOME/Music when the config is absent or has no entry.

// MusicDir resolves the user's music directory from their live XDG config
// (convenience wrapper over XDGMusicDir).
func MusicDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return XDGMusicDir(xdgUserDirsPath(), home)
}

// XDGMusicDir resolves the user's music directory from a user-dirs.dirs file.
// configPath may be empty/missing — the fallback is always $HOME/Music.
// Values are POSIX shell-ish: quoted with " and optionally $HOME-prefixed.
func XDGMusicDir(configPath, home string) (string, error) {
	f, err := os.Open(configPath) //nolint:gosec // caller-supplied config path is the API
	if err != nil {
		return filepath.Join(home, "Music"), nil
	}
	defer func() { _ = f.Close() }()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, value, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(name) != "XDG_MUSIC_DIR" {
			continue
		}
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"`)
		if strings.HasPrefix(value, "$HOME") {
			value = filepath.Join(home, strings.TrimPrefix(value, "$HOME"))
		}
		if !filepath.IsAbs(value) {
			return filepath.Join(home, "Music"), nil
		}
		return filepath.Clean(value), nil
	}
	return filepath.Join(home, "Music"), nil
}

// audioExts (issue #84): the formats the navigator lists and plays. mpv
// handles all of them; `sync` stays mp3-only (its own contract).
var audioExts = map[string]bool{
	".mp3": true, ".flac": true, ".ogg": true, ".oga": true, ".opus": true,
	".m4a": true, ".aac": true, ".wav": true, ".wma": true,
}

// ListDir returns the DIRECT children of dir: subdirectories first (sorted),
// then audio files (sorted). It is deliberately lazy — no recursion — so a
// $HOME-sized root lists instantly and descent stays explicit (issue #76 S3).
func ListDir(dir string) (dirs []string, files []string, err error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, err
	}
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, filepath.Join(dir, e.Name()))
			continue
		}
		if audioExts[strings.ToLower(filepath.Ext(e.Name()))] {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(dirs)
	sort.Strings(files)
	return dirs, files, nil
}
