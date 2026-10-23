package catalog

import (
	"os"
	"testing"
)

func TestParseMusicPageListsAlbums(t *testing.T) {
	raw, err := os.ReadFile("testdata/music.html")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	albums, err := ParseMusicPage(string(raw))
	if err != nil {
		t.Fatalf("ParseMusicPage: %v", err)
	}
	want := 3
	if len(albums) != want {
		t.Fatalf("got %d albums, want %d", len(albums), want)
	}
	if albums[0].Slug != "electric" {
		t.Errorf("albums[0].Slug = %q, want %q", albums[0].Slug, "electric")
	}
	if albums[0].BaseURL == "" {
		t.Errorf("albums[0].BaseURL is empty, want absolute album URL")
	}
}

func TestParseAlbumExtractsTracks(t *testing.T) {
	raw, err := os.ReadFile("testdata/album.html")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	album, tracks, err := ParseAlbum(string(raw))
	if err != nil {
		t.Fatalf("ParseAlbum: %v", err)
	}
	if album.Title != "Electric" {
		t.Errorf("album.Title = %q, want %q", album.Title, "Electric")
	}
	if album.Artist != "StreamBeats by Harris Heller" {
		t.Errorf("album.Artist = %q, want %q", album.Artist, "StreamBeats by Harris Heller")
	}
	if album.MinimumPrice != 0 {
		t.Errorf("album.MinimumPrice = %v, want 0 (free)", album.MinimumPrice)
	}
	if len(tracks) != 2 {
		t.Fatalf("got %d tracks, want 2", len(tracks))
	}
	first := tracks[0]
	if first.Title != "First Song" {
		t.Errorf("track title = %q, want %q", first.Title, "First Song")
	}
	if first.DurationSeconds != 142.761 {
		t.Errorf("track duration = %v, want 142.761", first.DurationSeconds)
	}
	if first.Slug != "first-song" {
		t.Errorf("track slug = %q, want %q", first.Slug, "first-song")
	}
	if first.StreamURL == "" {
		t.Errorf("track StreamURL is empty, want mp3-128 stream URL")
	}
	if tracks[1].StreamURL != "" {
		t.Errorf("track without stream: StreamURL = %q, want empty", tracks[1].StreamURL)
	}
}
