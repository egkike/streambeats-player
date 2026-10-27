package tui

// Bubbletea model for the StreamBeats player. The model is deliberately
// split into pure state transitions (Update + helpers) and thin commands that
// perform side effects (network resolve, mpv IPC) so the browsing and
// playback state machine can be unit tested without a network or mpv.

import (
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/charmbracelet/bubbles/key"

	tea "github.com/charmbracelet/bubbletea"

	"streambeats/internal/catalog"
	"streambeats/internal/player"
	"streambeats/internal/settings"
	"streambeats/internal/theme"
)

// Player is the playback surface the TUI drives. *player.Client implements it.
type Player interface {
	LoadFile(path, title string) error
	Play() error
	Pause() error
	Stop() error
	Seek(seconds float64) error
	SetVolume(percent int) error
	GetProperty(prop string) (float64, error)
	Close() error
	Quit() error

	// Events streams mpv lifecycle signals; ObserveIdleActive subscribes to
	// the idle-active property per DOCS/man/ipc.rst.
	Events() <-chan player.Event
	ObserveIdleActive() error
}

// Resolver turns a catalog track into a fresh, playable stream URL.
// *catalog.Client implements it.
type Resolver interface {
	ResolveStream(albumSlug, trackSlug string) (string, error)
}

// Mode selects which list the user is browsing.
type Mode int

const (
	// ModeCatalog browses the scraped StreamBeats catalog.
	ModeCatalog Mode = iota
	// ModeLibrary browses mp3 files already downloaded on disk.
	ModeLibrary
	// ModeConfig shows the settings screen (theme selection, Phase A).
	ModeConfig
)

const (
	defaultVolume = 100
	// statusFeedbackTicks is the expiry window for feedback statuses (T2/T3,
	// issue #40): ~4 ticks at the 1s cadence. The progress poll may only
	// clear a status after the window has passed, so feedback is visible.
	statusFeedbackTicks = 4
	volumeStep          = 5

	// tickInterval is how often playback progress is polled from mpv.
	tickInterval = time.Second

	// progressErrorThreshold is how many consecutive poll failures are
	// tolerated before a status line is shown (transient mpv errors are
	// normal right after loadfile).
	progressErrorThreshold = 3

	// defaultListHeight is used until the first WindowSizeMsg arrives.
	defaultListHeight = 12
)

// errNoPlayer is reported when a playback action needs mpv but none is
// connected. It is surfaced as a status line, never as a crash.
var errNoPlayer = errors.New("mpv is not connected")

// Options configures a Model.
type Options struct {
	Catalog       *catalog.Catalog
	LibraryFiles  []string
	LibraryRoot   string
	Player        Player // nil when mpv is unavailable
	Resolver      Resolver
	PlayerNote    string // friendly note shown when mpv could not be reached
	Width         int
	Height        int
	Theme         string          // raw theme id from settings; "" = default
	ThemeDark     bool            // resolved background detection result
	SettingsStore *settings.Store // nil disables persistence (tests, alt runtimes)
	// Debug, when set, records every received tea.Msg for diagnostic
	// sessions (STREAMBEATS_DEBUG_INPUT; never active in normal runs).
	Debug func(msg string)
}

// Model is the TUI state.
type Model struct {
	albums  []catalog.AlbumEntry
	player  Player
	resolve Resolver

	library       []string
	libraryRoot   string
	mode          Mode
	inTracks      bool
	albumCursor   int
	trackCursor   int
	libraryCursor int
	// configIndex/configReturn drive the settings screen: the selection
	// index over the theme list and the screen to go back to.
	configIndex  int
	configReturn Mode

	nowPlaying string
	playing    bool
	paused     bool
	volume     int
	position   float64
	duration   float64
	// Bookkeeping for end-of-track auto-advance: which album/track/library
	// item is currently playing (-1 when none). The basis is the played
	// source itself, not the browsed list, so navigation while playing
	// cannot derail the queue.
	playingAlbum int
	playingTrack int
	playingLib   int

	// prefetch caches a freshly resolved stream URL for the NEXT track,
	// filled right after a load succeeds so the eventual skip needs no
	// wait; prefetchSlug names the track it belongs to.
	prefetchSlug string
	prefetchURL  string
	// progressMisses counts consecutive poll failures; mpv reports transient
	// "property unavailable" right after a load, so status needs debouncing.
	progressMisses int

	// palette is the resolved theme palette (Phase A): every style the
	// view derives comes from it, so theme switches are pure state.
	palette theme.Palette
	// theme is the resolved theme id the palette came from; the config
	// screen marks it as current and settings persists it.
	theme         theme.Theme
	settingsStore *settings.Store
	// debug records received messages when the diagnostic kit is on.
	debug func(msg string)
	// debug records received messages when the diagnostic kit is on.
	// resolved with, so theme switches keep System's contrast pairing.
	darkBackground bool

	status          string
	statusClearTick int
	tick            int
	width           int
	height          int
	quitting        bool
}

