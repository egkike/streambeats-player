package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// Library navigator (issue #76): the `l` view lists the CURRENT directory
// lazily — directories first, audio files after; Enter descends into a
// directory, `u` ascends (clamped at the root), Enter on audio plays the
// absolute path via mpv. No recursive walk: a $HOME root must list instantly.

// applyList runs the listing cmd and feeds the msg back through Update,
// mirroring the real bubbletea event loop.
func applyList(t *testing.T, m Model, cmd tea.Cmd) Model {
	t.Helper()
	if cmd == nil {
		return m
	}
	m2, _ := m.Update(cmd())
	m = m2.(Model)
	return m
}

func navigatorFS(t *testing.T) (root, dirA, dirB string) {
	t.Helper()
	root = t.TempDir()
	dirA = filepath.Join(root, "Album A")
	dirB = filepath.Join(root, "Album B")
	if err := os.MkdirAll(filepath.Join(dirB, "nested"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(dirA, 0o750); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"z-song.mp3", "a-song.mp3", "cover.jpg"} {
		if err := os.WriteFile(filepath.Join(root, f), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dirB, "nested.mp3"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	return root, dirA, dirB
}

func navigatorModel(t *testing.T, root string) Model {
	dirs := []string{filepath.Join(root, "Album A"), filepath.Join(root, "Album B")}
	files := []string{filepath.Join(root, "a-song.mp3"), filepath.Join(root, "z-song.mp3")}
	return NewModel(Options{
		LibraryRoot:  root,
		LibraryDir:   root,
		LibDirs:      dirs,
		LibraryFiles: files,
		Player:       &fakePlayer{},
		Width:        80,
		Height:       24,
	})
}

func TestLibraryViewRendersDirsFirstLazily(t *testing.T) {
	root, _, _ := navigatorFS(t)
	m := navigatorModel(t, root)
	m, _ = press(t, m, 'l') // the keybind opens the library view
	_ = applyList
	out := m.View()
	if !strings.Contains(out, "Album A") || !strings.Contains(out, "Album B") {
		t.Fatalf("library view must list its directories:\n%s", out)
	}
	if !strings.Contains(out, "A Song") || !strings.Contains(out, "Z Song") {
		t.Fatalf("library view must list the current-dir audio files:\n%s", out)
	}
	// lazy: a nested file from a SUBDIRECTORY must never render at the root
	if strings.Contains(out, "nested") {
		t.Fatalf("library view must not walk subdirectories:\n%s", out)
	}
	// cover.jpg is not audio: filtered
	if strings.Contains(out, "cover") {
		t.Fatalf("non-audio files must be filtered out:\n%s", out)
	}
}

func TestLibraryEnterOnDirDescends(t *testing.T) {
	root, _, dirB := navigatorFS(t)
	m := navigatorModel(t, root)
	m, _ = press(t, m, 'l') // the keybind opens the library view
	_ = applyList
	// combined cursor: dirs first — one down reaches "Album B"
	m, _ = press(t, m, 'j')
	var listCmd tea.Cmd
	m, listCmd = update(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if m.libraryDir != dirB {
		t.Fatalf("libraryDir = %q, want %q", m.libraryDir, dirB)
	}
	if listCmd == nil {
		t.Fatal("descending must return the listing cmd")
	}
	msg := listCmd()
	listed, ok := msg.(LibListedMsg)
	if !ok {
		t.Fatalf("listing cmd returned %T, want LibListedMsg", msg)
	}
	if listed.Err != nil {
		t.Fatalf("LibListedMsg.Err = %v", listed.Err)
	}
	if listed.Dir != dirB || len(listed.Files) != 1 || !strings.HasSuffix(listed.Files[0], "nested.mp3") {
		t.Fatalf("listed dir/files = %q %v, want dirB with nested.mp3", listed.Dir, listed.Files)
	}
}

func TestLibraryAscendClampsAtRoot(t *testing.T) {
	root, _, _ := navigatorFS(t)
	m := navigatorModel(t, root)
	m, _ = press(t, m, 'l') // the keybind opens the library view
	_ = applyList
	// 'u' at the root stays at the root (clamped), and still re-lists
	var cmd tea.Cmd
	m, cmd = press(t, m, 'u')
	if m.libraryDir != root {
		t.Fatalf("ascend at root moved to %q, want clamp at %q", m.libraryDir, root)
	}
	if cmd == nil {
		t.Fatal("ascend must return the listing cmd (re-lists the current dir)")
	}
	m = applyList(t, m, cmd)

	// descend into a dir, then 'u' returns to the root listing
	m, _ = press(t, m, 'j')
	m, descend := update(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if descend == nil {
		t.Fatal("enter on a directory must descend")
	}
	m = applyList(t, m, descend)
	if m.libraryDir == root {
		t.Fatal("descend did not move the current dir")
	}
	m, cmd = press(t, m, 'u')
	if m.libraryDir != root {
		t.Fatalf("ascend landed at %q, want root %q", m.libraryDir, root)
	}
	listed := cmd().(LibListedMsg)
	if listed.Dir != root {
		t.Fatalf("ascend listed %q, want root", listed.Dir)
	}
}

func TestLibraryEnterOnFilePlaysAbsolutePath(t *testing.T) {
	root, _, _ := navigatorFS(t)
	fp := &fakePlayer{}
	dirs := []string{filepath.Join(root, "Album A"), filepath.Join(root, "Album B")}
	m := NewModel(Options{
		LibraryRoot:  root,
		LibraryDir:   root,
		LibDirs:      dirs,
		LibraryFiles: []string{filepath.Join(root, "a-song.mp3"), filepath.Join(root, "z-song.mp3")},
		Player:       fp,
		Width:        80,
		Height:       24,
	})
	m, _ = press(t, m, 'l') // the keybind opens the library view

	// cursor over the FILES segment: down past both dirs (a-song at index 2)
	m, _ = press(t, m, 'j')
	m, _ = press(t, m, 'j')
	var playCmd tea.Cmd
	m, playCmd = update(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if playCmd == nil {
		t.Fatal("enter on an audio file must return the load cmd")
	}
	playCmd() // the event loop runs side-effect cmds; mirror that here

	if len(fp.loaded) != 1 {
		t.Fatalf("play calls = %d, want 1 (loaded: %v)", len(fp.loaded), fp.loaded)
	}
	if want := filepath.Join(root, "a-song.mp3"); fp.loaded[0] != want {
		t.Fatalf("played %q, want the absolute path %q", fp.loaded[0], want)
	}
}
