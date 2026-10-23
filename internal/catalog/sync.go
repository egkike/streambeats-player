package catalog

// Local library sync: download fresh-resolved mp3-128 streams into a
// per-album layout and scan what is already on disk (incremental sync).

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// SyncResult summarizes one album sync pass.
type SyncResult struct {
	Slug       string   `json:"slug"`
	Files      []string `json:"files"` // newly downloaded files this pass
	Downloaded int      `json:"downloaded"`
	Skipped    int      `json:"skipped"` // tracks without a stream available
}

// DefaultLibraryPath is the user-visible music library location.
func DefaultLibraryPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("user home: %w", err)
	}
	return filepath.Join(home, "Music", "StreamBeats"), nil
}

// SyncAlbum downloads every downloadable track of one album into
// <root>/<NN - album slug>/<NN - track slug>.mp3, skipping files that already
// exist on disk (incremental) and tracks with no stream available.
// index (1-based) preserves the album's order in the catalog.
func (c *Client) SyncAlbum(root string, index int, albumSlug string) (SyncResult, error) {
	result := SyncResult{Slug: albumSlug}
	if root == "" {
		return result, fmt.Errorf("empty library root")
	}
	pageRaw, err := c.FetchAlbumPage(SiteBaseURL + "/album/" + albumSlug)
	if err != nil {
		return result, fmt.Errorf("sync %s: %w", albumSlug, err)
	}
	_, tracks, err := ParseAlbum(pageRaw)
	if err != nil {
		return result, fmt.Errorf("sync %s: %w", albumSlug, err)
	}
	albumDir := filepath.Join(root, fmt.Sprintf("%02d - %s", index, albumSlug))
	if err := os.MkdirAll(albumDir, 0o755); err != nil {
		return result, fmt.Errorf("sync %s: create dir: %w", albumSlug, err)
	}
	for i, t := range tracks {
		rel := fmt.Sprintf("%02d - %s.mp3", i+1, t.Slug)
		dest := filepath.Join(albumDir, rel)
		if _, err := os.Stat(dest); err == nil {
			continue // already on disk: not counted
		}
		url, err := c.ResolveStream(albumSlug, t.Slug)
		if err != nil {
			result.Skipped++ // no stream available right now
			continue
		}
		if err := downloadTo(c.HTTP, url, dest); err != nil {
			return result, fmt.Errorf("sync %s/%s: %w", albumSlug, t.Slug, err)
		}
		result.Downloaded++
		result.Files = append(result.Files, dest)
	}
	return result, nil
}

// downloadTo streams a URL into dest without buffering the whole file.
func downloadTo(client *http.Client, url, dest string) error {
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("fetch: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("fetch: status %d", resp.StatusCode)
	}
	tmp := dest + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("create %s: %w", tmp, err)
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		return fmt.Errorf("download %s: %w", url, err)
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, dest)
}

// ScanLibrary walks the library root and returns every local .mp3 file,
// sorted by path (album order then track order, thanks to the NN - prefix).
func ScanLibrary(root string) ([]string, error) {
	if _, err := os.Stat(root); err != nil {
		return nil, fmt.Errorf("library root %s: %w", root, err)
	}
	var files []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(path), ".mp3") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan %s: %w", root, err)
	}
	return files, nil
}