// NewModel builds a model from already-loaded data. No I/O happens here.
func NewModel(opts Options) Model {
	m := Model{
		player:        opts.Player,
		resolve:       opts.Resolver,
		library:       append([]string(nil), opts.LibraryFiles...),
		libraryRoot:   opts.LibraryRoot,
		volume:        defaultVolume,
		playingAlbum:  -1,
		playingTrack:  -1,
		playingLib:    -1,
		width:         opts.Width,
		height:        opts.Height,
		status:        opts.PlayerNote,
		settingsStore: opts.SettingsStore,
		debug:         opts.Debug,
	}
	if opts.Catalog != nil {
		m.albums = append([]catalog.AlbumEntry(nil), opts.Catalog.Albums...)
	}
	// Theme resolution: an unknown/empty id falls back to the default
	// theme, so callers never need to pre-validate (S4).
	if t, ok := theme.Lookup(opts.Theme); ok {
		m.theme = t
	} else {
		m.theme = theme.Default
	}
	m.darkBackground = opts.ThemeDark
	m.palette = theme.Resolve(m.theme, opts.ThemeDark)
	m.configIndex = configIndexFor(m.theme)
	if len(m.albums) == 0 {
		m.status = joinStatus(m.status, "catalog is empty — run `streambeats refresh`")
	}
	return m
}

// Init starts the progress ticker when an audio backend is connected.
func (m Model) Init() tea.Cmd {
	if m.player == nil {
		return nil
	}
	return tea.Batch(tickCmd(), waitEventsCmd(m.player))
}

// Cronological messages produced by side-effect commands.
type (
	tickMsg   struct{}
	loadedMsg struct {
		title string
		err   error
	}
	actionMsg struct {
		err error
	}
	progressMsg struct {
		pos float64
		dur float64
		err error
	}
	eventMsg struct {
		event player.Event
	}
	volumeMsg struct {
		volume int
		err    error
	}
	prefetchMsg struct {
		slug string
		url  string
		err  error
	}
)

