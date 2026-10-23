package main

// Unit tests for CLI argument parsing and catalog counting. Nothing here
// touches the network, the filesystem cache, or mpv.

import (
	"bytes"
	"strings"
	"testing"

	"streambeats/internal/catalog"
)

func TestParseCommand(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    command
		wantErr bool
	}{
		{"no args launches tui", nil, cmdTUI, false},
		{"explicit tui", []string{"tui"}, cmdTUI, false},
		{"refresh", []string{"refresh"}, cmdRefresh, false},
		{"sync", []string{"sync"}, cmdSync, false},
		{"help", []string{"help"}, cmdHelp, false},
		{"dash h", []string{"-h"}, cmdHelp, false},
		{"double dash help", []string{"--help"}, cmdHelp, false},
		{"unknown", []string{"bogus"}, cmdTUI, true},
		{"too many args", []string{"refresh", "sync"}, cmdTUI, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseCommand(tc.args)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseCommand(%v) = %v, want an error", tc.args, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseCommand(%v) unexpected error: %v", tc.args, err)
			}
			if got != tc.want {
				t.Fatalf("parseCommand(%v) = %v, want %v", tc.args, got, tc.want)
			}
		})
	}
}

func TestHelpExitCodeAndUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"--help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("help exit code = %d, want 0 (stderr: %s)", code, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{"streambeats", "refresh", "sync", "j/k", "space"} {
		if !strings.Contains(out, want) {
			t.Fatalf("usage output is missing %q:\n%s", want, out)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("help wrote to stderr: %s", stderr.String())
	}
}

func TestUnknownCommandExitCode(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"bogus"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("unknown command exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "unknown command") {
		t.Fatalf("stderr = %q, want an unknown command message", stderr.String())
	}
}

func TestTotalTracks(t *testing.T) {
	tests := []struct {
		name string
		cat  *catalog.Catalog
		want int
	}{
		{"nil catalog", nil, 0},
		{"empty catalog", &catalog.Catalog{}, 0},
		{
			name: "sums every album",
			cat: &catalog.Catalog{Albums: []catalog.AlbumEntry{
				{Tracks: []catalog.Track{{}, {}}},
				{Tracks: []catalog.Track{{}}},
			}},
			want: 3,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := totalTracks(tc.cat); got != tc.want {
				t.Fatalf("totalTracks(%v) = %d, want %d", tc.cat, got, tc.want)
			}
		})
	}
}
