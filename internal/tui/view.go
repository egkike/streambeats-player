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
	"streambeats/internal/theme"
)

// Theme styles (Phase A): the palette lives on the Model (resolved at
// construction from the settings theme id), so theme switches are pure
// state changes and the view never runs background detection.
func (m Model) titleStyle() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(m.palette.Title))
}

func (m Model) dimStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Dim))
}

func (m Model) selectedStyle() lipgloss.Style {
	// Issue #140 S3: quiet reads through one band — the selected row paints
	// a subtle background fill instead of relying on the fg color alone;
	// legacy themes keep the plain bold-fg style (zero added SGR).
	if m.theme == theme.Quiet {
		return lipgloss.NewStyle().Bold(true).
			Foreground(lipgloss.Color(m.palette.Selected)).
			Background(lipgloss.Color(quietBandColor))
	}
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(m.palette.Selected))
}

func (m Model) playingStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Playing))
}

func (m Model) statusStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Status))
}

func (m Model) barStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Bar))
}

const (
	minListHeight = 3
	defaultWidth  = 80
	// quietBandColor (issue #140 S3): the background of the selected-row
	// band under the Quiet theme, straight from the artboard spec card. It
	// is structural, never persisted: only the quiet theme renders it.
	quietBandColor = "#1c2433"
	// chromeLines is the vertical space OUTSIDE the list viewport, computed
	// exactly: title(1) + separator(1) + header(1) + player bar(2) +
	// keycaps(3, incl. borders) + reserved status slot(1). The keyboard-only
	// cycle (issue #60) replaced the button row + help lines with this block
	// at the same budget (panel stays 24 rows at the standard size).
	chromeLines = 9
)

// totalViewLines is the full-height paint budget: the terminal height when
// known, else title+header+list+chrome with the default list height.
func (m Model) totalViewLines() int {
	if m.height <= 0 {
		return 3 + defaultListHeight + chromeLines
	}
	return m.height
}

// View renders the whole screen. It must stay side-effect free.
//
// Layout contract (issue #31): the top block is title+header, the flexible
// middle is the list viewport padded to its full height, and the bottom block
// (player bar, keycaps, reserved status slot) is anchored to the bottom edge.
// Solo Teclado (issue #60): no zone scan — the render is plain text, the
// keyboard is the sole input.
func (m Model) View() string {
	if m.quitting {
		return ""
	}

	// Title line, issue #140 S3 gated: synthwave adds the cyan second-voice
	// dot between the app name and its description; legacy lines stay
	// byte-identical to the pre-#140 tree.
	title := m.titleStyle().Render("streambeats") + m.dimStyle().Render("  terminal music player")
	if m.theme == theme.Synthwave {
		title = m.titleStyle().Render("streambeats") + m.statusStyle().Render(" ◉") +
			m.dimStyle().Render(" terminal music player")
	}

	lines := make([]string, 0, m.totalViewLines())
	lines = append(lines,
		title,
		"",
		m.headerView(),
	)
	lines = append(lines, m.listRows()...)
	lines = append(lines, m.playerBarLines()...)
	// Keycaps row (Alt A, issue #60): the playback transport as rounded
	// keycaps — visual only, the keyboard does the work.
	lines = append(lines, m.keycapsView())
	// Reserved status slot (Alt 3, issue #60): feedback when there's a
	// message; navigation hints when empty. One line, always present.
	lines = append(lines, m.statusOrNavLine())
	return strings.Join(lines, "\n") + "\n"
}

// statusOrNavLine is the dual-purpose slot (Alt 3, issue #60): when there's
// a feedback message it shows it verbatim; when empty it shows the navigation
// hints. The nav hints use the dim style so they read as ambient, not as a
// message.
func (m Model) statusOrNavLine() string {
	if m.status != "" {
		return m.statusStyle().Render(m.status)
	}
	// Issue #143 S9/S10 + #146 S12-S14: the nav row obeys the painted-width
	// budget on one flat control — longest approved tier that fits — and
	// repaints as the artboard: single-dot separator, badged keys, dim
	// actions.
	return m.navRowFor(m.contentWidth())
}

// navHints is the compact navigation reference shown when the status slot
// is empty (the approved full line TEXT — badges live in navRowFor).
// Only keys NOT shown in the keycaps row (no playback repeats);
// issue #146 S15 carries the single dot.
func navHints() string {
	return navHintsFor(1000)
}