// Update implements tea.Model. It never performs I/O itself.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Diagnostic kit (never active in normal runs): one line per message.
	if m.debug != nil {
		m.debug(fmt.Sprintf("%T %+v | h=%d w=%d mode=%v", msg, msg, m.height, m.width, m.mode))
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)
	case tea.MouseMsg:
		// Mouse dispatch OFF (owner decision: keyboard-only panel). The
		// transport row is a visual indicator; the keybinds do the work.
		_ = msg
		return m, nil
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tickMsg:
		m.tick++
		if m.status != "" && m.tick >= m.statusClearTick {
			m.status = "" // feedback expired (T3)
			m.statusClearTick = 0
		}
		if m.player == nil {
			return m, nil
		}
		cmds := []tea.Cmd{tickCmd()}
		if m.playing && !m.paused {
			cmds = append(cmds, pollCmd(m.player))
		}
		return m, tea.Batch(cmds...)
	case loadedMsg:
		if msg.err != nil {
			m.playing = false
			m.paused = false
			m.status = friendlyError("playback failed", msg.err)
			return m, nil
		}
		m.nowPlaying = msg.title
		m.playing = true
		m.paused = false
		m.position, m.duration = 0, 0
		m.progressMisses = 0
		m.status = ""
		// The browse cursor follows playback so the ♪/▸ markers stay in sync
		// after auto-advances and skips (user report L31).
		if m.mode == ModeLibrary {
			if m.playingLib >= 0 && m.playingLib < len(m.library) {
				m.libraryCursor = m.playingLib
			}
		} else if m.playingAlbum >= 0 && m.playingAlbum < len(m.albums) {
			m.albumCursor = m.playingAlbum
			m.trackCursor = m.playingTrack
			m.inTracks = true
		}
		// Guarantee the ticker after every load (advances re-arm it too),
		// and prefetch the next track's fresh stream URL for instant skips.
		nextCmds := []tea.Cmd{tickCmd()}
		if pm, ok := m.prefetchNextCmd(); ok {
			nextCmds = append(nextCmds, pm)
		}
		return m, tea.Batch(nextCmds...)
	case actionMsg:
		if msg.err != nil {
			m.status = friendlyError("mpv error", msg.err)
		}
		return m, nil
	case eventMsg:
		// Re-arm the listener regardless of outcome.
		rearm := []tea.Cmd{waitEventsCmd(m.player)}
		// The single queue-advance trigger (per DOCS/man/ipc.rst:
		// observe_property + idle-active). Neither buffering underruns nor
		// the loadfile transition can flip idle-active, so this never fires
		// for a mere slow source — only for a truly finished one.
		if msg.event.Name == "property-change" && msg.event.Property == "idle-active" &&
			msg.event.Bool != nil && *msg.event.Bool &&
			m.playing && !m.paused {
			mNext, cmd := m.autoAdvance()
			if cmd != nil {
				rearm = []tea.Cmd{cmd, waitEventsCmd(m.player)}
			}
			return mNext, tea.Batch(rearm...)
		}
		return m, tea.Batch(rearm...)
	case prefetchMsg:
		if msg.err == nil {
			m.prefetchSlug, m.prefetchURL = msg.slug, msg.url
		}
		return m, nil
	case saveMsg:
		// Persistence feedback (Phase A): the failure keeps working and
		// never blocks the TUI (S4).
		if msg.err != nil {
			m.status = friendlyError("could not save settings", msg.err)
		} else {
			m.status = "theme saved"
		}
		return m, nil
	case progressMsg:
		if msg.err != nil {
			m.progressMisses++
			// Poll errors NEVER advance the queue and NEVER form a banner:
			// transient buffering/loadfile gaps are pure noise (the user
			// asked for a silent transition). The queue moving trigger is
			// idle-active per docs; the misses counter remains only as
			// potential diagnostics fuel.
			return m, nil
		}
		m.progressMisses = 0
		// A real progress reading proves the source is live: transient
		// property banners (mpv not reporting duration while buffering)
		// clear right here — but NEVER a fresh feedback status within its
		// window (issue #40 S4).
		if m.status == "" || m.tick >= m.statusClearTick {
			m.status = ""
		}
		m.position, m.duration = msg.pos, msg.dur
		return m, nil
	case volumeMsg:
		if msg.err != nil {
			m.setStatus(friendlyError("mpv error", msg.err))
			return m, nil
		}
		// mpv-reported volume is untrusted input for the UI math (R4-001):
		// clamp on receipt exactly like our own writes do, then state the
		// level honestly (issue #40 S1: the volume click/receipt used to be
		// silent).
		m.volume = clampVolume(msg.volume)
		m.setStatus(fmt.Sprintf("volume %d%%", m.volume))
		return m, nil
	}
	return m, nil
}

