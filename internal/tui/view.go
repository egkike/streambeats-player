package tui

// Rendering plus the small pure helpers it depends on: index clamping,
// duration/progress formatting, list windowing and title derivation. All of
// these are unit tested without a terminal.

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"streambeats/internal/catalog"
)

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	playingStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	statusStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	barStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
)

const (
	minListHeight = 3
	defaultWidth  = 80
	// chromeLines is vertical space used by title, header, player bar,
	// status and help lines.
	chromeLines = 9
)

// View renders the whole screen. It must stay side-effect free.
func (m Model) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render("streambeats"))
	b.WriteString(dimStyle.Render("  terminal music player"))
	b.WriteString("\n\n")

	b.WriteString(m.headerView())
	b.WriteString("\n")
	b.WriteString(m.listView())
	b.WriteString("\n")
	b.WriteString(m.playerBar())
	b.WriteString("\n")
	if m.status != "" {
		b.WriteString(statusStyle.Render("! " + m.status))
		b.WriteString("\n")
	}
	for _, line := range helpLines() {
		b.WriteString(dimStyle.Render(truncate(line, m.contentWidth())))
		b.WriteString("\n")
	}
	return b.String()
}

// headerView names the focused list.
func (m Model) headerView() string {
	switch {
	case m.mode == ModeLibrary:
		root := m.libraryRoot
		if root == "" {
			root = "local library"
		}
		return titleStyle.Render(fmt.Sprintf("Local library (%d files)", len(m.library))) +
			dimStyle.Render("  "+root)
	case m.inTracks:
		album, ok := m.selectedAlbum()
		if !ok {
			return titleStyle.Render("Tracks")
		}
		label := album.Title
		if album.Artist != "" {
			label = album.Artist + " — " + label
		}
		return titleStyle.Render(fmt.Sprintf("%s (%d tracks)", label, len(album.Tracks))) +
			dimStyle.Render("  esc: back")
	default:
		return titleStyle.Render(fmt.Sprintf("Albums (%d)", len(m.albums)))
	}
}

// listView renders the visible slice of the focused list.
func (m Model) listView() string {
	height := m.listHeight()
	width := m.contentWidth()

	switch {
	case m.mode == ModeLibrary:
		start, end := windowBounds(len(m.library), m.libraryCursor, height)
		if len(m.library) == 0 {
			return dimStyle.Render("  no local files yet — run `streambeats sync`") + "\n"
		}
		lines := make([]string, 0, end-start)
		for i := start; i < end; i++ {
			path := m.library[i]
			title := libraryEntryTitle(path)
			lines = append(lines, m.renderRow(i == clampCursor(m.libraryCursor, len(m.library)), m.nowPlaying == title, title, width))
		}
		return strings.Join(lines, "\n") + "\n"

	case m.inTracks:
		album, ok := m.selectedAlbum()
		if !ok {
			return dimStyle.Render("  no album selected") + "\n"
		}
		start, end := windowBounds(len(album.Tracks), m.trackCursor, height)
		if len(album.Tracks) == 0 {
			return dimStyle.Render("  album has no tracks") + "\n"
		}
		lines := make([]string, 0, end-start)
		for i := start; i < end; i++ {
			track := album.Tracks[i]
			label := fmt.Sprintf("%02d  %s", i+1, trackTitle(track))
			if track.DurationSeconds > 0 {
				label += "  " + formatDuration(track.DurationSeconds)
			}
			playing := m.nowPlaying != "" && m.nowPlaying == trackTitle(track)
			lines = append(lines, m.renderRow(i == clampCursor(m.trackCursor, len(album.Tracks)), playing, label, width))
		}
		return strings.Join(lines, "\n") + "\n"

	default:
		if len(m.albums) == 0 {
			return dimStyle.Render("  no albums cached — run `streambeats refresh`") + "\n"
		}
		start, end := windowBounds(len(m.albums), m.albumCursor, height)
		lines := make([]string, 0, end-start)
		for i := start; i < end; i++ {
			album := m.albums[i]
			label := fmt.Sprintf("%02d  %s", i+1, album.Title)
			if album.Artist != "" {
				label += "  " + album.Artist
			}
			label += fmt.Sprintf("  (%d tracks)", len(album.Tracks))
			lines = append(lines, m.renderRow(i == clampCursor(m.albumCursor, len(m.albums)), false, label, width))
		}
		return strings.Join(lines, "\n") + "\n"
	}
}

// renderRow draws one list row with cursor/playing markers.
func (m Model) renderRow(selected, playing bool, label string, width int) string {
	marker := "  "
	if selected {
		marker = "▸ "
	}
	if playing {
		label = "♪ " + label
	}
	line := truncate(marker+label, width)
	switch {
	case selected:
		return selectedStyle.Render(line)
	case playing:
		return playingStyle.Render(line)
	default:
		return line
	}
}

