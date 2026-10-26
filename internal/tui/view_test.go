package tui

// Unit tests for the pure rendering helpers (no terminal involved).

import (
	"math"
	"strings"
	"testing"
	"unicode/utf8"

	"streambeats/internal/catalog"
)

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name    string
		seconds float64
		want    string
	}{
		{"zero", 0, "0:00"},
		{"seconds only", 5, "0:05"},
		{"minute", 65, "1:05"},
		{"just under an hour", 3599, "59:59"},
		{"one hour", 3600, "1:00:00"},
		{"over an hour", 3725, "1:02:05"},
		{"negative clamps to zero", -12, "0:00"},
		{"NaN clamps to zero", math.NaN(), "0:00"},
		{"fractional truncates", 61.9, "1:01"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatDuration(tc.seconds); got != tc.want {
				t.Fatalf("formatDuration(%v) = %q, want %q", tc.seconds, got, tc.want)
			}
		})
	}
}

func TestProgressBar(t *testing.T) {
	tests := []struct {
		name               string
		position, duration float64
		width              int
		want               string
	}{
		{"zero width", 5, 10, 0, ""},
		{"no duration", 0, 0, 10, "░░░░░░░░░░"},
		{"half", 5, 10, 10, "█████░░░░░"},
		{"empty", 0, 10, 10, "░░░░░░░░░░"},
		{"full", 10, 10, 10, "██████████"},
		{"overrun clamps", 20, 10, 10, "██████████"},
		{"negative position clamps", -5, 10, 4, "░░░░"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := progressBar(tc.position, tc.duration, tc.width); got != tc.want {
				t.Fatalf("progressBar(%v, %v, %d) = %q, want %q",
					tc.position, tc.duration, tc.width, got, tc.want)
			}
		})
	}
}

func TestWindowBounds(t *testing.T) {
	tests := []struct {
		name               string
		n, cursor, height  int
		wantStart, wantEnd int
	}{
		{"empty list", 0, 0, 5, 0, 0},
		{"zero height", 10, 0, 0, 0, 0},
		{"fits entirely", 3, 2, 5, 0, 3},
		{"top edge", 10, 0, 4, 0, 4},
		{"middle centers", 10, 5, 4, 3, 7},
		{"bottom edge", 10, 9, 4, 6, 10},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			start, end := windowBounds(tc.n, tc.cursor, tc.height)
			if start != tc.wantStart || end != tc.wantEnd {
				t.Fatalf("windowBounds(%d, %d, %d) = (%d, %d), want (%d, %d)",
					tc.n, tc.cursor, tc.height, start, end, tc.wantStart, tc.wantEnd)
			}
		})
	}
}

