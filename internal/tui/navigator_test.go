package tui

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"streambeats/internal/settings"
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

// --- library root picker (issue #164, specs S1-S7) ----------------------
// The library root becomes editable through a dedicated ModeRootPicker that
// reuses the lazy-navigator vocabulary (j/k + arrows, enter descends, u
// ascends, o fixes, esc returns to Settings). Every test drives the real
// keys against an IN-MEMORY lister: no network, no mpv, no real filesystem
// cache (only a settings.json inside t.TempDir() for the persistence pins).

// fakeDir is one in-memory directory: its subdirectories, its audio files and
// an optional listing error (unreadable folder).
type fakeDir struct {
	dirs  []string
	files []string
	err   error
}

// pickerFS is the fake directory lister keyed by absolute path.
type pickerFS map[string]fakeDir

func (l pickerFS) List(dir string) ([]string, []string, error) {
	d, ok := l[dir]
	if !ok {
		return nil, nil, &fs.PathError{Op: "open", Path: dir, Err: fs.ErrNotExist}
	}
	if d.err != nil {
		return nil, nil, d.err
	}
	return append([]string(nil), d.dirs...), append([]string(nil), d.files...), nil
}

// pickerModel builds a model with the injected lister and a real (temp-dir)
// settings store. theme/panelTerminal ride along to pin the full-set persist.
func pickerModel(t *testing.T, lister pickerFS, home, root, themeID, terminal string) (Model, *settings.Store) {
	t.Helper()
	store := settings.NewStore(filepath.Join(t.TempDir(), "settings.json"))
	m := NewModel(Options{
		LibraryRoot:   root,
		LibraryDir:    root,
		ListDir:       lister.List,
		UserHomeDir:   home,
		SettingsStore: store,
		Theme:         themeID,
		ThemeDark:     true,
		PanelTerminal: terminal,
		Width:         80,
		Height:        24,
	})
	return m, store
}

// applyMsg runs one command and feeds its message back through Update,
// mirroring the real bubbletea event loop.
func applyMsg(t *testing.T, m Model, cmd tea.Cmd) Model {
	t.Helper()
	if cmd == nil {
		return m
	}
	m2, _ := m.Update(cmd())
	return m2.(Model)
}

// openRootPicker drives the real keys from the catalog: c opens Settings, the
// cursor walks down to the library-root row and enter opens the picker on
// $HOME, whose listing is applied.
func openRootPicker(t *testing.T, m Model) Model {
	t.Helper()
	m, _ = press(t, m, 'c')
	if m.mode != ModeConfig {
		t.Fatalf("c opened mode %v, want ModeConfig", m.mode)
	}
	for m.configIndex < configRootRowIndex(m) {
		m, _ = update(t, m, tea.KeyMsg{Type: tea.KeyDown})
	}
	if m.configIndex != configRootRowIndex(m) {
		t.Fatalf("settings cursor stopped at %d, want the library-root row %d", m.configIndex, configRootRowIndex(m))
	}
	var cmd tea.Cmd
	m, cmd = update(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.mode != ModeRootPicker {
		t.Fatalf("enter on the library-root row opened mode %v, want ModeRootPicker", m.mode)
	}
	return applyMsg(t, m, cmd)
}

// rowContaining returns the rendered line carrying substr ("" when absent).
func rowContaining(view, substr string) string {
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, substr) {
			return line
		}
	}
	return ""
}

const pickerHome = "/home/u"

// S1/S2: the picker is a dedicated mode reached from the settings row, opens
// at $HOME and lists DIRECTORIES only, each with a trailing slash.
func TestRootPickerOpensAtHomeWithDirsOnly(t *testing.T) {
	lister := pickerFS{
		pickerHome: {
			dirs:  []string{"/home/u/Downloads", "/home/u/Music"},
			files: []string{"/home/u/song.mp3"},
		},
	}
	m, _ := pickerModel(t, lister, pickerHome, "/old/root", "system", "")
	m = openRootPicker(t, m)

	if m.pickerDir != pickerHome {
		t.Fatalf("picker opened at %q, want $HOME %q", m.pickerDir, pickerHome)
	}
	if len(m.pickerDirs) != 2 {
		t.Fatalf("picker listed %v, want the two subdirectories", m.pickerDirs)
	}
	view := stripANSI(m.View())
	if !strings.Contains(view, "Music/ ") && !strings.Contains(view, "Music/\n") && !strings.HasSuffix(rowContaining(view, "Music/"), "Music/") {
		t.Errorf("directories must render with a trailing slash:\n%s", view)
	}
	if !strings.Contains(rowContaining(view, "Downloads"), "Downloads/") {
		t.Errorf("directories must render with a trailing slash:\n%s", view)
	}
	if strings.Contains(view, "song.mp3") {
		t.Errorf("the picker lists directories only, an audio file leaked in:\n%s", view)
	}
	if !strings.Contains(view, "Library root — "+pickerHome) {
		t.Errorf("picker header lost the current path:\n%s", view)
	}
}