// handleKey maps one keystroke to a pure state change plus optional command.
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// The settings screen owns the keyboard while open: only navigation,
	// commit and return act (no playback, no queue mutations).
	if m.mode == ModeConfig {
		return m.configKey(msg)
	}
	switch {
	case key.Matches(msg, keys.QueueNext):
		return m.queueNext()
	case key.Matches(msg, keys.QueuePrev):
		return m.queuePrev()
	case key.Matches(msg, keys.Quit):
		m.quitting = true
		if m.player == nil {
			return m, tea.Quit
		}
		// Stop playback and let mpv exit BEFORE the program tears down, so
		// audio never outlives the panel.
		return m, tea.Sequence(quitPlayerCmd(m.player), tea.Quit)
	case key.Matches(msg, keys.Up):
		m.moveCursor(-1)
	case key.Matches(msg, keys.Down):
		m.moveCursor(1)
	case key.Matches(msg, keys.Enter):
		return m.activateSelection()
	case key.Matches(msg, keys.Back):
		m.back()
	case key.Matches(msg, keys.Library):
		m.toggleMode()
	case key.Matches(msg, keys.Config):
		m.openConfig()
	case key.Matches(msg, keys.Pause):
		return m.togglePause()
	case key.Matches(msg, keys.Stop):
		return m.stopPlayback()
	case key.Matches(msg, keys.VolumeUp):
		return m.changeVolume(volumeStep)
	case key.Matches(msg, keys.VolumeDown):
		return m.changeVolume(-volumeStep)
	}
	return m, nil
}

// saveMsg closes the theme persistence cycle: the command ran outside the
// update loop and reports either way.
type saveMsg struct{ err error }

// saveThemeCmd persists the chosen theme to the settings store.
func saveThemeCmd(store settings.Store, chosen theme.Theme) tea.Cmd {
	return func() tea.Msg {
		return saveMsg{err: store.Save(settings.Settings{Theme: chosen.String()})}
	}
}

// openConfig enters the settings screen, remembering where the user came
// from so esc returns there.
func (m *Model) openConfig() {
	m.configReturn = m.mode
	m.configIndex = configIndexFor(m.theme)
	m.mode = ModeConfig
}

// setStatus records panel feedback and arms its expiry window: the value
// survives the next progress reading and clears itself after
// statusFeedbackTicks ticks (issue #40 S1/S3/S4).
func (m *Model) setStatus(text string) {
	if text == "" {
		return
	}
	m.status = text
	m.statusClearTick = m.tick + statusFeedbackTicks
}

// moveCursor clamps and updates the cursor of whichever list is focused.
func (m *Model) moveCursor(delta int) {
	switch {
	case m.mode == ModeLibrary:
		m.libraryCursor = clampCursor(m.libraryCursor+delta, len(m.library))
	case m.inTracks:
		n := 0
		if album, ok := m.selectedAlbum(); ok {
			n = len(album.Tracks)
		}
		m.trackCursor = clampCursor(m.trackCursor+delta, n)
	default:
		m.albumCursor = clampCursor(m.albumCursor+delta, len(m.albums))
	}
}

// back walks one level up: tracks -> albums, library -> catalog.
func (m *Model) back() {
	switch {
	case m.mode == ModeLibrary:
		m.mode = ModeCatalog
	case m.inTracks:
		m.inTracks = false
		m.trackCursor = 0
	}
}

// toggleMode switches between the catalog and the local library list.
func (m *Model) toggleMode() {
	if m.mode == ModeLibrary {
		m.mode = ModeCatalog
		m.status = ""
		return
	}
	m.mode = ModeLibrary
	m.inTracks = false
	if len(m.library) == 0 {
		m.status = "local library is empty — run `streambeats sync`"
	} else {
		m.status = ""
	}
}

// Config screen data (Phase A): the ordered theme list the config
// screen offers, imported from the theme registry so labels never fork.
var configThemeIDs = []theme.Theme{theme.Claro, theme.Oscuro, theme.System}

// configLabels is the display name for each configThemeIDs entry.
var configLabels = map[theme.Theme]string{
	theme.Claro:  "Claro",
	theme.Oscuro: "Oscuro",
	theme.System: "System",
}

// configIndexFor maps a resolved theme onto its config list position.
func configIndexFor(t theme.Theme) int {
	for i, id := range configThemeIDs {
		if id == t {
			return i
		}
	}
	// R3-002: resolve the fallback directly instead of recursing — if the
	// default ever left the list, the old call decomposed to infinity.
	return 0
}