func TestLibraryTitle(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{"album and track prefix", "/lib/01 - lofi-chill/03 - rainy-day.mp3", "Rainy Day"},
		{"single prefix", "02 - night-drive.mp3", "Night Drive"},
		{"no prefix", "song.mp3", "Song"},
		{"underscores", "01 - calm_waves.mp3", "Calm Waves"},
		{"no extension", "01 - intro", "Intro"},
		{"empty base", "/lib/01 - album/01 - .mp3", "unknown track"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := libraryTitle(tc.path); got != tc.want {
				t.Fatalf("libraryTitle(%q) = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}

func TestLibraryEntryTitle(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{"album and track", "/lib/01 - lofi-chill/03 - rainy-day.mp3", "Lofi Chill / Rainy Day"},
		{"flat file", "/lib/song.mp3", "Lib / Song"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := libraryEntryTitle(tc.path); got != tc.want {
				t.Fatalf("libraryEntryTitle(%q) = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}

func TestHumanizeSlug(t *testing.T) {
	tests := []struct{ in, want string }{
		{"cold-morning", "Cold Morning"},
		{"single", "Single"},
		{"two_words", "Two Words"},
		{"", ""},
	}
	for _, tc := range tests {
		if got := humanizeSlug(tc.in); got != tc.want {
			t.Fatalf("humanizeSlug(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name  string
		in    string
		width int
		want  string
	}{
		{"short", "abc", 10, "abc"},
		{"exact", "abcde", 5, "abcde"},
		{"long", "abcdefgh", 5, "abcd…"},
		{"width one", "abc", 1, "…"},
		{"zero width", "abc", 0, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := truncate(tc.in, tc.width); got != tc.want {
				t.Fatalf("truncate(%q, %d) = %q, want %q", tc.in, tc.width, got, tc.want)
			}
		})
	}
}

func TestHelpTextListsEveryAction(t *testing.T) {
	help := helpText()
	for _, want := range []string{"k/↑", "j/↓", "enter", "esc", "l", "space", "s", "< >", "+/-", "q"} {
		if !strings.Contains(help, want) {
			t.Fatalf("help text %q is missing %q", help, want)
		}
	}
}

func TestMPVArgs(t *testing.T) {
	args := mpvArgs("/run/user/1000/streambeats-mpv.sock")
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"--no-video",
		"--no-terminal",
		"--idle",
		"--input-ipc-server=/run/user/1000/streambeats-mpv.sock",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("mpv args %q are missing %q", joined, want)
		}
	}
}

// --- Panel layout (issue #31): bottom-anchored, status slot reserved. ---

func layoutModel(status string) Model {
	return NewModel(Options{
		Catalog: &catalog.Catalog{Albums: []catalog.AlbumEntry{
			{Title: "Alpha"},
			{Title: "Beta"},
		}},
		PlayerNote: status,
	})
}

// S2/S3: with fewer items than viewport rows, the player bar sits at the
// bottom: the blank rows between the last list row and the bottom block are
// the list's viewport padding.
func TestViewAnchorsBottomBlockToTheBottomPanelRight(t *testing.T) {
	m := layoutModel("")
	m.width, m.height = 60, 24
	out := m.View()
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	total := len(lines)
	barAt := strings.Index(out, "nothing playing")
	if barAt < 0 {
		t.Fatalf("player bar missing:\n%s", out)
	}
	// The bottom block (player bar 2 lines + reserved status 1 + help 3)
	// occupies the last 6 lines; nothing after it but the trailing newline.
	if got := strings.Join(lines[total-6:], "\n"); !strings.Contains(got, "nothing playing") {
		t.Fatalf("player bar not inside the last 6 lines:\n%s", got)
	}
	// The list viewport is fully padded: help lines are the last 3, so the
	// gap between the last list row ("Beta") and the bottom block must be
	// listHeight-2 blank rows (listHeight = 24-9 = 15).
	wantGap := 15 - 2
	gap := 0
	for i := total - 7; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) == "" {
			gap++
			continue
		}
		break
	}
	if gap != wantGap {
		t.Fatalf("blank rows between list and bottom block = %d, want %d (list padding)", gap, wantGap)
	}
	if got := strings.Count(out, "Alpha"); got < 1 {
		t.Fatalf("list rows missing:\n%s", out)
	}
}

// S4: the status slot is reserved — with or without a message the View has
// the same line count, so the controls never jump vertically.
func TestViewReservesTheStatusSlotR31(t *testing.T) {
	withStatus := layoutModel("something broke")
	withStatus.width, withStatus.height = 60, 24
	without := layoutModel("")
	without.width, without.height = 60, 24

	a := strings.Split(strings.TrimSuffix(withStatus.View(), "\n"), "\n")
	b := strings.Split(strings.TrimSuffix(without.View(), "\n"), "\n")
	if len(a) != len(b) {
		t.Fatalf("line count changed with status: %d vs %d (slot must be reserved)", len(a), len(b))
	}
	// The reserved line is the 3rd from the bottom: player bar(2) + status(1)
	// + help(2) block, so status sits exactly one help row above the bottom.
	statusAt := len(a) - 3
	if strings.TrimSpace(a[statusAt]) == "" {
		t.Fatalf("status slot did not render the message:\n%q", a[statusAt])
	}
	if strings.TrimSpace(b[statusAt]) != "" {
		t.Fatalf("slot must be blank without a message:\n%q", b[statusAt])
	}
}

// S5: total rendered lines equal the terminal height when it is tall enough
// (alt-screen contract: full-height paint, no dead space at the end).
func TestViewFillsTheWholePanelHeightR31(t *testing.T) {
	m := layoutModel("")
	m.width, m.height = 60, 24
	rendered := strings.Count(strings.TrimSuffix(m.View(), "\n"), "\n") + 1
	if rendered != 24 {
		t.Fatalf("rendered %d lines for a 24-row panel, want full-height paint", rendered)
	}
}

// S3 (issue #37, owner verbatim): both help lines must fit the 52-column
// floating panel WITHOUT truncation ("ambas lineas son más largas que el
// ancho de la ventana"). Bounding the content here means the renderer's
// truncate() never fires on the help block.
func TestHelpLinesFitThePanelWidthR31B(t *testing.T) {
	for i, line := range helpLines() {
		if got := utf8.RuneCountInString(line); got > 52 {
			t.Fatalf("help line %d is %d runes (>52, it truncates on the panel): %q", i, got, line)
		}
	}
}
