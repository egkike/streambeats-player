package tui

// Unit tests for the pure rendering helpers (no terminal involved).

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"streambeats/internal/catalog"
)

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name    string
		seconds float64
		want    string
	}{
		{"zero", 0, "0:00"},
		{"seconds only", 5, "0:05"},
		{"minute", 65, "1:05"},
		{"just under an hour", 3599, "59:59"},
		{"one hour", 3600, "1:00:00"},
		{"over an hour", 3725, "1:02:05"},
		{"negative clamps to zero", -12, "0:00"},
		{"NaN clamps to zero", math.NaN(), "0:00"},
		{"fractional truncates", 61.9, "1:01"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatDuration(tc.seconds); got != tc.want {
				t.Fatalf("formatDuration(%v) = %q, want %q", tc.seconds, got, tc.want)
			}
		})
	}
}

func TestProgressBar(t *testing.T) {
	tests := []struct {
		name               string
		position, duration float64
		width              int
		want               string
	}{
		{"zero width", 5, 10, 0, ""},
		{"no duration", 0, 0, 10, "░░░░░░░░░░"},
		{"half", 5, 10, 10, "█████░░░░░"},
		{"empty", 0, 10, 10, "░░░░░░░░░░"},
		{"full", 10, 10, 10, "██████████"},
		{"overrun clamps", 20, 10, 10, "██████████"},
		{"negative position clamps", -5, 10, 4, "░░░░"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := progressBar(tc.position, tc.duration, tc.width); got != tc.want {
				t.Fatalf("progressBar(%v, %v, %d) = %q, want %q",
					tc.position, tc.duration, tc.width, got, tc.want)
			}
		})
	}
}

func TestWindowBounds(t *testing.T) {
	tests := []struct {
		name               string
		n, cursor, height  int
		wantStart, wantEnd int
	}{
		{"empty list", 0, 0, 5, 0, 0},
		{"zero height", 10, 0, 0, 0, 0},
		{"fits entirely", 3, 2, 5, 0, 3},
		{"top edge", 10, 0, 4, 0, 4},
		{"middle centers", 10, 5, 4, 3, 7},
		{"bottom edge", 10, 9, 4, 6, 10},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			start, end := windowBounds(tc.n, tc.cursor, tc.height)
			if start != tc.wantStart || end != tc.wantEnd {
				t.Fatalf("windowBounds(%d, %d, %d) = (%d, %d), want (%d, %d)",
					tc.n, tc.cursor, tc.height, start, end, tc.wantStart, tc.wantEnd)
			}
		})
	}
}