// playerBar renders the now-playing line and progress.
func (m Model) playerBar() string {
	width := m.contentWidth()

	glyph := "■"
	switch {
	case m.playing && m.paused:
		glyph = "⏸"
	case m.playing:
		glyph = "▶"
	}
	title := m.nowPlaying
	if title == "" {
		title = "nothing playing — press enter on a track"
	}
	line := fmt.Sprintf("%s  %s", glyph, title)
	line += dimStyle.Render(fmt.Sprintf("   vol %d%%", m.volume))
	line = truncate(line, width)

	barWidth := width - 20
	if barWidth < 10 {
		barWidth = 10
	}
	progress := progressBar(m.position, m.duration, barWidth)
	times := fmt.Sprintf("%s / %s", formatDuration(m.position), formatDuration(m.duration))
	return line + "\n" + barStyle.Render(progress) + " " + dimStyle.Render(times)
}

// listHeight is how many rows fit below the header.
func (m Model) listHeight() int {
	if m.height <= 0 {
		return defaultListHeight
	}
	h := m.height - chromeLines
	if h < minListHeight {
		return minListHeight
	}
	return h
}

// contentWidth is the usable line width.
func (m Model) contentWidth() int {
	if m.width <= 0 {
		return defaultWidth
	}
	return m.width
}

// --- pure helpers ------------------------------------------------------

// clampCursor clamps a list cursor to [0, n-1]; empty lists yield 0.
func clampCursor(cursor, n int) int {
	if n <= 0 {
		return 0
	}
	if cursor < 0 {
		return 0
	}
	if cursor >= n {
		return n - 1
	}
	return cursor
}

// clampVolume clamps a volume percentage to [0, 100].
func clampVolume(volume int) int {
	if volume < 0 {
		return 0
	}
	if volume > 100 {
		return 100
	}
	return volume
}

// windowBounds returns the visible [start, end) slice for a cursor inside a
// list of n items shown in height rows.
func windowBounds(n, cursor, height int) (int, int) {
	if n <= 0 || height <= 0 {
		return 0, 0
	}
	if n <= height {
		return 0, n
	}
	start := cursor - height/2
	if start < 0 {
		start = 0
	}
	if start > n-height {
		start = n - height
	}
	return start, start + height
}

// formatDuration renders seconds as m:ss (or h:mm:ss above one hour).
func formatDuration(seconds float64) string {
	if seconds < 0 || seconds != seconds { // negative or NaN
		seconds = 0
	}
	total := int(seconds)
	h := total / 3600
	mins := (total % 3600) / 60
	secs := total % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, mins, secs)
	}
	return fmt.Sprintf("%d:%02d", mins, secs)
}

// progressBar renders a filled/empty bar of the given width.
func progressBar(position, duration float64, width int) string {
	if width <= 0 {
		return ""
	}
	if duration <= 0 || duration != duration || position < 0 {
		return strings.Repeat("░", width)
	}
	ratio := position / duration
	if ratio > 1 {
		ratio = 1
	}
	filled := int(ratio * float64(width))
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

// truncate cuts a line to at most width runes.
func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= width {
		return s
	}
	if width == 1 {
		return "…"
	}
	return string(runes[:width-1]) + "…"
}

// trackTitle is the display title of a catalog track.
func trackTitle(track catalog.Track) string {
	if strings.TrimSpace(track.Title) != "" {
		return track.Title
	}
	if track.Slug != "" {
		return humanizeSlug(track.Slug)
	}
	return "unknown track"
}

var trackPrefixPattern = regexp.MustCompile(`^\d+\s*-\s*`)

// libraryTitle derives a readable title from a synced mp3 path such as
// <root>/01 - album-slug/03 - track-slug.mp3 -> "Track Slug".
func libraryTitle(path string) string {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	for i := 0; i < 2; i++ {
		base = trackPrefixPattern.ReplaceAllString(base, "")
	}
	if strings.TrimSpace(base) == "" {
		return "unknown track"
	}
	return humanizeSlug(base)
}

// libraryEntryTitle combines the album directory note and the track title for
// display (and for matching the now-playing row).
func libraryEntryTitle(path string) string {
	title := libraryTitle(path)
	if note := libraryAlbumNote(path); note != "" {
		return note + " / " + title
	}
	return title
}

// libraryAlbumNote shows the album directory name for local files.
func libraryAlbumNote(path string) string {
	dir := filepath.Base(filepath.Dir(path))
	dir = trackPrefixPattern.ReplaceAllString(dir, "")
	if dir == "" || dir == "." || dir == string(filepath.Separator) {
		return ""
	}
	return humanizeSlug(dir)
}

// humanizeSlug turns "cold-morning" into "Cold Morning".
func humanizeSlug(slug string) string {
	replacer := strings.NewReplacer("-", " ", "_", " ")
	words := strings.Fields(replacer.Replace(slug))
	for i, word := range words {
		runes := []rune(word)
		runes[0] = []rune(strings.ToUpper(string(runes[0])))[0]
		words[i] = string(runes)
	}
	return strings.Join(words, " ")
}

// friendlyError prefixes an error with a human hint about the audio backend.
// Player-side errors already carry an "mpv error:" prefix; strip it here so
// the status line never shows the words back to back ("mpv error: mpv error:").
func friendlyError(prefix string, err error) string {
	if err == nil {
		return prefix
	}
	msg := strings.TrimPrefix(err.Error(), "mpv error: ")
	return prefix + ": " + msg + " (is mpv installed and running?)"
}

// joinStatus appends a note to a possibly empty status line.
func joinStatus(current, note string) string {
	if current == "" {
		return note
	}
	return current + " — " + note
}
