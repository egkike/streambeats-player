package tui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"streambeats/internal/catalog"
)

// Scrape-cycle scenarios (issue #75): the TUI boots immediately on a fresh
// cache and shows a live-scrape status; the list populates in place when
// the scrape completes; failures keep the app alive with an `r` retry.

func scrapeFixtureCatalog() *catalog.Catalog {
	return &catalog.Catalog{Albums: []catalog.AlbumEntry{
		{Title: "Album Uno", Tracks: []catalog.Track{{Title: "t1"}}},
		{Title: "Album Dos", Tracks: []catalog.Track{{Title: "t2"}}},
	}}
}

func TestFreshModelShowsScrapingStatus(t *testing.T) {
	scrapeCalled := 0
	scrapeCmd := tea.Cmd(func() tea.Msg { scrapeCalled++; return nil })
	m := NewModel(Options{ScrapeCmd: scrapeCmd, Player: &fakePlayer{}})

	if len(m.albums) != 0 {
		t.Fatalf("fresh model must boot with an empty list, got %d albums", len(m.albums))
	}
	if !strings.Contains(m.status, "scraping catalog") {
		t.Fatalf("status = %q, want the live-scrape notice", m.status)
	}
	if strings.Contains(m.status, "catalog is empty") {
		t.Fatalf("stale empty-catalog note must not show while scraping: %q", m.status)
	}
	// the scrape itself must NOT have run synchronously at construction
	if scrapeCalled != 0 {
		t.Fatalf("scrape ran %d times during NewModel; it must only run as a cmd", scrapeCalled)
	}
}

func TestCatalogScrapedMsgPopulatesList(t *testing.T) {
	scrapeCmd := tea.Cmd(func() tea.Msg { return nil })
	m := NewModel(Options{ScrapeCmd: scrapeCmd, Player: &fakePlayer{}})

	m, _ = update(t, m, CatalogScrapedMsg{Cat: scrapeFixtureCatalog()})

	if len(m.albums) != 2 {
		t.Fatalf("albums after scrape = %d, want 2", len(m.albums))
	}
	if strings.Contains(m.status, "scraping") {
		t.Fatalf("status must return to normal after a successful scrape: %q", m.status)
	}
	if m.scrapeCmd != nil {
		t.Fatal("scrape cmd must be cleared after success")
	}
}

func TestScrapeErrorKeepsAppAliveWithRetry(t *testing.T) {
	m := NewModel(Options{
		ScrapeCmd: tea.Cmd(func() tea.Msg { return CatalogScrapedMsg{Err: errors.New("bandcamp down")} }),
		Player:    &fakePlayer{},
	})
	m, _ = update(t, m, CatalogScrapedMsg{Err: errors.New("bandcamp down")})

	if !strings.Contains(m.status, "scrape failed") {
		t.Fatalf("status = %q, want the failure notice", m.status)
	}
	if !strings.Contains(m.status, "r") {
		t.Fatalf("status = %q, must name the retry key", m.status)
	}
	if m.scrapeCmd == nil {
		t.Fatal("failed scrape must keep the retry cmd for `r`")
	}
	if len(m.albums) != 0 {
		t.Fatalf("a failed scrape must not populate the list, got %d albums", len(m.albums))
	}

	// `r` re-issues the scrape: the returned cmd must be non-nil and the
	// status goes back to scraping.
	var retryCmd tea.Cmd
	m, retryCmd = press(t, m, 'r')
	if retryCmd == nil {
		t.Fatal("`r` must return the scrape cmd so the retry actually runs")
	}
	if !strings.Contains(m.status, "scraping catalog") {
		t.Fatalf("status after retry = %q, want the scraping notice again", m.status)
	}
}
