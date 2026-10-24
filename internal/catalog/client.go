package catalog

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

// Client fetches StreamBeats Bandcamp pages over HTTP.
type Client struct {
	BaseURL *string // nil = default; test override
	HTTP    *http.Client
}

// NewClient returns a client with the default Bandcamp base URL.
func NewClient() *Client {
	return &Client{HTTP: &http.Client{Timeout: 30 * time.Second}}
}

func (c *Client) get(path string) (string, error) {
	base := SiteBaseURL
	if c.BaseURL != nil {
		base = *c.BaseURL
	}
	req, err := http.NewRequestWithContext(context.Background(), "GET", base+path, nil)
	if err != nil {
		return "", fmt.Errorf("build request %s: %w", path, err)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", fmt.Errorf("get %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("get %s: not found (404)", path)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("get %s: unexpected status %d", path, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	return string(body), nil
}

// FetchMusicPage returns the raw /music landing page HTML.
func (c *Client) FetchMusicPage() (string, error) {
	return c.get("/music")
}

// FetchAlbumPage returns the raw HTML of one album page. It accepts absolute
// URLs (from the /music listing) and site-relative paths alike.
func (c *Client) FetchAlbumPage(albumURL string) (string, error) {
	u, err := url.Parse(albumURL)
	if err != nil || u.Path == "" {
		return "", fmt.Errorf("invalid album URL %q: %w", albumURL, err)
	}
	return c.get(u.Path)
}

// AlbumEntry is a catalog cache record for one album with its tracks.
type AlbumEntry struct {
	Slug   string  `json:"slug"`
	Title  string  `json:"title"`
	Artist string  `json:"artist"`
	Tracks []Track `json:"tracks"` // StreamURL excluded from cache
}

// Catalog is the persisted listing of every album and its tracks.
type Catalog struct {
	Albums      []AlbumEntry `json:"albums"`
	FetchedAt   time.Time    `json:"fetched_at"`
	SiteBaseURL string       `json:"site_base_url"`
}

// CachePath returns the default catalog cache location
// ($XDG_CACHE_HOME/streambeats/catalog.json).
func CachePath() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("user cache dir: %w", err)
	}
	return filepath.Join(dir, "streambeats", "catalog.json"), nil
}
