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

// Flaky network: the first fetch fails with 503, the retry succeeds.
// ResolveStream must self-heal with one retry instead of failing playback.
func TestResolveStreamRetriesTransientFetchFailure(t *testing.T) {
	albumHTML, _ := os.ReadFile(filepath.Join("testdata", "album.html"))
	calls := 0
	c := &Client{
		HTTP: &http.Client{Transport: &flakyStub{
			failFirst: func(req *http.Request) bool { calls++; return calls <= 1 },
			albums:    httpStub{"/album/electric": string(albumHTML)},
		}},
		BaseURL: ptr("http://stub"),
	}
	got, err := c.ResolveStream("electric", "first-song")
	if err != nil {
		t.Fatalf("ResolveStream should recover on retry: %v", err)
	}
	if got == "" {
		t.Fatal("empty url after retry")
	}
}

func TestResolveStreamAllFailsStillErrors(t *testing.T) {
	c := &Client{
		HTTP: &http.Client{Transport: &flakyStub{
			failFirst: func(req *http.Request) bool { return true },
			albums:    httpStub{},
		}},
		BaseURL: ptr("http://stub"),
	}
	if _, err := c.ResolveStream("electric", "first-song"); err == nil {
		t.Fatal("resolve must fail when both attempts fail")
	}
}

// flakyStub fails the first attempts, then serves from albums.
type flakyStub struct {
	failFirst func(*http.Request) bool
	albums    httpStub
}

func (s *flakyStub) RoundTrip(req *http.Request) (*http.Response, error) {
	if s.failFirst(req) {
		return newResponse(503, ""), nil
	}
	if resp, ok := s.albums.RoundTripResult(req); ok {
		return resp, nil
	}
	return newResponse(404, ""), nil
}
