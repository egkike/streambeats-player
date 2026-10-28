package catalog

import (
	"os"
	"path/filepath"
	"testing"
)

// XDG music-dir resolution (issue #76): the library root default must come
// from the user's real music folder (es systems: Música), never a hardcoded
// English path.

func writeUserDirs(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "user-dirs.dirs"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestXDGMusicDirParsesUserDirs(t *testing.T) {
	home := t.TempDir()
	cfg := writeUserDirs(t, "# comment\nXDG_MUSIC_DIR=\"$HOME/Música\"\nXDG_DOWNLOAD_DIR=\"$HOME/Descargas\"\n")

	got, err := XDGMusicDir(filepath.Join(cfg, "user-dirs.dirs"), home)
	if err != nil {
		t.Fatalf("XDGMusicDir: %v", err)
	}
	if want := filepath.Join(home, "Música"); got != want {
		t.Fatalf("XDGMusicDir = %q, want %q", got, want)
	}
}

func TestXDGMusicDirFallsBackToMusicWithoutConfig(t *testing.T) {
	home := t.TempDir()
	got, err := XDGMusicDir(filepath.Join(home, "does-not-exist"), home)
	if err != nil {
		t.Fatalf("XDGMusicDir: %v", err)
	}
	if want := filepath.Join(home, "Music"); got != want {
		t.Fatalf("fallback = %q, want %q", got, want)
	}
}

func TestXDGMusicDirFallsBackWhenEntryMissing(t *testing.T) {
	home := t.TempDir()
	cfg := writeUserDirs(t, "XDG_DOWNLOAD_DIR=\"$HOME/Descargas\"\n")
	got, err := XDGMusicDir(filepath.Join(cfg, "user-dirs.dirs"), home)
	if err != nil {
		t.Fatalf("XDGMusicDir: %v", err)
	}
	if want := filepath.Join(home, "Music"); got != want {
		t.Fatalf("fallback = %q, want %q", got, want)
	}
}

func TestListDirIsLazyAndSorted(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "b-album")
	if err := os.MkdirAll(filepath.Join(root, "a-folder", "nested"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(sub, 0o750); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"z-track.mp3", "a-track.mp3", "notes.txt", "cover.jpg"} {
		if err := os.WriteFile(filepath.Join(root, f), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// a nested mp3 must NEVER appear: the listing is lazy, not recursive
	if err := os.WriteFile(filepath.Join(sub, "inner.mp3"), nil, 0o600); err != nil {
		t.Fatal(err)
	}

	dirs, files, err := ListDir(root)
	if err != nil {
		t.Fatalf("ListDir: %v", err)
	}
	if len(dirs) != 2 || dirs[0] != filepath.Join(root, "a-folder") || dirs[1] != sub {
		t.Fatalf("dirs = %v, want [a-folder, b-album] (sorted, dirs first)", dirs)
	}
	if len(files) != 2 || files[0] != filepath.Join(root, "a-track.mp3") || files[1] != filepath.Join(root, "z-track.mp3") {
		t.Fatalf("files = %v, want only the root-level mp3s sorted", files)
	}
}

func TestDefaultLibraryPathFollowsXDG(t *testing.T) {
	home := t.TempDir()
	cfg := writeUserDirs(t, "XDG_MUSIC_DIR=\"$HOME/Música\"\n")
	t.Setenv("XDG_CONFIG_HOME", cfg)
	t.Setenv("HOME", home)

	got, err := DefaultLibraryPath()
	if err != nil {
		t.Fatalf("DefaultLibraryPath: %v", err)
	}
	if want := filepath.Join(home, "Música", "StreamBeats"); got != want {
		t.Fatalf("DefaultLibraryPath = %q, want %q", got, want)
	}
}