// Solo Teclado width calibration (issue #60 S6/L8, CPR probe): ghostty
// 1.3.1 with the panel preset (grapheme-width-method = legacy) paints the
// transport emoji family at ONE cell — NOT the 2 cells lipgloss assumes —
// while 🔊 paints 2. Border widths are built from this measured table so
// the frame closes even where measured ≠ painted width (round-6 span
// doctrine: hand-rolled spans calibrated on the real panel beat word-wide
// rune measurement).
var keycapIconWidths = map[string]int{"⏮️": 1, "⏯️": 1, "⏹️": 1, "⏭️": 1, "🔊": 2}

// Terminal-adaptive transport (issue #64): the calibrated emoji keycaps only
// close their frames on ghostty 1.3.1 with the panel preset (legacy
// grapheme-width method). Everywhere else the row renders pure-ASCII tokens:
// printable ASCII only, so the painted width equals the rune count on every
// terminal — the same doctrine as the pre-VS16 rounds.
var asciiTokens = map[string]string{
	"⏮️": "|<",
	"⏯️": "||",
	"⏭️": ">|",
	"⏹️": "[]",
	"🔊":  "vol",
}

// isGhosttyTerminal fails closed (issue #64 S7): only the exact env value
// "ghostty" selects the calibrated emoji keycaps; anything else — unset,
// empty, foreign — selects the ASCII tokens. Read ONCE in Launch; never
// inside the render path.
func isGhosttyTerminal(value string) bool {
	return value == "ghostty"
}

// keycapSpec is one cap: an icon from the calibrated table plus an ASCII key.
type keycapSpec struct {
	icon, key string
}

var keycaps = []keycapSpec{
	{"⏮️", "p"},
	{"⏯️", "spc"},
	{"⏭️", "n"},
	{"⏹️", "s"},
	{"🔊", "+/-"},
}

// keycapIconWidth is the calibrated painted width of an icon (CPR probe on
// the panel preset): the transport family paints 1 cell, 🔊 2. Uncalibrated
// icons fall back to the widest assumption so a new icon can't silently
// shrink a frame.
func keycapIconWidth(icon string) int {
	if w, ok := keycapIconWidths[icon]; ok {
		return w
	}
	return 2
}

// keycapInterior is the painted interior width of one cap: " icon key " —
// icon via the calibrated table, key by ASCII rune count. Built from the
// probe table, never from lipgloss's rune measurement (S6).
func keycapInterior(c keycapSpec) int {
	return 3 + keycapIconWidth(c.icon) + len([]rune(c.key))
}

// capWidth is the mode-aware painted width of the cap token (issue #64 S3):
// the calibrated probe table on ghostty — the single width source there —
// the rune count of the ASCII token anywhere else (stable by construction).
func (m Model) capWidth(c keycapSpec) int {
	if m.asciiTransport {
		return len([]rune(asciiTokens[c.icon]))
	}
	return keycapIconWidth(c.icon)
}

// capToken is the mode-aware painted token for one cap.
func (m Model) capToken(c keycapSpec) string {
	if m.asciiTransport {
		return asciiTokens[c.icon]
	}
	return c.icon
}

// capInterior is the mode-aware painted interior width: " token key ".
func (m Model) capInterior(c keycapSpec) int {
	return 3 + m.capWidth(c) + len([]rune(c.key))
}