// configTheme is the theme currently highlighted on the config screen.
func (m Model) configTheme() theme.Theme {
	return configThemeIDs[clampCursor(m.configIndex, len(configThemeIDs))]
}

// configKey runs the settings screen keyboard: navigation, commit and
// return; every playback/browse binding is inert while it is open.
func (m Model) configKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Up):
		m.configIndex = clampCursor(m.configIndex-1, len(configThemeIDs))
	case key.Matches(msg, keys.Down):
		m.configIndex = clampCursor(m.configIndex+1, len(configThemeIDs))
	case key.Matches(msg, keys.Enter):
		return m.applyThemeSelection()
	case key.Matches(msg, keys.Back):
		m.mode = m.configReturn
		if m.configReturn != ModeLibrary {
			m.inTracks = false
		}
	}
	return m, nil
}

// applyThemeSelection commits the highlighted theme: pure state switches
// immediately; persistence runs in a command and reports via message.
func (m Model) applyThemeSelection() (tea.Model, tea.Cmd) {
	chosen := m.configTheme()
	m.theme = chosen
	// The background detection stays whatever the model was built with
	// (darkBackground, set at construction): only the palette id changes.
	m.palette = theme.Resolve(chosen, m.darkBackground)
	m.mode = m.configReturn
	if m.configReturn != ModeLibrary {
		m.inTracks = false
	}
	if m.settingsStore == nil {
		m.status = joinStatus(m.status, chosen.String()+" applied (not saved anywhere)")
		return m, nil
	}
	return m, saveThemeCmd(*m.settingsStore, chosen)
}

// activateSelection is the enter key: open an album, or play the selected
// track/file.
func (m Model) activateSelection() (tea.Model, tea.Cmd) {
	if m.mode == ModeLibrary {
		if len(m.library) == 0 {
			m.status = "local library is empty — run `streambeats sync`"
			return m, nil
		}
		idx := clampCursor(m.libraryCursor, len(m.library))
		m.playingLib = idx
		m.playingAlbum, m.playingTrack = -1, -1
		path := m.library[idx]
		return m, loadFileCmd(m.player, path, libraryEntryTitle(path))
	}

	if !m.inTracks {
		album, ok := m.selectedAlbum()
		if !ok {
			m.status = "no albums in the catalog — run `streambeats refresh`"
			return m, nil
		}
		if len(album.Tracks) == 0 {
			m.status = fmt.Sprintf("album %q has no tracks", album.Title)
			return m, nil
		}
		m.inTracks = true
		m.trackCursor = 0
		return m, m.seedPrefetchCmd(album)
	}

	album, ok := m.selectedAlbum()
	if !ok {
		m.status = "no album selected"
		return m, nil
	}
	track, ok := m.selectedTrack()
	if !ok {
		m.status = "no track selected"
		return m, nil
	}
	if m.player == nil {
		m.status = friendlyError("playback unavailable", errNoPlayer)
		return m, nil
	}
	if m.resolve == nil {
		m.status = "stream resolver unavailable"
		return m, nil
	}
	m.playingAlbum = m.albumCursor
	m.playingTrack = clampCursor(m.trackCursor, len(album.Tracks))
	m.playingLib = -1
	// The fresh resolve takes a visible network roundtrip (Bandcamp tokens
	// expire — invariant 2): state the wait instead of leaving the panel
	// silent. loadedMsg clears it on success and replaces it on failure.
	m.status = "resolving stream…"
	// A prefetch seeded on album open (or on the previous advance) cuts the
	// whole resolve out of this load when it matches the exact track; any
	// mismatch or stale token falls back to a fresh resolve.
	prefetch := ""
	if m.prefetchSlug == track.Slug {
		prefetch = m.prefetchURL
	}
	return m, advanceCatalogCmd(m.player, m.resolve, album, track, prefetch)
}

