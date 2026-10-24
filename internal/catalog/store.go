package catalog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Store persists and reloads the catalog cache as JSON.
type Store struct{ Path string }

// NewStore returns a Store at the given cache path.
func NewStore(path string) *Store { return &Store{Path: path} }

// DefaultStore opens a Store at the platform cache location.
func DefaultStore() (*Store, error) {
	p, err := CachePath()
	if err != nil {
		return nil, err
	}
	return NewStore(p), nil
}

// Load returns the cached catalog; os.ErrNotExist surfaces as (:nil, err).
func (s *Store) Load() (*Catalog, error) {
	raw, err := os.ReadFile(s.Path)
	if err != nil {
		return nil, err
	}
	var cat Catalog
	if err := json.Unmarshal(raw, &cat); err != nil {
		return nil, fmt.Errorf("decode %s: %w", s.Path, err)
	}
	return &cat, nil
}

// Save atomically writes the catalog cache (write .tmp then rename).
func (s *Store) Save(cat *Catalog) error {
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o750); err != nil {
		return fmt.Errorf("mkdir cache dir: %w", err)
	}
	raw, err := json.MarshalIndent(cat, "", "  ")
	if err != nil {
		return fmt.Errorf("encode catalog: %w", err)
	}
	tmp := s.Path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return fmt.Errorf("write cache: %w", err)
	}
	if err := os.Rename(tmp, s.Path); err != nil {
		return fmt.Errorf("commit cache: %w", err)
	}
	return nil
}

// Refresh scrapes the full catalog: the /music landing page, then every album
// page for its track list. Albums that vanish (404) are skipped instead of
// failing the whole refresh; stale stream tokens are dropped (regenerate on
// play). It fails only when zero albums could be scraped.
func (c *Client) Refresh() (*Catalog, error) {
	musicRaw, err := c.FetchMusicPage()
	if err != nil {
		return nil, err
	}
	albums, err := ParseMusicPage(musicRaw)
	if err != nil {
		return nil, err
	}
	cat := &Catalog{SiteBaseURL: SiteBaseURL}
	for _, album := range albums {
		pageRaw, err := c.FetchAlbumPage(album.BaseURL)
		if err != nil {
			continue // vanished album: skip
		}
		detail, tracks, err := ParseAlbum(pageRaw)
		if err != nil {
			continue // malformed page: skip
		}
		clean := make([]Track, len(tracks))
		copy(clean, tracks)
		for i := range clean {
			clean[i].StreamURL = ""
		}
		cat.Albums = append(cat.Albums, AlbumEntry{
			Slug:   album.Slug,
			Title:  detail.Title,
			Artist: detail.Artist,
			Tracks: clean,
		})
	}
	cat.FetchedAt = time.Now().UTC()
	if len(cat.Albums) == 0 {
		return nil, fmt.Errorf("catalog empty: %d listed albums all failed to scrape", len(albums))
	}
	return cat, nil
}
