package catalog

import (
	"fmt"
	"time"
)

// ResolveStream returns a fresh, playable mp3-128 stream URL for one track by
// re-fetching the album page (Bandcamp stream tokens expire, so cached URLs
// must never be trusted). Transient fetch failures get exactly one retry
// with a short backoff: networks blip, and a full playback sequence should
// not die on a single hiccup.
func (c *Client) ResolveStream(albumSlug, trackSlug string) (string, error) {
	var url string
	var err error
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			time.Sleep(150 * time.Millisecond)
		}
		url, err = c.resolveStreamOnce(albumSlug, trackSlug)
		if err == nil {
			return url, nil
		}
	}
	return "", err
}

func (c *Client) resolveStreamOnce(albumSlug, trackSlug string) (string, error) {
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