// togglePause flips the pause state through the player.
func (m Model) togglePause() (tea.Model, tea.Cmd) {
	if m.player == nil {
		m.setStatus(friendlyError("playback unavailable", errNoPlayer))
		return m, nil
	}
	if !m.playing {
		// The play token now BEHAVES like the owner expects (issue #42
		// round-3): with nothing playing, pressing ▶/|> starts the
		// highlighted item. It replays the Enter flow one step at a time
		// (catalog: open the album, then play the highlighted/first track)
		// until playback starts — the SAME command path as Enter (S2
		// parity, no second action logic), looping at most twice.
		next, cmd := m.activateSelection()
		if nm, ok := next.(Model); ok && !nm.playing {
			return nm.activateSelection()
		}
		return next, cmd
	}
	p := m.player
	m.paused = !m.paused
	paused := m.paused
	if paused {
		m.setStatus("paused — space to resume")
	} else {
		m.setStatus("resumed")
	}
	return m, func() tea.Msg {
		var err error
		if paused {
			err = p.Pause()
		} else {
			err = p.Play()
		}
		return actionMsg{err: err}
	}
}

// stopPlayback stops audio and clears the now-playing bar.
func (m Model) stopPlayback() (tea.Model, tea.Cmd) {
	if m.player == nil {
		m.setStatus(friendlyError("playback unavailable", errNoPlayer))
		return m, nil
	}
	if !m.playing {
		m.setStatus("nothing is playing — press enter on a track")
		return m, nil
	}
	p := m.player
	m.playing = false
	m.paused = false
	m.nowPlaying = ""
	m.position, m.duration = 0, 0
	m.setStatus("stopped")
	return m, func() tea.Msg { return actionMsg{err: p.Stop()} }
}

// changeVolume moves the volume by delta percent, clamped to [0, 100]. With a
// connected player it reads the live mpv volume first so external changes are
// honored.
func (m Model) changeVolume(delta int) (tea.Model, tea.Cmd) {
	if m.player == nil {
		m.volume = clampVolume(m.volume + delta)
		m.status = fmt.Sprintf("volume %d%% (mpv not connected)", m.volume)
		return m, nil
	}
	return m, changeVolumeCmd(m.player, delta)
}

// selectedAlbum returns the highlighted album, clamped to the list.
func (m Model) selectedAlbum() (catalog.AlbumEntry, bool) {
	if len(m.albums) == 0 {
		return catalog.AlbumEntry{}, false
	}
	return m.albums[clampCursor(m.albumCursor, len(m.albums))], true
}

// selectedTrack returns the highlighted track of the highlighted album.
func (m Model) selectedTrack() (catalog.Track, bool) {
	album, ok := m.selectedAlbum()
	if !ok || len(album.Tracks) == 0 {
		return catalog.Track{}, false
	}
	return album.Tracks[clampCursor(m.trackCursor, len(album.Tracks))], true
}

// queueNext is the manual '>' skip: shares the auto-advance engine (and its
// in-flight guard) so a slow resolve does not stack jumps.
func (m Model) queueNext() (tea.Model, tea.Cmd) {
	if m.player == nil {
		m.setStatus(friendlyError("playback unavailable", errNoPlayer))
		return m, nil
	}
	if !m.playing {
		m.setStatus("nothing is playing — press enter on a track")
		return m, nil
	}
	return m.autoAdvance()
}

// queuePrev is the manual '<' skip: plays the previous item of the queue,
// restarting the first one when at the top.
func (m Model) queuePrev() (tea.Model, tea.Cmd) {
	if m.player == nil {
		m.status = friendlyError("playback unavailable", errNoPlayer)
		return m, nil
	}
	if !m.playing {
		m.setStatus("nothing is playing — press enter on a track")
		return m, nil
	}
	restart := func() tea.Model {
		m.status = ""
		return m
	}
	if m.mode == ModeLibrary {
		if m.playingLib <= 0 {
			// top of the queue: replay the current item
			if m.playingLib < 0 {
				m.status = "no previous track in the queue"
				return m, nil
			}
			return restart(), loadFileCmd(m.player, m.library[m.playingLib], libraryEntryTitle(m.library[m.playingLib]))
		}
		m.playingLib--
		m.playingAlbum, m.playingTrack = -1, -1
		m.status = ""
		m.progressMisses = 0
		return m, loadFileCmd(m.player, m.library[m.playingLib], libraryEntryTitle(m.library[m.playingLib]))
	}
	if m.playingAlbum < 0 || m.playingAlbum >= len(m.albums) {
		m.status = "no previous track in the queue"
		return m, nil
	}
	// precompute the cross-album target first: failure paths MUST leave the
	// bookkeeping untouched (the L29 regression: -1/-1 on a failed lookup)
	targetAlbum := m.playingAlbum
	targetTrack := m.playingTrack - 1
	for targetTrack < 0 {
		targetAlbum--
		if targetAlbum < 0 {
			m.status = "no previous track in the queue"
			return m, nil
		}
		targetTrack = len(m.albums[targetAlbum].Tracks) - 1
	}
	m.playingAlbum = targetAlbum
	m.playingTrack = targetTrack
	m.playingLib = -1
	m.status = ""
	m.progressMisses = 0
	album := m.albums[m.playingAlbum]
	return m, advanceCatalogCmd(m.player, m.resolve, album, album.Tracks[m.playingTrack], "")
}

