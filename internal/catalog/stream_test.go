package catalog

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

// Stream tokens in data-tralbum expire, so a resolver must re-fetch the album
// page and return a fresh mp3-128 URL for the exact track being played.

func TestResolveStreamReturnsFreshTokenizedURL(t *testing.T) {
	albumHTML, err := os.ReadFile(filepath.Join("testdata", "album.html"))
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	c := &Client{
		HTTP: &http.Client{Transport: httpStub{
			"/music":          `<a href="/album/electric">Electric</a>`,
			"/album/electric": string(albumHTML),
			"/album/inu":      "broken",
		}},
		BaseURL: ptr("http://stub"),
	}
	got, err := c.ResolveStream("electric", "first-song")
	if err != nil {
		t.Fatalf("ResolveStream: %v", err)
	}
	want := "https://t4.bcbits.com/stream/TOKEN/mp3-128/1"
	if got != want {
		t.Errorf("ResolveStream = %q, want %q", got, want)
	}
}

func TestResolveStreamUnknownTrackErrors(t *testing.T) {
	albumHTML, _ := os.ReadFile(filepath.Join("testdata", "album.html"))
	c := &Client{
		HTTP: &http.Client{Transport: httpStub{
			"/album/electric": string(albumHTML),
		}},
		BaseURL: ptr("http://stub"),
	}
	if _, err := c.ResolveStream("electric", "nope-song"); err == nil {
		t.Fatal("unknown track should error, not return empty URL")
	}
}

func TestResolveStreamVanishedAlbumErrors(t *testing.T) {
	c := &Client{
		HTTP:    &http.Client{Transport: httpStub{}},
		BaseURL: ptr("http://stub"),
	}
	if _, err := c.ResolveStream("electric", "first-song"); err == nil {
		t.Fatal("missing album page should error")
	}
}