// S2: j/k and the arrow keys move the cursor exactly like the rest of the app,
// clamped at both ends.
func TestRootPickerCursorMovesWithJKAndArrows(t *testing.T) {
	lister := pickerFS{
		pickerHome: {dirs: []string{"/home/u/a", "/home/u/b", "/home/u/c"}},
	}
	m, _ := pickerModel(t, lister, pickerHome, "/old/root", "system", "")
	m = openRootPicker(t, m)

	steps := []struct {
		msg  tea.KeyMsg
		want int
	}{
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}, 1},
		{tea.KeyMsg{Type: tea.KeyDown}, 2},
		{tea.KeyMsg{Type: tea.KeyDown}, 2}, // clamp at the end
		{tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}, 1},
		{tea.KeyMsg{Type: tea.KeyUp}, 0},
		{tea.KeyMsg{Type: tea.KeyUp}, 0}, // clamp at the start
	}
	for i, step := range steps {
		m, _ = update(t, m, step.msg)
		if m.pickerCursor != step.want {
			t.Fatalf("step %d (%s): cursor = %d, want %d", i, step.msg.String(), m.pickerCursor, step.want)
		}
	}
}

// S2/S4: enter descends into a subdirectory; an empty listing descends and
// teaches instead of blocking ("Descender + enseñar").
func TestRootPickerEnterDescendsAndTeachesOnEmptyListing(t *testing.T) {
	lister := pickerFS{
		pickerHome:           {dirs: []string{"/home/u/Albums", "/home/u/Empty"}},
		"/home/u/Albums":     {dirs: []string{"/home/u/Albums/One"}, files: []string{"/home/u/Albums/x.mp3"}},
		"/home/u/Empty":      {},
		"/home/u/Albums/One": {},
	}
	m, _ := pickerModel(t, lister, pickerHome, "/old/root", "system", "")
	m = openRootPicker(t, m)

	// cursor 1 = Empty
	m, _ = press(t, m, 'j')
	var cmd tea.Cmd
	m, cmd = update(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m = applyMsg(t, m, cmd)
	if m.pickerDir != "/home/u/Empty" {
		t.Fatalf("enter descended to %q, want /home/u/Empty", m.pickerDir)
	}
	wantTeaching := "no sub-directories here — o fixes this folder as the root"
	if m.status != wantTeaching {
		t.Fatalf("empty listing status = %q, want %q", m.status, wantTeaching)
	}
	if !strings.Contains(stripANSI(m.View()), wantTeaching) {
		t.Errorf("the teaching line must render in the status slot:\n%s", stripANSI(m.View()))
	}

	// Enter again on an empty listing never blocks: the folder stays the one
	// that `o` will fix.
	m, _ = update(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.pickerDir != "/home/u/Empty" {
		t.Errorf("enter on an empty listing moved to %q, want the same folder", m.pickerDir)
	}

	// A populated folder descends silently.
	m, cmd = press(t, m, 'u')
	m = applyMsg(t, m, cmd)
	m, cmd = update(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // cursor 0 = Albums
	m = applyMsg(t, m, cmd)
	if m.pickerDir != "/home/u/Albums" {
		t.Fatalf("descended to %q, want /home/u/Albums", m.pickerDir)
	}
	if m.status != "" {
		t.Fatalf("descending into a populated folder painted %q, want silence", m.status)
	}
}

// S7: the picker's ascend clamps at $HOME — NOT at the (old) library root.
func TestRootPickerAscendClampsAtHome(t *testing.T) {
	lister := pickerFS{
		pickerHome:               {dirs: []string{"/home/u/Music"}},
		"/home/u/Music":          {dirs: []string{"/home/u/Music/Old"}},
		"/home/u/Music/Old":      {dirs: []string{"/home/u/Music/Old/Deep"}},
		"/home/u/Music/Old/Deep": {},
	}
	m, _ := pickerModel(t, lister, pickerHome, "/home/u/Music/Old", "system", "")
	m = openRootPicker(t, m)

	// at $HOME: clamped, re-lists the base
	m, cmd := press(t, m, 'u')
	if m.pickerDir != pickerHome {
		t.Fatalf("ascend at $HOME moved to %q, want the clamp at %q", m.pickerDir, pickerHome)
	}
	if cmd == nil {
		t.Fatal("ascend must re-list the clamped directory")
	}
	m = applyMsg(t, m, cmd)

	// descend to the OLD library root, then u must cross ABOVE it (to
	// $HOME/Music) — the old root is not a clamp anymore.
	m, cmd = update(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m = applyMsg(t, m, cmd)
	if m.pickerDir != "/home/u/Music" {
		t.Fatalf("descended to %q, want /home/u/Music", m.pickerDir)
	}
	m, cmd = update(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m = applyMsg(t, m, cmd)
	if m.pickerDir != "/home/u/Music/Old" {
		t.Fatalf("descended to %q, want the old root /home/u/Music/Old", m.pickerDir)
	}
	m, cmd = press(t, m, 'u')
	m = applyMsg(t, m, cmd)
	if m.pickerDir != "/home/u/Music" {
		t.Fatalf("ascend landed at %q, want /home/u/Music (the root is NOT the clamp)", m.pickerDir)
	}
}

// S3: the fixed root is marked with COLOR ONLY — derived from libraryRoot, so
// the paint adds zero painted width and no glyph, and unmarking is free by
// construction (the old root loses the paint as soon as the root moves).
func TestRootPickerMarksTheFixedRootByColorOnly(t *testing.T) {
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(prev)

	root := "/home/u/Bravo"
	lister := pickerFS{
		pickerHome:      {dirs: []string{"/home/u/Alpha", root}},
		"/home/u/Alpha": {dirs: []string{"/home/u/Alpha/Sub"}},
		root:            {dirs: []string{"/home/u/Bravo/Sub"}},
	}
	m, store := pickerModel(t, lister, pickerHome, root, "system", "")
	m = openRootPicker(t, m)

	view := m.View()
	rootRow := rowContaining(view, "Bravo/")
	otherRow := rowContaining(view, "Alpha/")
	if rootRow == "" || otherRow == "" {
		t.Fatalf("picker rows missing:\n%s", stripANSI(view))
	}
	// system-on-dark paints Playing with ANSI 42 (38;5;42).
	if !strings.Contains(rootRow, "38;5;42") {
		t.Fatalf("the fixed root row lost its color marking: %q", rootRow)
	}
	if !strings.Contains(otherRow, "38;5;212") {
		t.Fatalf("the cursor row lost the selected paint: %q", otherRow)
	}
	// Zero painted width change: SGR only, no glyph, no extra column.
	if got := stripANSI(rootRow); got != "  Bravo/" {
		t.Fatalf("marked root row paints %q, want the plain gutter row %q", got, "  Bravo/")
	}

	// Unmark is free: fixing another folder moves the paint with it.
	m, cmd := press(t, m, 'o') // cursor 0 = Alpha
	m = applyMsg(t, m, cmd)
	if m.libraryRoot != "/home/u/Alpha" {
		t.Fatalf("o fixed %q, want the cursor folder /home/u/Alpha", m.libraryRoot)
	}
	if saved, err := store.Load(); err != nil || saved.LibraryRoot != "/home/u/Alpha" {
		t.Fatalf("persisted libraryRoot = %q (err %v), want /home/u/Alpha", saved.LibraryRoot, err)
	}
	m = openRootPicker(t, m)
	if row := rowContaining(m.View(), "Bravo/"); strings.Contains(row, "38;5;42") {
		t.Fatalf("the previous root kept its paint: %q", row)
	}
	// Move the cursor off the new root so the selection paint does not mask
	// its mark (the cursor always wins over the root mark).
	m, _ = press(t, m, 'j')
	if row := rowContaining(m.View(), "Alpha/"); !strings.Contains(row, "38;5;42") {
		t.Fatalf("the new root is not painted: %q", row)
	}
}

// S5: `o` fixes the CURSOR folder and persists the FULL settings set (theme +
// libraryRoot + panelTerminal) atomically, with an honest status.
func TestRootPickerOFixesCursorFolderAndPersistsFullSet(t *testing.T) {
	lister := pickerFS{
		pickerHome:       {dirs: []string{"/home/u/Albums", "/home/u/Other"}},
		"/home/u/Albums": {files: []string{"/home/u/Albums/01.mp3"}},
		"/home/u/Other":  {files: []string{"/home/u/Other/02.mp3"}},
	}
	m, store := pickerModel(t, lister, pickerHome, "/old/root", "cute", "kitty")
	m = openRootPicker(t, m)

	var cmd tea.Cmd
	m, cmd = press(t, m, 'o') // cursor 0 = Albums
	if m.mode != ModeConfig {
		t.Fatalf("o returned to mode %v, want ModeConfig", m.mode)
	}
	if cmd == nil {
		t.Fatal("o must persist through the save cmd")
	}
	m = applyMsg(t, m, cmd)

	wantStatus := "library root: /home/u/Albums (saved)"
	if m.status != wantStatus {
		t.Fatalf("status = %q, want %q", m.status, wantStatus)
	}
	if !strings.Contains(stripANSI(m.View()), wantStatus) {
		t.Errorf("the saved status must render:\n%s", stripANSI(m.View()))
	}
	saved, err := store.Load()
	if err != nil {
		t.Fatalf("load after save: %v", err)
	}
	if saved.LibraryRoot != "/home/u/Albums" || saved.Theme != "cute" || saved.PanelTerminal != "kitty" {
		t.Errorf("full-set persist lost siblings: %+v", saved)
	}
	if m.libraryRoot != "/home/u/Albums" || m.libraryDir != "/home/u/Albums" {
		t.Errorf("root dir bookkeeping = %q/%q, want the fixed folder", m.libraryRoot, m.libraryDir)
	}
}

// S3/S4: with an empty listing, `o` fixes the CURRENT directory (the header
// path) and announces the empty folder (music comes via sync).
func TestRootPickerOWithEmptyListingFixesCurrentDir(t *testing.T) {
	lister := pickerFS{
		pickerHome:      {dirs: []string{"/home/u/Empty"}},
		"/home/u/Empty": {},
	}
	m, store := pickerModel(t, lister, pickerHome, "/old/root", "system", "")
	m = openRootPicker(t, m)
	var cmd tea.Cmd
	m, cmd = update(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m = applyMsg(t, m, cmd)
	if m.pickerDir != "/home/u/Empty" {
		t.Fatalf("descended to %q, want /home/u/Empty", m.pickerDir)
	}
	m, cmd = press(t, m, 'o')
	m = applyMsg(t, m, cmd)

	want := "library root: /home/u/Empty (saved) — empty folder (music comes via sync)"
	if m.status != want {
		t.Fatalf("status = %q, want %q", m.status, want)
	}
	if saved, err := store.Load(); err != nil || saved.LibraryRoot != "/home/u/Empty" {
		t.Fatalf("persisted libraryRoot = %q (err %v), want the current folder", saved.LibraryRoot, err)
	}
}

// S4 scenario 3: an album-only target (subdirectories, no audio files) is a
// VALID root — it saves SILENTLY, without the empty-folder append.
func TestRootPickerOAlbumOnlyRootSavesSilently(t *testing.T) {
	lister := pickerFS{
		pickerHome: {dirs: []string{"/home/u/StreamBeats"}},
		"/home/u/StreamBeats": {
			dirs: []string{"/home/u/StreamBeats/Album A", "/home/u/StreamBeats/Album B"},
		},
	}
	m, store := pickerModel(t, lister, pickerHome, "/old/root", "system", "")
	m = openRootPicker(t, m)
	// `o` fixes the CURSOR folder: StreamBeats holds album folders only.
	m, cmd := press(t, m, 'o')
	m = applyMsg(t, m, cmd)

	want := "library root: /home/u/StreamBeats (saved)"
	if m.status != want {
		t.Fatalf("status = %q, want %q (album layouts are valid roots)", m.status, want)
	}
	if saved, err := store.Load(); err != nil || saved.LibraryRoot != "/home/u/StreamBeats" {
		t.Fatalf("persisted libraryRoot = %q (err %v), want /home/u/StreamBeats", saved.LibraryRoot, err)
	}
}

// S4 read failure: an unreadable target persists NOTHING, mutates nothing and
// reports an honest status (Artboard C).
func TestRootPickerOReadFailurePersistsNothing(t *testing.T) {
	lister := pickerFS{
		pickerHome:       {dirs: []string{"/home/u/Locked"}},
		"/home/u/Locked": {err: &fs.PathError{Op: "open", Path: "/home/u/Locked", Err: fs.ErrPermission}},
	}
	m, store := pickerModel(t, lister, pickerHome, "/old/root", "system", "")
	m = openRootPicker(t, m)
	m, _ = press(t, m, 'j') // cursor 0 = Locked

	before := m.libraryRoot
	m, cmd := press(t, m, 'o')
	if cmd != nil {
		t.Fatalf("an unreadable target must not fire a save cmd")
	}
	if m.libraryRoot != before {
		t.Errorf("read failure mutated the root: %q, want %q", m.libraryRoot, before)
	}
	if m.mode != ModeRootPicker {
		t.Errorf("read failure left the picker (mode = %v)", m.mode)
	}
	wantStatus := "can not read /home/u/Locked: permission denied"
	if m.status != wantStatus {
		t.Fatalf("status = %q, want %q", m.status, wantStatus)
	}
	if _, err := store.Load(); err == nil {
		if saved, _ := store.Load(); saved.LibraryRoot != "" {
			t.Errorf("read failure persisted a root: %q", saved.LibraryRoot)
		}
	}
	if _, err := os.Stat(store.Path); err == nil {
		t.Errorf("read failure wrote %s, want nothing persisted", store.Path)
	}
}

// S6: esc inside the picker returns to Settings with settings.json untouched
// (no data side-effect) and leaves the settings screen's own exit intact.
func TestRootPickerEscReturnsToSettingsUntouched(t *testing.T) {
	lister := pickerFS{pickerHome: {dirs: []string{"/home/u/a"}}}
	m, store := pickerModel(t, lister, pickerHome, "/old/root", "system", "")
	m = openRootPicker(t, m)
	m, _ = press(t, m, 'j')

	before := m.libraryRoot
	m, cmd := update(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if cmd != nil {
		t.Errorf("esc must not fire a command")
	}
	if m.mode != ModeConfig {
		t.Fatalf("esc landed in mode %v, want ModeConfig", m.mode)
	}
	if m.libraryRoot != before {
		t.Errorf("esc mutated the root: %q, want %q", m.libraryRoot, before)
	}
	if _, err := os.Stat(store.Path); err == nil {
		t.Errorf("esc wrote %s, want nothing persisted", store.Path)
	}
	// The settings screen keeps its own exit: a second esc leaves Settings.
	m, _ = update(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.mode != ModeCatalog {
		t.Fatalf("esc from Settings landed in mode %v, want the catalog", m.mode)
	}
}

// S6: the read-only row copy becomes the picker affordance — one row, one
// budget, no "(settings.json)" leftover, and no extra rows on the screen.
func TestSettingsRootRowOffersThePicker(t *testing.T) {
	m, _ := pickerModel(t, pickerFS{}, pickerHome, "/home/u/Music", "system", "")
	m, _ = press(t, m, 'c')
	view := stripANSI(m.View())
	want := "library root: /home/u/Music (enter to change)"
	if !strings.Contains(view, want) {
		t.Fatalf("settings view is missing %q:\n%s", want, view)
	}
	if strings.Contains(view, "settings.json") {
		t.Errorf("the read-only settings.json copy must be gone:\n%s", view)
	}
	// ONE library-root row: the budget does not grow.
	if got := strings.Count(view, "library root:"); got != 1 {
		t.Fatalf("settings screen renders %d library-root rows, want exactly 1", got)
	}
	// The row is the LAST row of the flat settings cursor.
	if configRootRowIndex(m) != configRowCount(m)-1 {
		t.Fatalf("library-root row index %d is not the last row of %d", configRootRowIndex(m), configRowCount(m))
	}
}

// S7: after a fix the library view refetches from the NEW root on next open —
// lazily, through the same injected lister (no recursion, no real FS).
func TestLibraryRootFixRefetchesOnNextLibraryOpen(t *testing.T) {
	lister := pickerFS{
		pickerHome:    {dirs: []string{"/home/u/New"}},
		"/home/u/New": {dirs: []string{"/home/u/New/Album"}, files: []string{"/home/u/New/01.mp3"}},
	}
	m, _ := pickerModel(t, lister, pickerHome, "/old/root", "system", "")
	m = openRootPicker(t, m)
	// `o` fixes the CURSOR folder /home/u/New: its own listing is what the
	// lazily opened library view will show.
	var cmd tea.Cmd
	m, cmd = press(t, m, 'o')
	m = applyMsg(t, m, cmd)
	if m.libraryRoot != "/home/u/New" {
		t.Fatalf("fixed root = %q, want /home/u/New", m.libraryRoot)
	}

	// back to the catalog, then open the library: the listing comes from the
	// NEW root through the injected lister.
	m, _ = update(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	m, cmd = press(t, m, 'l')
	if cmd == nil {
		t.Fatal("opening the library after a root fix must refetch the listing")
	}
	listed, ok := cmd().(LibListedMsg)
	if !ok {
		t.Fatalf("library listing cmd returned %T, want LibListedMsg", cmd())
	}
	if listed.Dir != "/home/u/New" {
		t.Fatalf("library refetched %q, want the new root /home/u/New", listed.Dir)
	}
	if len(listed.Files) != 1 || len(listed.Dirs) != 1 {
		t.Fatalf("refetched listing = %v / %v, want one dir and one file", listed.Dirs, listed.Files)
	}
}

// Guard (in-cycle finding, #164): replacing the library listing while a file
// plays leaves the stale playingLib pointing past the new slice — queuePrev
// must answer honestly instead of indexing out of range (TUI must not crash).
func TestQueuePrevAnswersHonestlyWhenLibraryListingReplaced(t *testing.T) {
	m := testModel(Options{Player: &fakePlayer{}})
	m.mode = ModeLibrary
	m.library = []string{"/lib/only.mp3"}
	m.playing = true
	m.playingLib = 4 // stale: points past the replaced listing
	m, cmd := update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'<'}})
	if cmd != nil {
		t.Fatal("stale-index queuePrev spawned a command")
	}
	if m.playingLib != 4 || m.playing != true {
		t.Fatalf("stale-index queuePrev mutated state: lib %d playing %v", m.playingLib, m.playing)
	}
	if !strings.Contains(m.status, "the library listing changed") {
		t.Fatalf("status = %q, want the honest listing-changed note", m.status)
	}
}

// R3-001 (in-cycle advisory, re-derived as a REAL race on main): a slow
// listing for a folder the picker already left must mutate nothing — pressing
// u while a stale listing is still in flight keeps the HEAD listing instead of
// being clobbered (pickerDir/pickerDirs/cursor reset) by the old message.
func TestRootPickerStaleListingForAFolderAlreadyLeftDropsMutatesNothing(t *testing.T) {
	lister := pickerFS{
		pickerHome:           {dirs: []string{"/home/u/Albums"}},
		"/home/u/Albums":     {dirs: []string{"/home/u/Albums/One"}},
		"/home/u/Albums/One": {},
	}
	m, _ := pickerModel(t, lister, pickerHome, "/old/root", "system", "")
	m = openRootPicker(t, m)

	// descend into Albums with its listing applied
	var cmd tea.Cmd
	m, cmd = update(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m = applyMsg(t, m, cmd)
	if m.pickerDir != "/home/u/Albums" {
		t.Fatalf("descend landed on %q, want /home/u/Albums", m.pickerDir)
	}

	// ascend back to $HOME: the model moves optimistically and lists $HOME;
	// the ascend listing is left IN FLIGHT
	m, ascendCmd := update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})
	if m.pickerDir != pickerHome {
		t.Fatalf("ascend landed on %q, want %q", m.pickerDir, pickerHome)
	}

	// the SLOW listing for /home/u/Albums arrives after we already left
	stale := RootListedMsg{Dir: "/home/u/Albums", Dirs: []string{"/home/u/Albums/One"}}
	m, _ = update(t, m, stale)
	if m.pickerDir != pickerHome {
		t.Fatalf("stale listing moved the picker to %q, want %q", m.pickerDir, pickerHome)
	}
	if len(m.pickerDirs) != 0 {
		t.Fatalf("stale listing brought folders %v, want empty $HOME rows", m.pickerDirs)
	}

	// the real $HOME listing (in flight) still applies when it lands
	m = applyMsg(t, m, ascendCmd)
	if m.pickerDir != pickerHome || len(m.pickerDirs) != 1 {
		t.Fatalf("real listing lost: dir %q dirs %v", m.pickerDir, m.pickerDirs)
	}
}