// autoAdvance plays the successor of the source that just finished. It is
// the pure state change + command pair for end-of-track detection; the
// caller must have confirmed the track really ended (poll errors exhausted
// a live playback, not a transient hiccup).
// A minimum interval between two auto-advances fences the queue against any
// delayed error/event storm: if a wave arrives ahead of schedule, the ship
// waits (the stale wave is swallowed) instead of machine-gunning through the
// track list.
func (m Model) autoAdvance() (tea.Model, tea.Cmd) {
	clear := func() tea.Model {
		m.playing, m.paused = false, false
		m.status = "playlist ended"
		return m
	}
	if m.mode == ModeLibrary {
		if m.playingLib < 0 || m.playingLib+1 >= len(m.library) {
			return clear(), nil
		}
		m.playingLib++
		m.playingAlbum, m.playingTrack = -1, -1
		next := m.library[m.playingLib]
		m.status = ""
		return m, loadFileCmd(m.player, next, libraryEntryTitle(next))
	}
	if m.player == nil || m.resolve == nil || m.playingAlbum < 0 || m.playingAlbum >= len(m.albums) {
		return clear(), nil
	}
	album := m.albums[m.playingAlbum]
	if m.playingTrack+1 >= len(album.Tracks) {
		// cross forward: next non-empty album's first track; keep the
		// playlist-ended stop once the catalog is exhausted
		next := -1
		for a := m.playingAlbum + 1; a < len(m.albums); a++ {
			if len(m.albums[a].Tracks) > 0 {
				next = a
				break
			}
		}
		if next == -1 {
			return clear(), nil
		}
		m.playingAlbum = next
		m.playingTrack = 0
		album = m.albums[next]
	} else {
		m.playingTrack++
	}
	m.playingLib = -1
	m.status = ""
	track := album.Tracks[m.playingTrack]
	// A valid prefetch for the exact next track cuts the whole resolve
	// roundtrip out of the jump; fall back to a fresh resolve on any
	// stale-token failure.
	prefetch := ""
	if m.prefetchSlug == track.Slug {
		prefetch = m.prefetchURL
	}
	return m, advanceCatalogCmd(m.player, m.resolve, album, track, prefetch)
}

// --- commands (side effects) -------------------------------------------

// tickCmd schedules the next progress poll.
func tickCmd() tea.Cmd {
	return tea.Tick(tickInterval, func(time.Time) tea.Msg { return tickMsg{} })
}

// pollCmd reads playback progress from mpv.
func pollCmd(p Player) tea.Cmd {
	return func() tea.Msg {
		dur, err := p.GetProperty("duration")
		if err != nil {
			return progressMsg{err: err}
		}
		pos, err := p.GetProperty("time-pos")
		if err != nil {
			return progressMsg{err: err}
		}
		return progressMsg{pos: pos, dur: dur}
	}
}

// waitEventsCmd parks a goroutine on the mpv event channel (nil-safe:
// a backing nil channel would block forever, so bail out early).
func waitEventsCmd(p Player) tea.Cmd {
	if p == nil {
		return nil
	}
	ch := p.Events()
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		ev := <-ch
		return eventMsg{event: ev}
	}
}