// keycapsView renders the playback transport. Issue #140 S3: under the
// quiet theme it is one weightless inline line of ASCII tokens and keys —
// no boxes, so the ghostty emoji-width calibration is simply not reached;
// every other theme keeps the bordered caps with the calibrated widths.
func (m Model) keycapsView() string {
	if m.theme == theme.Quiet {
		accent := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Bar)).Bold(true)
		hint := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Dim))
		parts := make([]string, 0, len(keycaps))
		for _, c := range keycaps {
			parts = append(parts, accent.Render(asciiTokens[c.icon])+" "+hint.Render(c.key))
		}
		return strings.Join(parts, "   ")
	}
	border := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Bar))
	accent := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Playing)).Bold(true)
	hint := lipgloss.NewStyle().Foreground(lipgloss.Color(m.palette.Dim))

	tops := make([]string, 0, len(keycaps))
	mids := make([]string, 0, len(keycaps))
	bottoms := make([]string, 0, len(keycaps))
	for _, c := range keycaps {
		w := m.capInterior(c)
		icon := m.capToken(c)
		iconW := m.capWidth(c)
		tops = append(tops, border.Render("╭"+strings.Repeat("─", w)+"╮"))
		// Interior: border bar + plain spaces around the styled token and
		// key — fill-free by owner decision (S8).
		mids = append(mids, border.Render("│")+" "+accent.Render(icon)+" "+
			hint.Render(c.key)+strings.Repeat(" ", w-(2+iconW+len([]rune(c.key))))+border.Render("│"))
		bottoms = append(bottoms, border.Render("╰"+strings.Repeat("─", w)+"╯"))
	}
	return strings.Join(tops, "") + "\n" + strings.Join(mids, "") + "\n" + strings.Join(bottoms, "")
}

// headerView names the focused list.
func (m Model) headerView() string {
	switch {
	case m.mode == ModeConfig:
		return m.titleStyle().Render("Settings") +
			m.dimStyle().Render("  theme / terminal: enter to apply  esc: back")
	case m.mode == ModeLibrary:
		root := m.libraryRoot
		if root == "" {
			root = "local library"
		}
		base := m.libraryDir
		if base == "" {
			base = root
		}
		label := "Local library"
		if base != "" && base != "local library" {
			label = "Local library — " + filepath.Base(base)
		}
		return m.titleStyle().Render(fmt.Sprintf("%s (%d songs, %d folders)", label, len(m.library), len(m.libDirs))) +
			m.dimStyle().Render("  "+root+"  u: up")
	case m.inTracks:
		album, ok := m.selectedAlbum()
		if !ok {
			return m.titleStyle().Render("Tracks")
		}
		label := album.Title
		if album.Artist != "" {
			label = album.Artist + " — " + label
		}
		return m.titleStyle().Render(fmt.Sprintf("%s (%d tracks)", label, len(album.Tracks))) +
			m.dimStyle().Render("  esc: back")
	default:
		return m.titleStyle().Render(fmt.Sprintf("Albums (%d)", len(m.albums)))
	}
}