func TestLibraryTitle(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{"album and track prefix", "/lib/01 - lofi-chill/03 - rainy-day.mp3", "Rainy Day"},
		{"single prefix", "02 - night-drive.mp3", "Night Drive"},
		{"no prefix", "song.mp3", "Song"},
		{"underscores", "01 - calm_waves.mp3", "Calm Waves"},
		{"no extension", "01 - intro", "Intro"},
		{"empty base", "/lib/01 - album/01 - .mp3", "unknown track"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := libraryTitle(tc.path); got != tc.want {
				t.Fatalf("libraryTitle(%q) = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}

func TestLibraryEntryTitle(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{"album and track", "/lib/01 - lofi-chill/03 - rainy-day.mp3", "Lofi Chill / Rainy Day"},
		{"flat file", "/lib/song.mp3", "Lib / Song"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := libraryEntryTitle(tc.path); got != tc.want {
				t.Fatalf("libraryEntryTitle(%q) = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}

func TestHumanizeSlug(t *testing.T) {
	tests := []struct{ in, want string }{
		{"cold-morning", "Cold Morning"},
		{"single", "Single"},
		{"two_words", "Two Words"},
		{"", ""},
	}
	for _, tc := range tests {
		if got := humanizeSlug(tc.in); got != tc.want {
			t.Fatalf("humanizeSlug(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name  string
		in    string
		width int
		want  string
	}{
		{"short", "abc", 10, "abc"},
		{"exact", "abcde", 5, "abcde"},
		{"long", "abcdefgh", 5, "abcd…"},
		{"width one", "abc", 1, "…"},
		{"zero width", "abc", 0, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := truncate(tc.in, tc.width); got != tc.want {
				t.Fatalf("truncate(%q, %d) = %q, want %q", tc.in, tc.width, got, tc.want)
			}
		})
	}
}

func TestMPVArgs(t *testing.T) {
	args := mpvArgs("/run/user/1000/streambeats-mpv.sock")
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"--no-video",
		"--no-terminal",
		"--idle",
		"--input-ipc-server=/run/user/1000/streambeats-mpv.sock",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("mpv args %q are missing %q", joined, want)
		}
	}
}

// --- Panel layout (issue #31): bottom-anchored, status slot reserved. ---

func layoutModel(status string) Model {
	return NewModel(Options{
		Catalog: &catalog.Catalog{Albums: []catalog.AlbumEntry{
			{Title: "Alpha"},
			{Title: "Beta"},
		}},
		PlayerNote: status,
	})
}

// S2/S3: with fewer items than viewport rows, the player bar sits at the
// bottom: the blank rows between the last list row and the bottom block are
// the list's viewport padding.
func TestViewAnchorsBottomBlockToTheBottomPanelRight(t *testing.T) {
	m := layoutModel("")
	m.width, m.height = 60, 24
	out := m.View()
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	total := len(lines)
	barAt := strings.Index(out, "nothing playing")
	if barAt < 0 {
		t.Fatalf("player bar missing:\n%s", out)
	}
	// The bottom block (player bar 2 lines + keycaps 3 + reserved status 1)
	// occupies the last 6 lines; nothing after it but the trailing newline.
	if got := strings.Join(lines[total-6:], "\n"); !strings.Contains(got, "nothing playing") {
		t.Fatalf("player bar not inside the last 6 lines:\n%s", got)
	}
	// The list viewport is fully padded: the keycap+status rows are the last
	// 4 lines, so the gap between the last list row ("Beta") and the bottom
	// block must be listHeight-2 blank rows (listHeight = 24-9 = 15).
	wantGap := 15 - 2
	gap := 0
	for i := total - 7; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) == "" {
			gap++
			continue
		}
		break
	}
	if gap != wantGap {
		t.Fatalf("blank rows between list and bottom block = %d, want %d (list padding)", gap, wantGap)
	}
	if got := strings.Count(out, "Alpha"); got < 1 {
		t.Fatalf("list rows missing:\n%s", out)
	}
}

// S4/S5 (issue #60): the status slot is reserved — with or without a message
// the View has the same line count, so the controls never jump vertically —
// and DUAL-PURPOSE (Alt 3, owner decision): a message renders as-is; an empty
// status renders the navigation hints. It is the panel's last line, under the
// keycap row.
func TestViewReservesTheStatusSlotR31(t *testing.T) {
	withStatus := layoutModel("theme saved")
	withStatus.width, withStatus.height = 60, 24
	without := layoutModel("")
	without.width, without.height = 60, 24

	a := strings.Split(strings.TrimSuffix(withStatus.View(), "\n"), "\n")
	b := strings.Split(strings.TrimSuffix(without.View(), "\n"), "\n")
	if len(a) != len(b) {
		t.Fatalf("line count changed with status: %d vs %d (slot must be reserved)", len(a), len(b))
	}
	// The slot is the last painted line: player bar(2) + keycaps(3) + status(1).
	statusAt := len(a) - 1
	if got := strings.TrimSpace(a[statusAt]); got != "theme saved" {
		t.Fatalf("status slot must render the message as-is, got %q", got)
	}
	if strings.Contains(b[statusAt], "theme saved") || !strings.Contains(b[statusAt], "enter open") {
		t.Fatalf("empty status slot must show the nav hints, got %q", b[statusAt])
	}
}

// S5: total rendered lines equal the terminal height when it is tall enough
// (alt-screen contract: full-height paint, no dead space at the end).
func TestViewFillsTheWholePanelHeightR31(t *testing.T) {
	m := layoutModel("")
	m.width, m.height = 60, 24
	rendered := strings.Count(strings.TrimSuffix(m.View(), "\n"), "\n") + 1
	if rendered != 24 {
		t.Fatalf("rendered %d lines for a 24-row panel, want full-height paint", rendered)
	}
}

// Solo Teclado width calibration (issue #60 S6/L8): the caps' borders must
// close on the OWNER's panel, where ghostty 1.3.1 preset-only runs
// grapheme-width-method = legacy and paints the transport emoji family at
// ONE cell while lipgloss measures TWO (CPR probe: ESC[6n cursor reports —
// every emoji base x20 lands on the same column as the ASCII baseline;
// 🔊 lands 20 columns further, i.e. 2 cells). The guard pins the measured
// table so the hand-built rows drift-proof; icons outside the table are a
// test failure, not an auto-guess.
func TestKeycapWidthsMatchTheCalibratedTable(t *testing.T) {
	// 🔀 (U+1F500) is EAW-wide emoji-presentation (no VS16): the CPR doctrine
	// predicts 2 painted cells on the ghostty panel preset, like 🔊 — issue
	// #172 S2; the owner's live panel check after GREEN re-confirms (#60
	// ritual: a mis-measured probe flips the table, never the frame math).
	want := map[string]int{"⏮️": 1, "⏯️": 1, "⏹️": 1, "⏭️": 1, "🔊": 2, "🔀": 2}
	if len(keycapIconWidths) != len(want) {
		t.Fatalf("keycapIconWidths = %v, want the calibrated table %v (measured by the CPR probe)", keycapIconWidths, want)
	}
	for icon, width := range want {
		if got := keycapIconWidth(icon); got != width {
			t.Fatalf("keycapIconWidth(%q) = %d, calibrated %d (CPR probe)", icon, got, width)
		}
	}
	for _, c := range keycaps {
		if _, ok := keycapIconWidths[c.icon]; !ok {
			t.Fatalf("cap icon %q is not in the calibrated width table", c.icon)
		}
		for _, r := range c.key {
			if r < 0x20 || r >= 0x7f {
				t.Fatalf("cap key %q must be plain ASCII (painted = counted), got %q", c.key, c.key)
			}
		}
		if got := keycapInterior(c); got != 3+keycapIconWidth(c.icon)+len([]rune(c.key)) {
			t.Fatalf("keycapInterior(%v) = %d, want 3 + calibrated icon width + key runes", c, got)
		}
	}
}

// Solo Teclado (issue #60 S3+S6): the keycap row renders the approved
// icon+key pairs and the borders CLOSE along the calibrated widths: icons
// ⏮️⏯️⏹️⏭️ = 1 cell, 🔊 = 2 (CPR probe) — so the interiors are 5/7/5/5/8 and
// the frames are 7/9/7/7/10 wide end to end.
func TestKeycapsRowShowsTheApprovedPairs(t *testing.T) {
	// lipgloss downgrades to NONE outside a TTY and drops every SGR run:
	// the A2 accent pins need real ANSI, so force the profile for this test
	// only (restored afterwards).
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(prev)

	m := layoutModel("")
	m.width, m.height = 52, 24
	rows := strings.Split(strings.TrimSuffix(m.View(), "\n"), "\n")
	caps := stripANSI(strings.Join(rows[20:23], "\n"))
	// O2 (issue #60 S7 + #172 S1): prev, play, next, stop, volume — shuffle LAST.
	for _, want := range []string{
		"\u256d─────\u256e\u256d───────\u256e\u256d─────\u256e\u256d─────\u256e\u256d────────\u256e\u256d──────\u256e",
		"│ \u23ee\ufe0f p ││ \u23ef\ufe0f spc ││ \u23ed\ufe0f n ││ \u23f9\ufe0f s ││ \U0001f50a +/- ││ \U0001f500 z │",
		"\u2570─────\u256f\u2570───────\u256f\u2570─────\u256f\u2570─────\u256f\u2570────────\u256f\u2570──────\u256f",
	} {
		if !strings.Contains(caps, want) {
			t.Fatalf("keycaps missing %q:\n%s", want, caps)
		}
	}
	// A2 (S8/L11, owner pick): icons are the accent — BOLD over the panel
	// background, NO fill (the owner rejected the filled look on-device);
	// widths unchanged (stripANSI above still passes).
	raw := strings.Join(rows[20:23], "\n")
	if !strings.Contains(raw, "\x1b[1;") {
		t.Fatalf("accent bold missing on the icons:\n%q", raw)
	}
	if strings.Contains(raw, "\x1b[48") {
		t.Fatalf("caps must render WITHOUT the fill background:\n%q", raw)
	}
}

// stripANSI removes SGR sequences so tests can compare the painted text
// layout without caring where lipgloss injected styling runs.
func stripANSI(s string) string {
	return ansiPattern.ReplaceAllString(s, "")
}

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// Solo Teclado (issue #60 S4): the keycap row owns the playback keys, so the
// nav hints advertise ONLY what the caps don't show — they never repeat a
// playback action or its key.
func TestNavHintsExcludeTheKeycapKeys(t *testing.T) {
	hints := navHints()
	if hints != "\u2191\u2193 k/j \u00b7 enter open esc back l lib c settings q quit" {
		t.Fatalf("nav hints = %q, want the approved compact line (issue #151 S19)", hints)
	}
	for _, repeated := range []string{"\u23ee\ufe0f", "\u23ef\ufe0f", "\u23f9\ufe0f", "\u23ed\ufe0f", "\U0001f50a", "\u2423", "\u00b1", "\u2795", "\u2796"} {
		if strings.Contains(hints, repeated) {
			t.Fatalf("nav hints repeat the keycap token %q: %q", repeated, hints)
		}
	}
}

// Issue #143 S9: the nav row is the one view row that never ran a painted-
// width budget — on side-anchored panels (~50-56 columns) the v1.11.0 line
// (55 runes) clips its tail and the owner saw "q qui". It must fit any
// painted width: no wrap, no clipped tail.
func TestNavHintsFitThePaintedWidthAtEveryTier(t *testing.T) {
	for _, w := range []int{84, 52, 51, 47, 46, 43, 26, 25, 10} {
		m := NewModel(Options{Theme: "system", ThemeDark: true, Width: w, Height: 24})
		// The empty-catalog boot message is pre-existing status behavior;
		// this test pins the NAV row, not status rendering.
		m.status = ""
		got := stripANSI(m.statusOrNavLine())
		if runes := len([]rune(got)); runes > w {
			t.Errorf("width %d: nav row paints %d runes, must fit (no wrap, no clip):\n%q", w, runes, got)
		}
	}
}

// Issue #143 S10: the approved tier ladder, wired through statusOrNavLine
// via a stub for this RED run (the real ladder replaces it in GREEN).
func TestNavHintTiersArePinned(t *testing.T) {
	// Issue #146 S12 + #149 S17 + #151 S19: the full tier is the compact
	// form — the single dot between the move pair and the rest, single
	// spaces everywhere else (52 runes) — so the dot survives narrow
	// ghostty side panels (~51-55 columns, owner's retest evidence).
	// The 56 and 50 tiers dissolve into it.
	wantTiers := []string{
		"\u2191\u2193 k/j \u00b7 enter open esc back l lib c settings q quit",
		"k/j enter open esc back l lib c settings q quit",
		"enter open esc back l lib c settings q quit",
		"enter open esc back q quit",
		"q quit",
	}
	cases := []struct {
		width int
		want  string
	}{
		{84, wantTiers[0]},
		{52, wantTiers[0]},
		{51, wantTiers[1]},
		{47, wantTiers[1]},
		{46, wantTiers[2]},
		{43, wantTiers[2]},
		{42, wantTiers[3]},
		{26, wantTiers[3]},
		{25, wantTiers[4]},
		{6, wantTiers[4]},
	}
	for _, tt := range cases {
		if got := navHintsFor(tt.width); got != tt.want {
			t.Errorf("navHintsFor(%d) = %q, want %q", tt.width, got, tt.want)
		}
	}
}

// Issue #146 S13/S14: badged keys paint a theme band under the mapped
// themes — cute carries the gentle-pi selection token #28121E (SGR
// 48;2;40;18;30); legacy themes paint NO band (their row differs only by
// the dot). The badge SGR is zero painted width (S14).
func TestNavRowBadgesByTheme(t *testing.T) {
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(prev)

	cute := themedAlbumModel(t, "cute")
	cute.status = ""
	row := cute.statusOrNavLine()
	if !strings.Contains(row, "48;2;40;18;30") {
		t.Fatalf("cute nav row lost the #28121E badge band:\n%s", row)
	}
	if runes := len([]rune(stripANSI(row))); runes != 52 {
		t.Errorf("cute nav row paints %d runes, want exactly 52 (badge is zero-width, issue #151 S19)", runes)
	}
	legacy := themedAlbumModel(t, "system")
	legacy.status = ""
	if strings.Contains(legacy.statusOrNavLine(), ";48;2;") {
		t.Fatalf("legacy system nav row must paint no band:\n%s", legacy.statusOrNavLine())
	}
}

// Issue #149 S16: the tier-0 dot paints in the Status hue (distinct
// from the badge keys, high contrast) — cute's #F2B86D paints SGR
// 38;2;242;184;109; the painted width stays exactly 56 (S16/S14).
func TestNavDotPaintsInTheStatusHue(t *testing.T) {
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(prev)

	cute := themedAlbumModel(t, "cute")
	cute.status = ""
	row := cute.statusOrNavLine()
	if !strings.Contains(row, "38;2;242;184;109") {
		t.Fatalf("the dot lost the Status hue (want #F2B86D 38;2;242;184;109):\n%s", row)
	}
	if runes := len([]rune(stripANSI(row))); runes != 52 {
		t.Errorf("full nav row paints %d runes, want 52 with the dot (zero-width SGR, issue #151 S19)", runes)
	}
}

// Issue #140 S5: the settings screen manages more than themes since the
// panel terminal section (issue #90); the approved rename is "c settings".
func TestNavHintsRenameToCSettings(t *testing.T) {
	hints := navHints()
	if !strings.Contains(hints, "c settings") {
		t.Fatalf("nav hints lost the approved c settings rename: %q", hints)
	}
	if strings.Contains(hints, "c theme") {
		t.Fatalf("nav hints still advertise the old c theme label: %q", hints)
	}
}

// Issue #140 S4 (T2): the settings screen renders the three labeled
// sections — ambient themes first, artboard themes after — and every theme
// row carries its owner-approved display name.
func TestSettingsScreenListsAllThemesInSections(t *testing.T) {
	m := NewModel(Options{Theme: "system", ThemeDark: true})
	m.mode = ModeConfig
	view := stripANSI(m.View())
	for _, want := range []string{"mode:", "Claro", "Oscuro", "System", "themes:",
		"Vinyl dark", "Synthwave night", "Quiet ink", "Cute", "Sexy", "panel terminal:"} {
		if !strings.Contains(view, want) {
			t.Errorf("settings view is missing %q:\n%s", want, view)
		}
	}
	// Section order is pinned: mode: before themes:, themes: before the
	// panel terminal rows.
	idx := func(s string) int { return strings.Index(view, s) }
	if !(idx("mode:") < idx("themes:") && idx("themes:") < idx("panel terminal:")) {
		t.Errorf("section headers out of order:\n%s", view)
	}
}

// --- per-theme structure (issue #140 S3, owner scope decision: palette +
// --- structure per theme; every branch is gated to its own theme id so the
// --- legacy claro/oscuro/system bytes never move).

// themedAlbumModel builds a catalog model of the given theme id with two
// albums (9 and 8 tracks) so every structure branch below sees real counts
// in its rows. Uses NewModel directly — no fakes needed for rendering.
func themedAlbumModel(t testing.TB, themeID string) Model {
	t.Helper()
	mk := func(title string, tracks int) catalog.AlbumEntry {
		a := catalog.AlbumEntry{Slug: title, Title: title, Artist: "StreamBeats"}
		for i := 0; i < tracks; i++ {
			a.Tracks = append(a.Tracks, catalog.Track{Title: fmt.Sprintf("Track %02d", i+1), DurationSeconds: 200})
		}
		return a
	}
	m := NewModel(Options{
		Theme:     themeID,
		ThemeDark: true,
		Width:     80,
		Catalog:   &catalog.Catalog{Albums: []catalog.AlbumEntry{mk("Cold Front", 9), mk("Neon Alley", 8)}},
	})
	return m
}

// Issue #140 S3: vinyl rows fill the gap between the label and its count
// tail with dotted leaders. The artist stays in place (artboard fidelity),
// so the leader run anchors on the whole label head.
func TestVinylRowsUseDottedLeaders(t *testing.T) {
	view := stripANSI(themedAlbumModel(t, "vinyl").View())
	if !strings.Contains(view, `StreamBeats ·`) || !strings.Contains(view, `···`) {
		t.Fatalf("vinyl album row is missing dotted leaders after the label:\n%s", view)
	}
	if !strings.Contains(view, ` 9 tracks`) {
		t.Fatalf("vinyl album row lost its leader-tail format:\n%s", view)
	}
}

// Issue #140 S3: synthwave speaks with the cyan second voice — markered
// album counts and a dot separator in the title line.
func TestSynthwaveRowsUseTheSecondVoiceMarkers(t *testing.T) {
	view := stripANSI(themedAlbumModel(t, "synthwave").View())
	if !strings.Contains(view, `» 8 tracks`) {
		t.Fatalf("synthwave album row lost the markered count:\n%s", view)
	}
	if !strings.Contains(view, `◉`) {
		t.Fatalf("synthwave title line lost the dot separator:\n%s", view)
	}
}

// Issue #140 S3: quiet reads through one band — the selected row carries a
// subtle background fill; legacy themes never emit any background SGR.
func TestQuietSelectedRowCarriesABandLegacyDoesNot(t *testing.T) {
	// Background SGR arrives only with a color profile: force truecolor for
	// this test like the keycap pins do (restored afterwards).
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(prev)

	quiet := themedAlbumModel(t, "quiet")
	// lipgloss merges fg+bg into one SGR run: pin the band's own composite
	// code (48;2;28;36;51 = #1c2433) instead of a generic 48 prefix.
	if !strings.Contains(quiet.View(), "48;2;28;36;51") {
		t.Fatalf("quiet selected row lost the background band:\n%s", quiet.View())
	}
	legacy := themedAlbumModel(t, "system")
	if strings.Contains(legacy.View(), "\x1b[48;") {
		t.Fatalf("legacy system view must not paint any background SGR:\n%s", legacy.View())
	}
}

// Issue #140 S3: quiet drops the keycap boxes for a weightless inline
// transport — one line of ascii tokens + keys; legacy themes keep the box
// caps exactly as calibrated (issue #60/#64).
func TestQuietTransportIsBoxlessInline(t *testing.T) {
	quietView := stripANSI(themedAlbumModel(t, "quiet").View())
	if strings.Contains(quietView, "╭") {
		t.Fatalf("quiet transport must not render the keycap boxes:\n%s\n", quietView)
	}
	if !strings.Contains(quietView, `|< p   || spc   >| n`) {
		t.Fatalf("quiet inline transport is missing the token groups:\n%s\n", quietView)
	}
	legacyView := stripANSI(themedAlbumModel(t, "system").View())
	if !strings.Contains(legacyView, "╭") {
		t.Fatalf("legacy system transport lost the calibrated box caps:\n%s", legacyView)
	}
}

// Issue #158 S29: the pure middle-ellipsis helper — head ~3/5, ellipsis,
// tail ~2/5 of the remaining budget, exact-fit passthrough. Literals are
// pinned so the policy is visible, not derived from the same math.
func TestTruncateMiddleKeepsBothEnds(t *testing.T) {
	long := "An Extremely Long Album Title That Keeps Going Past Reasonable Width Boundaries" // 79 runes
	tests := []struct {
		budget int
		want   string
	}{
		{0, ""},
		{79, long}, // exact fit passes through
		{42, "An Extremely Long Album … Width Boundaries"},
		{30, "An Extremely Long…h Boundaries"},
		{2, "…"},
	}
	for _, tt := range tests {
		if got := truncateMiddle(long, tt.budget); got != tt.want {
			t.Errorf("truncateMiddle(79-rune title, %d) = %q, want %q", tt.budget, got, tt.want)
		}
	}
	if got := truncateMiddle("Short", 30); got != "Short" {
		t.Errorf("short titles pass through untouched: %q", got)
	}
}

// Issue #158 S26: overflowing album rows drop the artist chunk first,
// then middle-truncate the title so the count tail ALWAYS paints
// (stripANSI of the row ends with the count; painted ≤ width).
func TestOverflowingAlbumRowKeepsTheCountTail(t *testing.T) {
	a := catalog.AlbumEntry{Slug: "long", Title: "An Extremely Long Album Title That Keeps Going Past Reasonable Width Boundaries", Artist: "StreamBeats"}
	a.Tracks = append(a.Tracks, catalog.Track{Title: "t"})
	for _, themeID := range []string{"cute", "system", "synthwave", "quiet"} {
		m := NewModel(Options{Theme: themeID, ThemeDark: true, Width: 54, Catalog: &catalog.Catalog{Albums: []catalog.AlbumEntry{a}}})
		m.status = ""
		view := stripANSI(m.View())
		var line string
		for _, l := range strings.Split(view, "\n") {
			if strings.Contains(l, "01") && strings.Contains(l, "Extrem") {
				line = l
				break
			}
		}
		if line == "" {
			t.Fatalf("theme %s: overflow row missing:\n%s", themeID, view)
		}
		if runes := len([]rune(line)); runes > 54 {
			t.Errorf("theme %s: overflow row paints %d runes at width 54", themeID, runes)
		}
		// quiet renders the count as a bare aligned column (no 'tracks'
		// word by design); the others carry the full count tail.
		countVisible := strings.Contains(line, " 1 ")
		if themeID != "quiet" {
			countVisible = strings.Contains(line, "1 tracks")
		}
		if !countVisible {
			t.Errorf("theme %s: the count tail died on overflow (today's bug):\n%q", themeID, line)
		}
		if !strings.Contains(line, "…") {
			t.Errorf("theme %s: overflow title lost the middle ellipsis:\n%q", themeID, line)
		}
	}
}

// Issue #158 S27: overflow TRACK rows keep the duration tail too.
func TestOverflowingTrackRowKeepsTheDurationTail(t *testing.T) {
	m := themedAlbumModel(t, "cute")
	m.width, m.height = 54, 24
	m.inTracks = true
	m.trackCursor = 0
	// The track list renders selectedAlbum()'s album; swap long titles in.
	m.albums = []catalog.AlbumEntry{{Slug: "long", Title: "Long Album", Tracks: []catalog.Track{{Title: "An Extremely Long Track Title That Keeps Going Past Every Reasonable Width", DurationSeconds: 200}}}}
	view := stripANSI(m.View())
	var line string
	for _, l := range strings.Split(view, "\n") {
		if strings.Contains(l, "An Extremely") {
			line = l
			break
		}
	}
	if line == "" {
		t.Fatalf("overflow track row missing:\n%s", view)
	}
	if !strings.Contains(line, "…") {
		t.Errorf("overflow track title lost the middle ellipsis:\n%q", line)
	}
	if !strings.Contains(line, "3:20") {
		t.Errorf("the duration tail died on overflow:\n%q", line)
	}
}

// Issue #172 S5: the 6-cap transport row must fit the 52-column panel in
// BOTH render modes (painted columns, SGR-stripped). The emoji probe table
// assumption (🔀 = 2, like 🔊) is what keeps the emoji frames closed; if a
// live probe ever flips it down to 1, the table changes — never the frame
// math (#60 CPR ritual).
func TestKeycapsRowFitsAtWidth52(t *testing.T) {
	m := layoutModel("")
	m.width, m.height = 52, 24
	rows := strings.Split(strings.TrimSuffix(m.View(), "\n"), "\n")
	emojiLine := stripANSI(strings.Join(rows[20:23], "\n"))
	if strings.Contains(emojiLine, "╭") {
		if widest := maxRuneWidth(emojiLine); widest > 52 {
			t.Fatalf("emoji keycaps row paints %d runes, must fit 52", widest)
		}
	}
	m.asciiTransport = true
	rows2 := strings.Split(strings.TrimSuffix(m.View(), "\n"), "\n")
	asciiLine := stripANSI(strings.Join(rows2[20:23], "\n"))
	if runes := maxRuneWidth(asciiLine); runes > 52 {
		t.Fatalf("ASCII keycaps row paints %d runes, must fit 52", runes)
	}
}

// maxRuneWidth reports the widest line (rune count) of a multi-line strip.
func maxRuneWidth(s string) int {
	widest := 0
	for _, line := range strings.Split(s, "\n") {
		if n := len([]rune(line)); n > widest {
			widest = n
		}
	}
	return widest
}
