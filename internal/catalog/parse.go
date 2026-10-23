package catalog

// Parsing of StreamBeats Bandcamp pages. Album pages embed a data-tralbum
// attribute whose escaped JSON payload carries the full track list and
// per-track mp3-128 stream URLs (tokenized by Bandcamp; regenerate per play).

import (
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strings"
)

const SiteBaseURL = "https://streambeats.bandcamp.com"

// Album is one StreamBeats release listed on the /music page.
type Album struct {
	Slug    string `json:"slug"`
	Title   string `json:"title"`
	BaseURL string `json:"base_url"`
}

// Track is one song inside an album, as parsed from data-tralbum.
type Track struct {
	Title           string  `json:"title"`
	Slug            string  `json:"slug"`
	DurationSeconds float64 `json:"duration_seconds"`
	StreamURL       string  `json:"stream_url"` // may be empty; tokenized, regenerable
}

// AlbumDetail is the metadata of a single album page.
type AlbumDetail struct {
	Title        string  `json:"title"`
	Artist       string  `json:"artist"`
	MinimumPrice float64 `json:"minimum_price"`
}

var albumSlugPattern = regexp.MustCompile(`href="/album/([a-z0-9-]+)"`)
var tralbumPattern = regexp.MustCompile(`data-tralbum="([^"]*)"`)
var trackSlugPattern = regexp.MustCompile(`^/track/([a-z0-9-]+)$`)

// ParseMusicPage extracts the album list from the /music landing page.
func ParseMusicPage(raw string) ([]Album, error) {
	seen := map[string]bool{}
	var albums []Album
	for _, m := range albumSlugPattern.FindAllStringSubmatch(raw, -1) {
		slug := m[1]
		if seen[slug] {
			continue
		}
		seen[slug] = true
		albums = append(albums, Album{
			Slug:    slug,
			BaseURL: SiteBaseURL + "/album/" + slug,
		})
	}
	if len(albums) == 0 {
		return nil, fmt.Errorf("no albums found on /music page")
	}
	return albums, nil
}

// ParseAlbum extracts album metadata and its tracks from an album page.
func ParseAlbum(raw string) (AlbumDetail, []Track, error) {
	m := tralbumPattern.FindStringSubmatch(raw)
	if m == nil {
		return AlbumDetail{}, nil, fmt.Errorf("no data-tralbum attribute found")
	}
	var payload struct {
		Current struct {
			Title        string   `json:"title"`
			MinimumPrice *float64 `json:"minimum_price"`
		} `json:"current"`
		Artist    string `json:"artist"`
		TrackInfo []struct {
			Title     string             `json:"title"`
			Duration  float64            `json:"duration"`
			TitleLink string             `json:"title_link"`
			File      map[string]*string `json:"file"`
		} `json:"trackinfo"`
	}
	if err := json.Unmarshal([]byte(html.UnescapeString(m[1])), &payload); err != nil {
		return AlbumDetail{}, nil, fmt.Errorf("decode data-tralbum: %w", err)
	}

	detail := AlbumDetail{
		Title:  payload.Current.Title,
		Artist: payload.Artist,
	}
	if payload.Current.MinimumPrice != nil {
		detail.MinimumPrice = *payload.Current.MinimumPrice
	}

	tracks := make([]Track, 0, len(payload.TrackInfo))
	for _, ti := range payload.TrackInfo {
		track := Track{
			Title:           ti.Title,
			DurationSeconds: ti.Duration,
			Slug:            slugFromTrackLink(ti.TitleLink),
		}
		if u, ok := ti.File["mp3-128"]; ok && u != nil {
			track.StreamURL = resolveStreamURL(*u)
		}
		tracks = append(tracks, track)
	}
	return detail, tracks, nil
}

func slugFromTrackLink(link string) string {
	m := trackSlugPattern.FindStringSubmatch(strings.TrimSpace(link))
	if m == nil {
		return ""
	}
	return m[1]
}

// resolveStreamURL turns the Bandcamp protocol-relative stream href into an
// absolute URL safe to hand to a player.
func resolveStreamURL(href string) string {
	if href == "" {
		return ""
	}
	if u, err := url.Parse(href); err == nil {
		if u.Scheme == "" && u.Host != "" {
			return "https:" + href
		}
	}
	return href
}