// listRows renders the visible slice of the focused list, padded to the full
// viewport height (S2: a shorter list keeps its viewport so the bottom block
// stays anchored at the panel edge).
func (m Model) listRows() []string {
	height := m.listHeight()
	width := m.contentWidth()

	rows := make([]string, 0, height)
	switch {
	// Settings screen, issue #140 S4: three labeled sections on ONE flat
	// cursor — the ambient trio under the mode header, the five artboard
	// themes under the themes header, then the existing terminal section.
	// Headers are dim dividers the cursor never lands on.
	case m.mode == ModeConfig:
		all := configAllThemes()
		for i, id := range all {
			if i == 0 {
				rows = append(rows, m.dimStyle().Render("  mode:"))
			} else if i == len(configThemeIDs) {
				rows = append(rows, m.dimStyle().Render("  themes:"))
			}
			label := configLabels[id]
			if id == m.theme {
				label += "  (current)"
			}
			rows = append(rows, m.renderRow(i == m.configIndex, false, label, width))
			// Terminal section header (issue #90): visual-only divider, the
			// cursor does not land on it.
			if i == len(all)-1 {
				rows = append(rows, m.dimStyle().Render("  panel terminal:"))
				for j, terminalID := range m.panelOffer {
					label := terminalID
					if terminalID == m.panelTerminal {
						label += "  (current)"
					}
					rows = append(rows, m.renderRow(len(all)+j == m.configIndex, false, label, width))
				}
				if len(m.panelOffer) == 0 {
					rows = append(rows, m.dimStyle().Render("  no panel terminal installed"))
				}
			}
		}
		// Library root (issue #76): read-only here — edit settings.json
		// (libraryRoot) to point the navigator elsewhere.
		if m.libraryRoot != "" {
			rows = append(rows, m.dimStyle().Render(fmt.Sprintf("  library root: %s (settings.json)", m.libraryRoot)))
		}
		return padRows(rows, height)
	case m.mode == ModeLibrary:
		total := m.libEntryCount()
		if total == 0 {
			rows = append(rows, m.dimStyle().Render("  this folder has no music — u: up, streambeats sync to fill"))
			return padRows(rows, height)
		}
		start, end := windowBounds(total, m.libraryCursor, height)
		for i := start; i < end; i++ {
			if i < len(m.libDirs) {
				// subdirectories first, with a trailing slash marker (#76)
				name := filepath.Base(m.libDirs[i]) + "/"
				rows = append(rows, m.renderRow(i == m.libraryCursor, false, name, width))
				continue
			}
			fileIdx := i - len(m.libDirs)
			path := m.library[fileIdx]
			title := libraryEntryTitle(path)
			// Playing is matched BY INDEX (#84): titles repeat across formats
			// (Song.mp3 / Song.flac) and a title match would mark every row
			// with the same name at once.
			playing := m.playing && m.playingLib == fileIdx
			rows = append(rows, m.renderRow(i == clampCursor(m.libraryCursor, total), playing, title, width))
		}
		return padRows(rows, height)

	case m.inTracks:
		album, ok := m.selectedAlbum()
		if !ok {
			rows = append(rows, m.dimStyle().Render("  no album selected"))
			return padRows(rows, height)
		}
		start, end := windowBounds(len(album.Tracks), m.trackCursor, height)
		if len(album.Tracks) == 0 {
			rows = append(rows, m.dimStyle().Render("  album has no tracks"))
			return padRows(rows, height)
		}
		for i := start; i < end; i++ {
			track := album.Tracks[i]
			label := m.trackRowLabel(i, track)
			playing := m.nowPlaying != "" && m.nowPlaying == trackTitle(track)
			rows = append(rows, m.renderRow(i == clampCursor(m.trackCursor, len(album.Tracks)), playing, label, width))
		}
		return padRows(rows, height)

	default:
		if len(m.albums) == 0 {
			rows = append(rows, m.dimStyle().Render("  no albums cached — run `streambeats refresh`"))
			return padRows(rows, height)
		}
		start, end := windowBounds(len(m.albums), m.albumCursor, height)
		for i := start; i < end; i++ {
			album := m.albums[i]
			rows = append(rows, m.renderRow(i == clampCursor(m.albumCursor, len(m.albums)), false,
				m.albumRowLabel(i, album), width))
		}
		return padRows(rows, height)
	}
}

// albumRowLabel formats one album row; the artboard themes own three
// layouts (issue #140 S3), all gated to their theme id so the legacy
// formats never move:
//   - vinyl: dotted leaders fill the gap between the label and the count;
//   - synthwave: the count takes the second-voice marker;
//   - quiet: the count aligns to a fixed column.
func (m Model) albumRowLabel(i int, album catalog.AlbumEntry) string {
	count := len(album.Tracks)
	switch m.theme {
	case theme.Vinyl:
		head := fmt.Sprintf("%02d  %s", i+1, album.Title)
		// The artboard keeps the artist as the dim middle chunk; plain is
		// faithful enough (rows are single-styled).
		if album.Artist != "" {
			head += "  " + album.Artist
		}
		tail := fmt.Sprintf("%d tracks", count)
		prefixWidth := 2 + len([]rune(head)) + 1 + len([]rune(tail)) + 2
		if dots := m.contentWidth() - prefixWidth; dots > 0 {
			return head + " " + strings.Repeat("·", dots) + " " + tail
		}
		return head + " … " + tail
	case theme.Synthwave:
		return fmt.Sprintf("%02d  %s  » %d tracks", i+1, album.Title, count)
	case theme.Quiet:
		return fmt.Sprintf("%02d  %-24s %2d", i+1, album.Title, count)
	default:
		// Legacy byte-parity contract: the shipped format, artist included.
		label := fmt.Sprintf("%02d  %s", i+1, album.Title)
		if album.Artist != "" {
			label += "  " + album.Artist
		}
		return label + fmt.Sprintf("  (%d tracks)", count)
	}
}

