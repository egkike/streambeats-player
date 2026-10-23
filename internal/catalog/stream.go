package catalog

import (
	"fmt"
)

// ResolveStream returns a fresh, playable mp3-128 stream URL for one track by
// re-fetching the album page (Bandcamp stream tokens expire, so cached URLs
// must never be trusted).
func (c *Client) ResolveStream(albumSlug, trackSlug string) (string, error) {
	pageRaw, err := c.FetchAlbumPage(SiteBaseURL + "/album/" + albumSlug)
	if err != nil {
		return "", fmt.Errorf("resolve stream %s/%s: %w", albumSlug, trackSlug, err)
	}
	_, tracks, err := ParseAlbum(pageRaw)
	if err != nil {
		return "", fmt.Errorf("resolve stream %s/%s: %w", albumSlug, trackSlug, err)
	}
	for _, t := range tracks {
		if t.Slug == trackSlug {
			if t.StreamURL == "" {
				return "", fmt.Errorf("track %s/%s has no stream available", albumSlug, trackSlug)
			}
			return t.StreamURL, nil
		}
	}
	return "", fmt.Errorf("track %q not found in album %q", trackSlug, albumSlug)
}
