package tui

// Unit tests for the pure rendering helpers (no terminal involved).

import (
	"math"
	"strings"
	"testing"
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
	for _, want := range []string{"j/down", "k/up", "enter", "esc", "l", "space", "s", "+", "-", "q"} {
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
