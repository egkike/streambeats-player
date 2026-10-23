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
)

const (
	defaultVolume = 100
	volumeStep    = 5

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
	Catalog      *catalog.Catalog
	LibraryFiles []string
	LibraryRoot  string
	Player       Player // nil when mpv is unavailable
	Resolver     Resolver
	PlayerNote   string // friendly note shown when mpv could not be reached
	Width        int
	Height       int
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

	nowPlaying string
	playing    bool
	paused     bool
	volume     int
	position   float64
	duration   float64
	// progressMisses counts consecutive poll failures; mpv reports transient
	// "property unavailable" right after a load, so status needs debouncing.
	progressMisses int

	status   string
	width    int
	height   int
	quitting bool
}

// NewModel builds a model from already-loaded data. No I/O happens here.
func NewModel(opts Options) Model {
	m := Model{
		player:      opts.Player,
		resolve:     opts.Resolver,
		library:     append([]string(nil), opts.LibraryFiles...),
		libraryRoot: opts.LibraryRoot,
		volume:      defaultVolume,
		width:       opts.Width,
		height:      opts.Height,
		status:      opts.PlayerNote,
	}
	if opts.Catalog != nil {
		m.albums = append([]catalog.AlbumEntry(nil), opts.Catalog.Albums...)
	}
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
	return tickCmd()
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
	volumeMsg struct {
		volume int
		err    error
	}
)

// Update implements tea.Model. It never performs I/O itself.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tickMsg:
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
		return m, nil
	case actionMsg:
		if msg.err != nil {
			m.status = friendlyError("mpv error", msg.err)
		}
		return m, nil
	case progressMsg:
		if msg.err != nil {
			m.progressMisses++
			if m.progressMisses >= progressErrorThreshold {
				m.status = friendlyError("mpv error", msg.err)
			}
			return m, nil
		}
		m.progressMisses = 0
		m.position, m.duration = msg.pos, msg.dur
		return m, nil
	case volumeMsg:
		if msg.err != nil {
			m.status = friendlyError("mpv error", msg.err)
			return m, nil
		}
		m.volume = msg.volume
		return m, nil
	}
	return m, nil
}

// handleKey maps one keystroke to a pure state change plus optional command.
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Quit):
		m.quitting = true
		return m, tea.Quit
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

// activateSelection is the enter key: open an album, or play the highlighted
// track/file.
func (m Model) activateSelection() (tea.Model, tea.Cmd) {
	if m.mode == ModeLibrary {
		if len(m.library) == 0 {
			m.status = "local library is empty — run `streambeats sync`"
			return m, nil
		}
		path := m.library[clampCursor(m.libraryCursor, len(m.library))]
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
		return m, nil
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
	return m, resolveAndPlayCmd(m.player, m.resolve, album, track)
}

// togglePause flips the pause state through the player.
func (m Model) togglePause() (tea.Model, tea.Cmd) {
	if m.player == nil {
		m.status = friendlyError("playback unavailable", errNoPlayer)
		return m, nil
	}
	if !m.playing {
		m.status = "nothing is playing — press enter on a track"
		return m, nil
	}
	p := m.player
	m.paused = !m.paused
	paused := m.paused
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
		m.status = friendlyError("playback unavailable", errNoPlayer)
		return m, nil
	}
	if !m.playing {
		return m, nil
	}
	p := m.player
	m.playing = false
	m.paused = false
	m.nowPlaying = ""
	m.position, m.duration = 0, 0
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

// strategy: resolve first, then load, so the player never sees a stale URL.
func resolveAndPlayCmd(p Player, r Resolver, album catalog.AlbumEntry, track catalog.Track) tea.Cmd {
	return func() tea.Msg {
		url, err := r.ResolveStream(album.Slug, track.Slug)
		if err != nil {
			return loadedMsg{err: err}
		}
		title := trackTitle(track)
		if err := playFile(p, url, title); err != nil {
			return loadedMsg{title: title, err: err}
		}
		return loadedMsg{title: title}
	}
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
