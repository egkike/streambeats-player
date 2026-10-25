package main

// Tests for the fresh-install TUI fallback (issue #17): with no catalog
// cache, the TUI path scrapes live (like runSync) instead of erroring.
// The catalog client is stubbed (no network); launch is injected; the
// store is a real one on a t.TempDir path.

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"streambeats/internal/catalog"
)

func fixtureCatalog() *catalog.Catalog {
	return &catalog.Catalog{Albums: []catalog.AlbumEntry{
		{Tracks: []catalog.Track{{}}},
	}}
}

// newTUIDeps builds an isolated dependency set for one test store path.
func newFakeTUIDeps(t *testing.T, catalogOut *catalog.Catalog, refreshErr error, launched *int) *tuiDeps {
	t.Helper()
	return &tuiDeps{
		store: &catalog.Store{Path: filepath.Join(t.TempDir(), "catalog.json")},
		refresh: func() (*catalog.Catalog, error) {
			return catalogOut, refreshErr
		},
		launch: func(cat *catalog.Catalog) error {
			*launched++
			return nil
		},
	}
}

func TestFreshTUIScrapesWhenCacheMissing(t *testing.T) {
	var stdout, stderr bytes.Buffer
	launched := 0
	deps := newFakeTUIDeps(t, fixtureCatalog(), nil, &launched)

	code := runTUI(&stdout, &stderr, deps)
	if code != 0 {
		t.Fatalf("fresh-install TUI exit = %d, want 0 (stderr: %s)", code, stderr.String())
	}
	if !contains(stdout.String(), "scraping it now") {
		t.Fatalf("stdout missing the friendly scrape note:\n%s", stdout.String())
	}
	if _, err := os.Stat(deps.store.Path); err != nil {
		t.Fatalf("scraped catalog was not saved to the cache: %v", err)
	}
	if launched != 1 {
		t.Fatalf("TUI launch calls = %d, want 1", launched)
	}
}

func TestTUIWithCacheSkipsScrape(t *testing.T) {
	var stdout, stderr bytes.Buffer
	launched := 0
	deps := newFakeTUIDeps(t, fixtureCatalog(), nil, &launched)
	if err := deps.store.Save(fixtureCatalog()); err != nil {
		t.Fatal(err)
	}

	code := runTUI(&stdout, &stderr, deps)
	if code != 0 {
		t.Fatalf("cached TUI exit = %d, want 0 (stderr: %s)", code, stderr.String())
	}
	if contains(stdout.String(), "scraping") {
		t.Fatal("cached TUI must not re-scrape")
	}
	if launched != 1 {
		t.Fatalf("launch calls = %d, want 1", launched)
	}
}

func TestFreshTUIScrapeFailureIsFriendly(t *testing.T) {
	var stdout, stderr bytes.Buffer
	launched := 0
	deps := newFakeTUIDeps(t, nil, errors.New("network unreachable"), &launched)

	code := runTUI(&stdout, &stderr, deps)
	if code == 0 {
		t.Fatal("refresh failure must keep the TUI out")
	}
	out := stderr.String()
	if !contains(out, "refresh catalog") || contains(out, "goroutine") {
		t.Fatalf("stderr not friendly:\n%s", out)
	}
	if launched != 0 {
		t.Fatal("TUI must not launch on refresh failure")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && bytes.Contains([]byte(s), []byte(sub))
}