// quitMsg marks the graceful playback teardown completion on quit.
type quitMsg struct{}

// quitPlayerCmd stops the source, asks mpv to exit, and releases the socket.
// Errors are swallowed: quitting must always succeed, audible ghosts must
// never survive the panel.
func quitPlayerCmd(p Player) tea.Cmd {
	return func() tea.Msg {
		_ = p.Stop()
		_ = p.Quit()
		_ = p.Close()
		return quitMsg{}
	}
}

// prefetchNextCmd resolves the URL of the successor of the currently playing
// catalog track, so the eventual end-of-track jump needs no wait.
func (m Model) prefetchNextCmd() (tea.Cmd, bool) {
	if m.player == nil || m.resolve == nil || m.mode != ModeCatalog ||
		m.playingAlbum < 0 || m.playingAlbum >= len(m.albums) {
		return nil, false
	}
	album := m.albums[m.playingAlbum]
	next := m.playingTrack + 1
	if next >= len(album.Tracks) {
		return nil, false
	}
	return prefetchCmdFor(m.resolve, album, album.Tracks[next]), true
}

// prefetchCmdFor builds the background resolve for one catalog track.
func prefetchCmdFor(r Resolver, album catalog.AlbumEntry, track catalog.Track) tea.Cmd {
	return func() tea.Msg {
		url, err := r.ResolveStream(album.Slug, track.Slug)
		if err != nil {
			return prefetchMsg{err: err}
		}
		return prefetchMsg{slug: track.Slug, url: url}
	}
}

// advanceCatalogCmd loads track via a prefetched URL when it matches, with
// a one-shot fresh resolve fallback for expired tokens.
func advanceCatalogCmd(p Player, r Resolver, album catalog.AlbumEntry, track catalog.Track, prefetched string) tea.Cmd {
	return func() tea.Msg {
		title := trackTitle(track)
		if prefetched != "" {
			if err := playFile(p, prefetched, title); err == nil {
				return loadedMsg{title: title}
			}
		}
		url, err := r.ResolveStream(album.Slug, track.Slug)
		if err != nil {
			return loadedMsg{err: err}
		}
		if err := playFile(p, url, title); err != nil {
			return loadedMsg{title: title, err: err}
		}
		return loadedMsg{title: title}
	}
}

// changeVolumeCmd reads the current mpv volume, adds delta, clamps and writes
// it back.
func changeVolumeCmd(p Player, delta int) tea.Cmd {
	return func() tea.Msg {
		current, err := p.GetProperty("volume")
		if err != nil {
			return volumeMsg{err: err}
		}
		next := clampVolume(int(math.Round(current)) + delta)
		if err := p.SetVolume(next); err != nil {
			return volumeMsg{err: err}
		}
		return volumeMsg{volume: next}
	}
}

// seedPrefetchCmd resolves the first track of a just-opened album in the
// background so the eventual first play needs no network wait (issue #62).
// It fires once per album open — never on cursor movement (Bandcamp
// courtesy: every fetch is a full album page) — and only when playback is
// actually available. When the prefetch already matches, it does nothing.
func (m Model) seedPrefetchCmd(album catalog.AlbumEntry) tea.Cmd {
	if m.player == nil || m.resolve == nil || len(album.Tracks) == 0 {
		return nil
	}
	track := album.Tracks[0]
	if m.prefetchSlug == track.Slug {
		return nil
	}
	return prefetchCmdFor(m.resolve, album, track)
}

// loadFileCmd plays a local file directly, without any stream resolution.
func loadFileCmd(p Player, path, title string) tea.Cmd {
	return func() tea.Msg {
		if err := playFile(p, path, title); err != nil {
			return loadedMsg{title: title, err: err}
		}
		return loadedMsg{title: title}
	}
}

// playFile loads and starts one source through the player.
func playFile(p Player, path, title string) error {
	if p == nil {
		return errNoPlayer
	}
	if err := p.LoadFile(path, title); err != nil {
		return err
	}
	return p.Play()
}
