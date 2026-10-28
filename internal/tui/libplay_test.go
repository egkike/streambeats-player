package tui

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"streambeats/internal/player"
)

// Library manual-play defects (v1.8.0 validation): replacing a playing
// stream must not let the replaced file's idle-active drive a queue advance,
// and the cursor sync after a library play must target the combined row.

func libPlayModel(t *testing.T, fp *fakePlayer) Model {
	t.Helper()
	root := t.TempDir()
	dirs := []string{filepath.Join(root, "Album A"), filepath.Join(root, "Album B")}
	files := []string{
		filepath.Join(root, "f0.mp3"), filepath.Join(root, "f1.mp3"), filepath.Join(root, "f2.mp3"),
	}
	m := NewModel(Options{
		LibraryRoot:  root,
		LibraryDir:   root,
		LibDirs:      dirs,
		LibraryFiles: files,
		Player:       fp,
		Width:        80,
		Height:       24,
	})
	m, _ = press(t, m, 'l')
	return m
}

var trueVar = true

func idleActiveEvent() eventMsg {
	return eventMsg{event: player.Event{
		Name: "property-change", Property: "idle-active", Bool: &trueVar,
	}}
}

func TestLibraryManualPlaySwallowsReplacedStreamIdle(t *testing.T) {
	fp := &fakePlayer{}
	m := libPlayModel(t, fp)

	// play file 0 (cursor 2 = first file after the two dirs)
	m, _ = press(t, m, 'j')
	m, _ = press(t, m, 'j')
	var playCmd tea.Cmd
	m, playCmd = update(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	playCmd()

	// the replaced stream's idle-active arrives while the new file loads
	// (BEFORE its loadedMsg): it must NOT advance the queue
	m2, _ := m.Update(idleActiveEvent())
	m = m2.(Model)
	if len(fp.loaded) != 1 {
		t.Fatalf("spurious advance loaded %d files (want 1): %v", len(fp.loaded), fp.loaded)
	}
	if m.playingLib != 0 {
		t.Fatalf("playingLib moved to %d by the replaced stream's idle-active", m.playingLib)
	}

	// the load resolves: playing=true, cursor syncs to the combined row
	m2, _ = m.Update(loadedMsg{title: "001 / f0"})
	m = m2.(Model)
	if !m.playing {
		t.Fatal("the played file must be playing after its loadedMsg")
	}
}

func TestLibraryCursorSyncTargetsCombinedRow(t *testing.T) {
	fp := &fakePlayer{}
	m := libPlayModel(t, fp)

	// play file 0 (combined row 2: two dirs first)
	m, _ = press(t, m, 'j')
	m, _ = press(t, m, 'j')
	var playCmd tea.Cmd
	m, playCmd = update(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	playCmd()
	m2, _ := m.Update(loadedMsg{title: "f0"})
	m = m2.(Model)

	if m.libraryCursor != 2 {
		t.Fatalf("cursor = %d, want 2 (the played file's combined row)", m.libraryCursor)
	}
}

func TestLoadWindowStaysOpenUntilProgressConfirms(t *testing.T) {
	fp := &fakePlayer{}
	m := libPlayModel(t, fp)

	// two dirs first: move to the first file row (combined cursor 2)
	m, _ = press(t, m, 'j')
	m, _ = press(t, m, 'j')
	var playCmd tea.Cmd
	m, playCmd = update(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	playCmd()

	// the replaced stream's idle-active can arrive AFTER loadedMsg — the
	// load command returns before mpv finishes switching files. The window
	// must stay open until a real progress reading confirms the new source.
	m2, _ := m.Update(loadedMsg{title: "f0"})
	m = m2.(Model)
	m2, _ = m.Update(idleActiveEvent())
	m = m2.(Model)
	if len(fp.loaded) != 1 {
		t.Fatalf("idle-active after loadedMsg advanced the queue (loads: %v)", fp.loaded)
	}
	if m.playingLib != 0 {
		t.Fatalf("playingLib moved to %d", m.playingLib)
	}

	// the first real progress reading proves the new source is live and
	// closes the window: a LATER genuine idle-active advances again
	m2, _ = m.Update(progressMsg{pos: 1, dur: 100})
	m = m2.(Model)
	var nextCmd tea.Cmd
	m2b, cmd2 := m.Update(idleActiveEvent())
	m = m2b.(Model)
	nextCmd = cmd2
	if m.playingLib != 1 || nextCmd == nil {
		t.Fatalf("after the window closed, a genuine idle-active must advance (playingLib=%d cmd=%v)", m.playingLib, nextCmd)
	}
}

func TestLibraryPlayingMarkerMatchesByIndex(t *testing.T) {
	root := t.TempDir()
	fp := &fakePlayer{}
	files := []string{
		filepath.Join(root, "Song.mp3"),
		filepath.Join(root, "middle.mp3"),
		filepath.Join(root, "Song.flac"), // same basename as file 0
	}
	m := NewModel(Options{
		LibraryRoot:  root,
		LibraryDir:   root,
		LibraryFiles: files,
		Player:       fp,
		Width:        80,
		Height:       24,
	})
	m, _ = press(t, m, 'l')
	// play file 0 ("Song.mp3"); the real title carries the parent-dir note
	var playCmd tea.Cmd
	m, playCmd = update(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	playCmd()
	m2, _ := m.Update(loadedMsg{title: libraryEntryTitle(files[0])})
	m = m2.(Model)

	out := m.View()
	marked := strings.Count(out, "♪")
	if marked != 1 {
		t.Fatalf("view marks %d rows as playing (want exactly 1):\n%s", marked, out)
	}
}
