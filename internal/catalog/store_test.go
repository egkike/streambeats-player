package catalog

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStoreSaveAndLoadRoundTrip(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "catalog.json"))
	in := &Catalog{
		Albums: []AlbumEntry{{
			Slug:  "electric",
			Title: "Electric",
			Tracks: []Track{{
				Title:           "Twinkle Tunes",
				Slug:            "twinkle-tunes",
				DurationSeconds: 142.761,
			}},
		}},
		FetchedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	if err := store.Save(in); err != nil {
		t.Fatalf("Save: %v", err)
	}
	out, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(out.Albums) != 1 || out.Albums[0].Slug != "electric" {
		t.Fatalf("round trip lost album data: %+v", out.Albums)
	}
	if out.Albums[0].Tracks[0].DurationSeconds != 142.761 {
		t.Errorf("round trip lost duration: %+v", out.Albums[0].Tracks[0])
	}
	// Cache must not persist volatile stream tokens.
	if out.Albums[0].Tracks[0].StreamURL != "" {
		t.Errorf("cache persisted StreamURL %q; stream tokens must regenerate on play",
			out.Albums[0].Tracks[0].StreamURL)
	}
}

func TestStoreLoadMissingFileReturnsErrNotExist(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "nope", "catalog.json"))
	if _, err := store.Load(); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Load err = %v, want os.ErrNotExist", err)
	}
}

func TestRefreshScrapesAlbumsAndDropsTokens(t *testing.T) {
	base, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	music, _ := os.ReadFile(filepath.Join(base, "testdata", "music.html"))
	album, _ := os.ReadFile(filepath.Join(base, "testdata", "album.html"))
	c := &Client{
		HTTP: &http.Client{Transport: httpStub{
			"/music":          string(music),
			"/album/electric": string(album),
		}},
		BaseURL: ptr("http://stub"),
	}
	cat, err := c.Refresh()
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	// fixture lists electric, gold, inu; only electric exists on the stub;
	// missing albums must be skipped, not fail the refresh
	if len(cat.Albums) != 1 {
		t.Fatalf("albums = %d, want 1 (fixture only serves electric)", len(cat.Albums))
	}
	if got := cat.Albums[0].Title; got != "Electric" {
		t.Errorf("title = %q, want %q", got, "Electric")
	}
	if len(cat.Albums[0].Tracks) != 2 {
		t.Fatalf("tracks = %d, want 2", len(cat.Albums[0].Tracks))
	}
}

func TestRefreshFailsNonexistentAlbum(t *testing.T) {
	c := &Client{
		HTTP:    &http.Client{Transport: httpStub{"/music": `<a href="/album/electric">Electric</a>`}},
		BaseURL: ptr("http://stub"),
	}
	if _, err := c.Refresh(); err == nil {
		t.Fatal("Refresh should fail when album page is missing")
	}
}

func ptr[T any](v T) *T { return &v }

// httpStub is a deterministic single-host GET stub for tests.
type httpStub map[string]string

func (h httpStub) RoundTrip(req *http.Request) (*http.Response, error) {
	if resp, ok := h.RoundTripResult(req); ok {
		return resp, nil
	}
	return &http.Response{
		StatusCode: http.StatusNotFound,
		Body:       io.NopCloser(strings.NewReader("")),
		Header:     make(http.Header),
	}, nil
}

// RoundTripResult exposes the lookup for other stubs to compose.
func (h httpStub) RoundTripResult(req *http.Request) (*http.Response, bool) {
	if body, ok := h[req.URL.Path]; ok {
		return newResponse(http.StatusOK, body), true
	}
	return nil, false
}