// trackRowLabel formats one track row with the theme's tail layout (issue
// #140 S3): vinyl fills with leaders, synthwave markers the duration with
// the second voice, the rest keep the plain two-space tail.
func (m Model) trackRowLabel(i int, track catalog.Track) string {
	head := fmt.Sprintf("%02d  %s", i+1, trackTitle(track))
	if track.DurationSeconds <= 0 {
		return head
	}
	tail := formatDuration(track.DurationSeconds)
	if m.theme == theme.Vinyl {
		prefixWidth := 2 + len([]rune(head)) + 1 + len([]rune(tail)) + 2
		if dots := m.contentWidth() - prefixWidth; dots > 0 {
			return head + " " + strings.Repeat("·", dots) + " " + tail
		}
		return head + " … " + tail
	}
	if m.theme == theme.Synthwave {
		return head + "  » " + tail
	}
	return head + "  " + tail
}

// padRows fills rows up to height with blank lines (viewport padding).
func padRows(rows []string, height int) []string {
	for len(rows) < height {
		rows = append(rows, "")
	}
	return rows
}

// renderRow draws one list row with cursor/playing markers. Under the
// quiet theme the selected row pads to full width so its background band
// reads as one calm bar (issue #140 S3); other themes are untouched.
func (m Model) renderRow(selected, playing bool, label string, width int) string {
	marker := "  "
	if selected {
		marker = "▸ "
	}
	if playing {
		label = "♪ " + label
	}
	line := truncate(marker+label, width)
	if selected && m.theme == theme.Quiet {
		if pad := width - len([]rune(line)); pad > 0 {
			line += strings.Repeat(" ", pad)
		}
	}
	switch {
	case selected:
		return m.selectedStyle().Render(line)
	case playing:
		return m.playingStyle().Render(line)
	default:
		return line
	}
}

