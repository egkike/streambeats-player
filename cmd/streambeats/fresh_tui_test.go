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

	tea "github.com/charmbracelet/bubbletea"

	"streambeats/internal/catalog"
	"streambeats/internal/tui"
)

func fixtureCatalog() *catalog.Catalog {
	return &catalog.Catalog{Albums: []catalog.AlbumEntry{
		{Tracks: []catalog.Track{{}}},
	}}
}

// newTUIDeps builds an isolated dependency set for one test store path.
// The launch stub records what the TUI received: cat (nil on the fresh
// path, issue #75) and the scrape cmd (nil when a cache already exists).
func newFakeTUIDeps(t *testing.T, catalogOut *catalog.Catalog, refreshErr error, launched *int) *tuiDeps {
	t.Helper()
	return &tuiDeps{
		store: &catalog.Store{Path: filepath.Join(t.TempDir(), "catalog.json")},
		refresh: func() (*catalog.Catalog, error) {
			return catalogOut, refreshErr
		},
		launch: func(cat *catalog.Catalog, scrape tea.Cmd) error {
			*launched++
			lastLaunchCat = cat
			lastLaunchScrape = scrape
			return nil
		},
	}
}

// lastLaunchCat/lastLaunchScrape capture what runTUI handed to launch.
var (
	lastLaunchCat    *catalog.Catalog
	lastLaunchScrape tea.Cmd
)

func TestFreshTUILaunchesImmediatelyWithScrapeCmd(t *testing.T) {
	var stdout, stderr bytes.Buffer
	launched := 0
	refreshed := 0
	deps := newFakeTUIDeps(t, fixtureCatalog(), nil, &launched)
	deps.refresh = func() (*catalog.Catalog, error) {
		refreshed++
		return fixtureCatalog(), nil
	}

	code := runTUI(&stdout, &stderr, deps)
	if code != 0 {
		t.Fatalf("fresh-install TUI exit = %d, want 0 (stderr: %s)", code, stderr.String())
	}
	// issue #75: the TUI boots BEFORE the scrape runs — no blocking startup,
	// no pre-TUI terminal message. The scrape only runs inside the TUI loop.
	if launched != 1 {
		t.Fatalf("TUI launch calls = %d, want 1", launched)
	}
	if lastLaunchCat != nil {
		t.Fatal("fresh launch must hand the TUI a nil catalog (populate in place)")
	}
	if lastLaunchScrape == nil {
		t.Fatal("fresh launch must wire the scrape cmd")
	}
	if refreshed != 0 {
		t.Fatalf("scrape ran %d times synchronously; it must only run as a cmd", refreshed)
	}
	if contains(stdout.String(), "scraping it now") {
		t.Fatal("the pre-TUI terminal message is gone (issue #75): the status lives inside the TUI")
	}

	// Running the cmd performs the scrape AND persists the cache (S3).
	msg := lastLaunchScrape()
	scraped, ok := msg.(tui.CatalogScrapedMsg)
	if !ok || scraped.Err != nil || scraped.Cat == nil {
		t.Fatalf("scrape cmd returned %+v, want a populated CatalogScrapedMsg", msg)
	}
	if _, err := os.Stat(deps.store.Path); err != nil {
		t.Fatalf("scrape cmd did not save the cache: %v", err)
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
	if lastLaunchCat == nil {
		t.Fatal("cached launch must hand the TUI the cached catalog")
	}
	if lastLaunchScrape != nil {
		t.Fatal("cached launch must NOT wire a scrape cmd")
	}
}

func TestFreshTUIScrapeFailureIsFriendly(t *testing.T) {
	// issue #75: the scrape no longer runs (or fails) before the TUI boots —
	// a refresh error surfaces inside the TUI as a retryable status. runTUI
	// itself must still boot the TUI with the scrape cmd wired.
	var stdout, stderr bytes.Buffer
	launched := 0
	deps := newFakeTUIDeps(t, nil, errors.New("network unreachable"), &launched)

	code := runTUI(&stdout, &stderr, deps)
	if code != 0 {
		t.Fatalf("fresh TUI must boot even when the scrape will fail (error surfaces in-TUI), got exit %d (stderr: %s)", code, stderr.String())
	}
	if launched != 1 {
		t.Fatalf("launch calls = %d, want 1", launched)
	}
	if lastLaunchScrape == nil {
		t.Fatal("scrape cmd must be wired so the failure lands in the TUI status")
	}
	msg := lastLaunchScrape()
	scraped, ok := msg.(tui.CatalogScrapedMsg)
	if !ok || scraped.Err == nil {
		t.Fatalf("scrape cmd must carry the failure, got %+v", msg)
	}
	if _, err := os.Stat(deps.store.Path); !os.IsNotExist(err) {
		t.Fatal("a failed scrape must not write the cache")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && bytes.Contains([]byte(s), []byte(sub))
}
