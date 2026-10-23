package catalog

// Local library sync: download resolved streams into a per-album layout
// and scan what is already downloaded (so sync is incremental).

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A synced library must look like:
//   <root>/<NN - album slug>/<NN - track slug>.mp3
// and ScanLibrary must return every existing local track file.

func TestSyncAlbumDownloadsTrackFiles(t *testing.T) {
	root := t.TempDir()
	albumHTML, _ := os.ReadFile(filepath.Join("testdata", "album.html"))
	c := &Client{
		HTTP: &http.Client{Transport: &streamStub{
			albums: httpStub{"/album/electric": string(albumHTML)},
			audio:  map[string]string{"1": "fake-audio-bytes"},
		}},
		BaseURL: ptr("http://stub"),
	}
	got, err := c.SyncAlbum(root, 1, "electric")
	if err != nil {
		t.Fatalf("SyncAlbum: %v", err)
	}
	if len(got.Files) != 1 {
		t.Fatalf("downloaded %d files, want 1", len(got.Files))
	}
	wantDir := filepath.Join(root, "01 - electric")
	if filepath.Dir(got.Files[0]) != wantDir {
		t.Errorf("file dir = %q, want %q", filepath.Dir(got.Files[0]), wantDir)
	}
	if filepath.Ext(got.Files[0]) != ".mp3" {
		t.Errorf("file = %q, want .mp3", got.Files[0])
	}
	data, err := os.ReadFile(got.Files[0])
	if err != nil {
		t.Fatalf("read downloaded: %v", err)
	}
	if string(data) != "fake-audio-bytes" {
		t.Errorf("content = %q", data)
	}
	// tracks with no stream must be skipped, not fail the sync
	if got.Skipped != 1 {
		t.Errorf("skipped = %d, want 1 (Second Song has no stream)", got.Skipped)
	}
}

func TestSyncAlbumIsIncremental(t *testing.T) {
	root := t.TempDir()
	albumHTML, _ := os.ReadFile(filepath.Join("testdata", "album.html"))
	c := &Client{
		HTTP: &http.Client{Transport: &streamStub{
			albums: httpStub{"/album/electric": string(albumHTML)},
			audio:  map[string]string{"1": "fake-audio-bytes"},
		}},
		BaseURL: ptr("http://stub"),
	}
	if _, err := c.SyncAlbum(root, 1, "electric"); err != nil {
		t.Fatalf("first SyncAlbum: %v", err)
	}
	second, err := c.SyncAlbum(root, 1, "electric")
	if err != nil {
		t.Fatalf("second SyncAlbum: %v", err)
	}
	if len(second.Files) != 0 {
		t.Errorf("re-sync downloaded %d files again, want 0", len(second.Files))
	}
	if second.Downloaded != 0 || second.Skipped != 1 {
		t.Errorf("second pass summary = %+v", second)
	}
}

func TestScanLibraryReturnsExistingFiles(t *testing.T) {
	root := t.TempDir()
	album := filepath.Join(root, "01 - electric")
	if err := os.MkdirAll(album, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(album, "01 - first-song.mp3"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(album, "cover.jpg"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	files, err := ScanLibrary(root)
	if err != nil {
		t.Fatalf("ScanLibrary: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("found %d files, want 1 (only .mp3)", len(files))
	}
	if filepath.Base(files[0]) != "01 - first-song.mp3" {
		t.Errorf("found %q", files[0])
	}
}

func TestScanLibraryMissingRootErrors(t *testing.T) {
	if _, err := ScanLibrary(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Fatal("missing root should error")
	}
}

func TestSyncRejectsEmptyRoot(t *testing.T) {
	c := NewClient()
	if _, err := c.SyncAlbum("", 1, "electric"); err == nil {
		t.Fatal("empty library root must be rejected")
	}
}

// streamStub serves album pages and track audio keyed by URL basename.
type streamStub struct {
	albums httpStub
	audio  map[string]string
}

func (s *streamStub) RoundTrip(req *http.Request) (*http.Response, error) {
	if resp, ok := s.albums.RoundTripResult(req); ok {
		return resp, nil
	}
	base := filepath.Base(req.URL.Path)
	if body, ok := s.audio[base]; ok {
		return newResponse(http.StatusOK, body), nil
	}
	return newResponse(http.StatusNotFound, ""), nil
}

func newResponse(code int, body string) *http.Response {
	return &http.Response{
		StatusCode: code,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}