// playerBar renders the now-playing line and progress.
// playerBarLines splits the now-playing line and the progress line.
func (m Model) playerBarLines() []string {
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
	line += m.dimStyle().Render(fmt.Sprintf("   vol %d%%", m.volume))
	line = truncate(line, width)

	barWidth := width - 20
	if barWidth < 10 {
		barWidth = 10
	}
	progress := progressBar(m.position, m.duration, barWidth)
	times := fmt.Sprintf("%s / %s", formatDuration(m.position), formatDuration(m.duration))
	// Issue #140 S3: under synthwave the second voice (the status palette,
	// cyan) carries the timestamps; legacy lines keep the dim style.
	timesStyle := m.dimStyle()
	if m.theme == theme.Synthwave {
		timesStyle = m.statusStyle()
	}
	return []string{line, m.barStyle().Render(progress) + " " + timesStyle.Render(times)}
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

// navHintPair is one key+action unit of the nav row (issue #146 S12):
// the key renders as a SGR badge (zero painted width), the action stays
// plain-dim.
type navHintPair struct{ key, action string }

var (
	// Full vocabulary with the arrows; the first separator is the single
	// painted dot (owner refinement, artboard 05).
	navPairsFull = []navHintPair{
		{"↑↓ k/j", ""}, {"enter", "open"}, {"esc", "back"},
		{"l", "lib"}, {"c", "settings"}, {"q", "quit"},
	}
	// Single-space tier: same vocabulary, one space between pairs. Issue
	// #151 S19: the standalone 50-rune arrows tier dissolved into the
	// compact full tier; the pair set stays for the badge render path.
	navPairsArrows = navPairsFull
	// Arrows dropped (47).
	navPairsCompact = []navHintPair{
		{"k/j", ""}, {"enter", "open"}, {"esc", "back"},
		{"l", "lib"}, {"c", "settings"}, {"q", "quit"},
	}
	// Narrow-panel tier for the <=46-column strips (issue #149 S17):
	// the move vocabulary compresses away but the library/settings keys
	// stay. 43 runes, single-space separators.
	navPairsLibrary = []navHintPair{
		{"enter", "open"}, {"esc", "back"}, {"l", "lib"},
		{"c", "settings"}, {"q", "quit"},
	}
	// Bare navigation essentials for tight strips (26).
	navPairsEssential = []navHintPair{{"enter", "open"}, {"esc", "back"}, {"q", "quit"}}
	// Floor (6): below six painted columns nothing renders (documented
	// degenerate case, issue #143 S10).
	navPairsFloor = []navHintPair{{"q", "quit"}}
)

// navBadgeBgs (issue #146 S13): per-theme key-band tokens; unlisted
// themes render plain-dim keys — their row differs only by the dot.
var navBadgeBgs = map[theme.Theme]string{
	theme.Vinyl:     "#241b12",
	theme.Synthwave: "#2a2350",
	theme.Quiet:     quietBandColor,
	theme.Cute:      "#28121E",
	theme.Sexy:      "#28121E",
}

// badgeKey renders a key token: every whitespace word is badge-wrapped so
// multi-token keys (↑↓ k/j) leave their separating space un-styled (the
// artboard shows two adjacent badges).
func (m Model) badgeKey(key string) string {
	style := m.navKeyStyle()
	fields := make([]string, 0, 2)
	for _, word := range strings.Fields(key) {
		fields = append(fields, style.Render(word))
	}
	return strings.Join(fields, " ")
}

// navKeyStyle is the badge look: bold Selected fg over the theme band;
// themes without a band keep the plain dim paint of v1.11.1.
func (m Model) navKeyStyle() lipgloss.Style {
	bg, ok := navBadgeBgs[m.theme]
	if !ok {
		return m.dimStyle()
	}
	return lipgloss.NewStyle().Bold(true).
		Foreground(lipgloss.Color(m.palette.Selected)).
		Background(lipgloss.Color(bg))
}

// navRowFor paints the longest tier that fits width with badges and the
// dot ladder of the artboard. Measured rune budgets (issue #146 S12):
// 56 / 50 / 47 / 26 / floor 6; the badge SGR adds zero painted width, so
// the stripANSI of every tier equals navHintsFor's text exactly (S14).
func (m Model) navRowFor(width int) string {
	switch {
	case width >= 52:
		// Issue #151 S19: the compact full tier — single spaces make the
		// dot affordable at ~52-column side panels (owner retest).
		return m.renderNavPairs(navPairsFull, true, " ")
	case width >= 47:
		return m.renderNavPairs(navPairsCompact, false, " ")
	case width >= 43:
		return m.renderNavPairs(navPairsLibrary, false, " ")
	case width >= 26:
		return m.renderNavPairs(navPairsEssential, false, " ")
	default:
		return m.renderNavPairs(navPairsFloor, false, " ")
	}
}

// navRowText renders the tier geometry without SGR — the exact painted
// text navHintsFor delegates to (issue #146 S14); tier 0 carries the dot.
func navRowText(pairs []navHintPair, dotFirst bool, sep string) string {
	var b strings.Builder
	for i, p := range pairs {
		if i > 0 {
			run := sep
			if dotFirst && i == 1 {
				run = " · "
			}
			b.WriteString(run)
		}
		b.WriteString(p.key)
		if p.action != "" {
			b.WriteString(" " + p.action)
		}
	}
	return b.String()
}

// navHintsFor returns the approved tier TEXT for the width (no badges —
// styling lives in navRowFor; the stripANSI of navRowFor equals this exact
// text, which the S14 contract pins).
func navHintsFor(width int) string {
	switch {
	case width >= 52:
		// Issue #151 S19: the compact full tier — single spaces make the
		// dot affordable at ~52-column side panels (owner retest).
		return navRowText(navPairsFull, true, " ")
	case width >= 47:
		return navRowText(navPairsCompact, false, " ")
	case width >= 43:
		return navRowText(navPairsLibrary, false, " ")
	case width >= 26:
		return navRowText(navPairsEssential, false, " ")
	default:
		return navRowText(navPairsFloor, false, " ")
	}
}

// renderNavPairs joins badges, dim actions and separator runs into one
// row. tier 0 (dotFirst) puts the single Bar-colored dot between the move
// pair and the first action, plain double spaces elsewhere; other tiers
// use the plain single-space run.
func (m Model) renderNavPairs(pairs []navHintPair, dotFirst bool, sep string) string {
	var b strings.Builder
	for i, p := range pairs {
		if i > 0 {
			run := sep
			if dotFirst && i == 1 {
				// Issue #149 S16: the dot rides the Status hue (distinct
				// from the badge keys, high contrast on the dark
				// surface) — the Bar tone read too dark in ghostty.
				run = " " + m.statusStyle().Render("·") + " "
			}
			b.WriteString(run)
		}
		b.WriteString(m.badgeKey(p.key))
		if p.action != "" {
			b.WriteString(" " + m.dimStyle().Render(p.action))
		}
	}
	return b.String()
}
