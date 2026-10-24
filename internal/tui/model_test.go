package tui

// Unit tests for the pure model logic: cursor clamping, mode transitions and
// the command/message round trips. Fakes stand in for mpv and the catalog so
// no test touches the network or a real player.

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"streambeats/internal/catalog"
)

// fakePlayer records playback calls instead of talking to mpv.
type fakePlayer struct {
	loaded     []string
	titles     []string
	volumes    []int
	playCalls  int
	pauseCalls int
	stopCalls  int

	loadErr error
	props   map[string]float64
	propErr error
}

func (f *fakePlayer) LoadFile(path, title string) error {
	if f.loadErr != nil {
		return f.loadErr
	}
	f.loaded = append(f.loaded, path)
	f.titles = append(f.titles, title)
	return nil
}

func (f *fakePlayer) Play() error        { f.playCalls++; return nil }
func (f *fakePlayer) Pause() error       { f.pauseCalls++; return nil }
func (f *fakePlayer) Stop() error        { f.stopCalls++; return nil }
func (f *fakePlayer) Seek(float64) error { return nil }
func (f *fakePlayer) SetVolume(percent int) error {
	f.volumes = append(f.volumes, percent)
	return nil
}

func (f *fakePlayer) GetProperty(prop string) (float64, error) {
	if f.propErr != nil {
		return 0, f.propErr
	}
	if v, ok := f.props[prop]; ok {
		return v, nil
	}
	return 0, nil
}

func (f *fakePlayer) Close() error { return nil }

// fakeResolver returns a canned stream URL instead of scraping Bandcamp.
type fakeResolver struct {
	url   string
	err   error
	calls []string
}

func (f *fakeResolver) ResolveStream(albumSlug, trackSlug string) (string, error) {
	f.calls = append(f.calls, albumSlug+"/"+trackSlug)
	if f.err != nil {
		return "", f.err
	}
	return f.url, nil
}

func testCatalog() *catalog.Catalog {
	return &catalog.Catalog{Albums: []catalog.AlbumEntry{
		{
			Slug:   "album-one",
			Title:  "Album One",
			Artist: "StreamBeats",
			Tracks: []catalog.Track{
				{Title: "First Song", Slug: "first-song", DurationSeconds: 125},
				{Title: "Second Song", Slug: "second-song", DurationSeconds: 61},
			},
		},
		{
			Slug:   "album-two",
			Title:  "Album Two",
			Artist: "StreamBeats",
			Tracks: []catalog.Track{{Title: "Third Song", Slug: "third-song"}},
		},
	}}
}

func testModel(opts Options) Model {
	if opts.Catalog == nil {
		opts.Catalog = testCatalog()
	}
	return NewModel(opts)
}

// update applies one message and returns the concrete model plus command.
func update(t *testing.T, m Model, msg tea.Msg) (Model, tea.Cmd) {
	t.Helper()
	next, cmd := m.Update(msg)
	model, ok := next.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want tui.Model", next)
	}
	return model, cmd
}

// runCmd executes a command and returns its message (nil-safe).
func runCmd(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected a command, got nil")
	}
	return cmd()
}

// press sends a rune keystroke through Update.
func press(t *testing.T, m Model, r rune) (Model, tea.Cmd) {
	t.Helper()
	return update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
}

func TestClampCursor(t *testing.T) {
	tests := []struct {
		name   string
		cursor int
		n      int
		want   int
	}{
		{"empty list", 3, 0, 0},
		{"negative", -4, 5, 0},
		{"inside", 3, 5, 3},
		{"last", 4, 5, 4},
		{"past end", 9, 5, 4},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := clampCursor(tc.cursor, tc.n); got != tc.want {
				t.Fatalf("clampCursor(%d, %d) = %d, want %d", tc.cursor, tc.n, got, tc.want)
			}
		})
	}
}

func TestClampVolume(t *testing.T) {
	tests := []struct {
		in   int
		want int
	}{
		{-10, 0},
		{0, 0},
		{42, 42},
		{100, 100},
		{105, 100},
	}
	for _, tc := range tests {
		if got := clampVolume(tc.in); got != tc.want {
			t.Fatalf("clampVolume(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestCursorNavigationClamps(t *testing.T) {
	m := testModel(Options{})
	if m.albumCursor != 0 {
		t.Fatalf("initial album cursor = %d, want 0", m.albumCursor)
	}

	var cmd tea.Cmd
	m, _ = press(t, m, 'k') // up at the top is a no-op
	if m.albumCursor != 0 {
		t.Fatalf("cursor after k = %d, want 0", m.albumCursor)
	}
	for i := 0; i < 5; i++ {
		m, cmd = press(t, m, 'j')
		if cmd != nil {
			t.Fatalf("navigation should not emit commands, got %T", cmd)
		}
	}
	if m.albumCursor != 1 {
		t.Fatalf("cursor after repeated j = %d, want 1 (clamped)", m.albumCursor)
	}

	// Enter opens the album, then j/k walk the track list with clamping.
	m, _ = press(t, m, 'k') // back to album one, which has two tracks
	m, _ = update(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if !m.inTracks {
		t.Fatal("enter did not open the track list")
	}
	m, _ = press(t, m, 'j')
	m, _ = press(t, m, 'j')
	if m.trackCursor != 1 {
		t.Fatalf("track cursor = %d, want 1 (clamped)", m.trackCursor)
	}
	m, _ = press(t, m, 'k')
	m, _ = press(t, m, 'k')
	if m.trackCursor != 0 {
		t.Fatalf("track cursor = %d, want 0 (clamped)", m.trackCursor)
	}

	// Esc walks back to the album list.
	m, _ = update(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.inTracks {
		t.Fatal("esc did not leave the track list")
	}
}

func TestLibraryModeToggle(t *testing.T) {
	m := testModel(Options{LibraryFiles: []string{"/lib/01 - album/01 - first.mp3"}})
	if m.mode != ModeCatalog {
		t.Fatalf("initial mode = %v, want ModeCatalog", m.mode)
	}

	m, _ = press(t, m, 'l')
	if m.mode != ModeLibrary {
		t.Fatalf("mode after l = %v, want ModeLibrary", m.mode)
	}
	if m.status != "" {
		t.Fatalf("unexpected status with files present: %q", m.status)
	}

	// Enter plays the local file directly, without resolving a stream.
	fake := &fakePlayer{}
	resolver := &fakeResolver{url: "https://should-not-be-used"}
	m.player = fake
	m.resolve = resolver
	m, cmd := update(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	msg := runCmd(t, cmd)
	loaded, ok := msg.(loadedMsg)
	if !ok {
		t.Fatalf("command returned %T, want loadedMsg", msg)
	}
	if loaded.err != nil {
		t.Fatalf("loadedMsg err = %v", loaded.err)
	}
	if len(resolver.calls) != 0 {
		t.Fatalf("library playback resolved a stream: %v", resolver.calls)
	}
	if len(fake.loaded) != 1 || fake.loaded[0] != "/lib/01 - album/01 - first.mp3" {
		t.Fatalf("loaded files = %v, want the local path", fake.loaded)
	}
	m, _ = update(t, m, msg)
	if m.nowPlaying != "Album / First" {
		t.Fatalf("nowPlaying = %q, want %q", m.nowPlaying, "Album / First")
	}
	if !m.playing {
		t.Fatal("model should be playing after a successful load")
	}

	m, _ = press(t, m, 'l')
	if m.mode != ModeCatalog {
		t.Fatalf("mode after second l = %v, want ModeCatalog", m.mode)
	}
}

func TestLibraryModeEmptyShowsHint(t *testing.T) {
	m := testModel(Options{})
	m, _ = press(t, m, 'l')
	if !strings.Contains(m.status, "streambeats sync") {
		t.Fatalf("status = %q, want a sync hint", m.status)
	}
	if _, cmd := update(t, m, tea.KeyMsg{Type: tea.KeyEnter}); cmd != nil {
		t.Fatal("enter on an empty library should not emit a command")
	}
}

func TestEnterPlaysTrackWithFreshStream(t *testing.T) {
	fake := &fakePlayer{}
	resolver := &fakeResolver{url: "https://fresh.example/stream"}
	m := testModel(Options{Player: fake, Resolver: resolver})

	m, _ = update(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // open album one
	m, cmd := update(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	msg := runCmd(t, cmd)

	loaded, ok := msg.(loadedMsg)
	if !ok {
		t.Fatalf("command returned %T, want loadedMsg", msg)
	}
	if loaded.err != nil {
		t.Fatalf("loadedMsg err = %v", loaded.err)
	}
	if want := []string{"album-one/first-song"}; len(resolver.calls) != 1 || resolver.calls[0] != want[0] {
		t.Fatalf("resolver calls = %v, want %v", resolver.calls, want)
	}
	if len(fake.loaded) != 1 || fake.loaded[0] != "https://fresh.example/stream" {
		t.Fatalf("loaded = %v, want the freshly resolved URL", fake.loaded)
	}
	if fake.playCalls != 1 {
		t.Fatalf("play calls = %d, want 1", fake.playCalls)
	}
	if len(fake.titles) != 1 || fake.titles[0] != "First Song" {
		t.Fatalf("titles = %v, want [First Song]", fake.titles)
	}

	m, _ = update(t, m, msg)
	if m.nowPlaying != "First Song" || !m.playing || m.paused {
		t.Fatalf("state after load: nowPlaying=%q playing=%v paused=%v", m.nowPlaying, m.playing, m.paused)
	}
	if m.status != "" {
		t.Fatalf("status after successful load = %q, want empty", m.status)
	}
}

func TestResolveErrorBecomesStatus(t *testing.T) {
	fake := &fakePlayer{}
	resolver := &fakeResolver{err: errors.New("bandcamp unreachable")}
	m := testModel(Options{Player: fake, Resolver: resolver})

	m, _ = update(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m, cmd := update(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	msg := runCmd(t, cmd)
	m, _ = update(t, m, msg)

	if m.playing {
		t.Fatal("model must not claim playback after a resolve error")
	}
	if !strings.Contains(m.status, "playback failed") || !strings.Contains(m.status, "bandcamp unreachable") {
		t.Fatalf("status = %q, want a friendly playback error", m.status)
	}
	if len(fake.loaded) != 0 {
		t.Fatalf("player received %v after a resolve error", fake.loaded)
	}
}

func TestPlayerLoadErrorBecomesStatus(t *testing.T) {
	fake := &fakePlayer{loadErr: errors.New("ipc closed")}
	m := testModel(Options{Player: fake, Resolver: &fakeResolver{url: "https://x/stream"}})

	m, _ = update(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m, cmd := update(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = update(t, m, runCmd(t, cmd))

	if m.playing {
		t.Fatal("model must not claim playback after a load error")
	}
	if !strings.Contains(m.status, "ipc closed") {
		t.Fatalf("status = %q, want the load error text", m.status)
	}
}

func TestPlaybackWithoutPlayerIsFriendly(t *testing.T) {
	m := testModel(Options{}) // no player

	m, cmd := update(t, m, tea.KeyMsg{Type: tea.KeySpace})
	if cmd != nil {
		t.Fatal("pause without a player should not emit a command")
	}
	if !strings.Contains(m.status, "mpv") {
		t.Fatalf("status = %q, want an mpv hint", m.status)
	}

	m, cmd = press(t, m, 's')
	if cmd != nil {
		t.Fatal("stop without a player should not emit a command")
	}
	if !strings.Contains(m.status, "mpv") {
		t.Fatalf("status = %q, want an mpv hint", m.status)
	}

	// Play also fails fast instead of resolving a stream for nobody.
	m, _ = update(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m, cmd = update(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("play without a player should not emit a command")
	}
	if !strings.Contains(m.status, "mpv") {
		t.Fatalf("status = %q, want an mpv hint", m.status)
	}
}

func TestPauseResumeAndStop(t *testing.T) {
	fake := &fakePlayer{}
	m := testModel(Options{Player: fake, Resolver: &fakeResolver{url: "https://x/stream"}})
	m, _ = update(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m, cmd := update(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	m, _ = update(t, m, runCmd(t, cmd))

	m, cmd = update(t, m, tea.KeyMsg{Type: tea.KeySpace})
	if !m.paused {
		t.Fatal("space did not pause playback")
	}
	if msg := runCmd(t, cmd); msg != (actionMsg{}) {
		t.Fatalf("pause command returned %#v, want empty actionMsg", msg)
	}
	if fake.pauseCalls != 1 {
		t.Fatalf("pause calls = %d, want 1", fake.pauseCalls)
	}

	m, cmd = update(t, m, tea.KeyMsg{Type: tea.KeySpace})
	if m.paused {
		t.Fatal("space did not resume playback")
	}
	runCmd(t, cmd)
	if fake.pauseCalls != 1 || fake.playCalls != 2 {
		t.Fatalf("calls pause=%d play=%d, want pause=1 play=2", fake.pauseCalls, fake.playCalls)
	}

	m, cmd = press(t, m, 's')
	if m.playing || m.paused || m.nowPlaying != "" {
		t.Fatalf("stop left state: playing=%v paused=%v nowPlaying=%q", m.playing, m.paused, m.nowPlaying)
	}
	runCmd(t, cmd)
	if fake.stopCalls != 1 {
		t.Fatalf("stop calls = %d, want 1", fake.stopCalls)
	}
}

func TestVolumeClamping(t *testing.T) {
	fake := &fakePlayer{props: map[string]float64{"volume": 98}}
	m := testModel(Options{Player: fake})

	m, cmd := press(t, m, '+')
	m, _ = update(t, m, runCmd(t, cmd))
	if m.volume != 100 {
		t.Fatalf("volume = %d, want 100 (clamped)", m.volume)
	}

	fake.props["volume"] = 100
	m, cmd = press(t, m, '+')
	m, _ = update(t, m, runCmd(t, cmd))
	if m.volume != 100 {
		t.Fatalf("volume = %d, want 100 (clamped)", m.volume)
	}

	fake.props["volume"] = 2
	m, cmd = press(t, m, '-')
	m, _ = update(t, m, runCmd(t, cmd))
	if m.volume != 0 {
		t.Fatalf("volume = %d, want 0 (clamped)", m.volume)
	}

	fake.props["volume"] = 0
	m, cmd = press(t, m, '-')
	m, _ = update(t, m, runCmd(t, cmd))
	if m.volume != 0 {
		t.Fatalf("volume = %d, want 0 (clamped)", m.volume)
	}

	want := []int{100, 100, 0, 0}
	if len(fake.volumes) != len(want) {
		t.Fatalf("SetVolume calls = %v, want %v", fake.volumes, want)
	}
	for i := range want {
		if fake.volumes[i] != want[i] {
			t.Fatalf("SetVolume calls = %v, want %v", fake.volumes, want)
		}
	}
}

func TestVolumeReadErrorBecomesStatus(t *testing.T) {
	fake := &fakePlayer{propErr: errors.New("property unavailable")}
	m := testModel(Options{Player: fake})

	_, cmd := press(t, m, '+')
	m, _ = update(t, m, runCmd(t, cmd))
	if !strings.Contains(m.status, "property unavailable") {
		t.Fatalf("status = %q, want the volume read error", m.status)
	}
	if len(fake.volumes) != 0 {
		t.Fatalf("SetVolume should not run after a read error, got %v", fake.volumes)
	}
}

func TestQuit(t *testing.T) {
	m := testModel(Options{})
	m, cmd := press(t, m, 'q')
	if !m.quitting {
		t.Fatal("q did not mark the model as quitting")
	}
	if msg := runCmd(t, cmd); msg != tea.Quit() {
		t.Fatalf("quit command returned %#v, want tea.QuitMsg", msg)
	}
}

func TestTickWithoutPlayerDoesNothing(t *testing.T) {
	m := testModel(Options{})
	if cmd := m.Init(); cmd != nil {
		t.Fatal("Init should not tick without a player")
	}
	m, cmd := update(t, m, tickMsg{})
	if cmd != nil {
		t.Fatalf("tick without a player returned %T, want nil", cmd)
	}
}

func TestTickPollsProgressWhilePlaying(t *testing.T) {
	fake := &fakePlayer{props: map[string]float64{"time-pos": 12, "duration": 200}}
	m := testModel(Options{Player: fake})
	m.playing = true

	if cmd := m.Init(); cmd == nil {
		t.Fatal("Init should schedule a tick when a player is connected")
	}

	_, cmd := update(t, m, tickMsg{})
	msg := runCmd(t, cmd)
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("tick returned %T, want tea.BatchMsg", msg)
	}
	if len(batch) != 2 {
		t.Fatalf("tick batch size = %d, want 2 (next tick + poll)", len(batch))
	}

	var progress progressMsg
	for _, c := range batch {
		if c == nil {
			continue
		}
		if p, ok := c().(progressMsg); ok {
			progress = p
		}
	}
	if progress.err != nil {
		t.Fatalf("progress error = %v", progress.err)
	}
	if progress.pos != 12 || progress.dur != 200 {
		t.Fatalf("progress = %+v, want pos=12 dur=200", progress)
	}

	m, _ = update(t, m, progress)
	if m.position != 12 || m.duration != 200 {
		t.Fatalf("model progress = %v/%v, want 12/200", m.position, m.duration)
	}
}

func TestProgressErrorBecomesStatus(t *testing.T) {
	m := testModel(Options{Player: &fakePlayer{}})

	// One-off property errors are normal right after loadfile: no status yet.
	m, _ = update(t, m, progressMsg{err: errors.New("broken pipe")})
	if m.status != "" {
		t.Fatalf("status after a single poll error = %q, want empty", m.status)
	}
	m, _ = update(t, m, progressMsg{err: errors.New("broken pipe")})
	if m.status != "" {
		t.Fatalf("status after two poll errors = %q, want empty", m.status)
	}

	m, _ = update(t, m, progressMsg{err: errors.New("broken pipe")})
	if !strings.Contains(m.status, "broken pipe") {
		t.Fatalf("status = %q, want the progress error after %d misses", m.status, progressErrorThreshold)
	}
}

func TestProgressRecoveryResetsMisses(t *testing.T) {
	m := testModel(Options{Player: &fakePlayer{}})
	m, _ = update(t, m, progressMsg{err: errors.New("temporary")})
	m, _ = update(t, m, progressMsg{pos: 1, dur: 60})
	if m.progressMisses != 0 || m.status != "" {
		t.Fatalf("progress misses = %d, status = %q; want a clean reset", m.progressMisses, m.status)
	}
}

func TestEnterOnEmptyCatalogShowsStatus(t *testing.T) {
	m := NewModel(Options{})
	if !strings.Contains(m.status, "streambeats refresh") {
		t.Fatalf("status = %q, want a refresh hint", m.status)
	}
	if _, cmd := update(t, m, tea.KeyMsg{Type: tea.KeyEnter}); cmd != nil {
		t.Fatal("enter on an empty catalog should not emit a command")
	}
}

func TestPlayerNoteSurfacesInStatus(t *testing.T) {
	m := NewModel(Options{PlayerNote: "audio disabled: mpv not found"})
	if !strings.Contains(m.status, "mpv not found") {
		t.Fatalf("status = %q, want the player note", m.status)
	}
}

func TestViewRendersWithoutPanicking(t *testing.T) {
	m := testModel(Options{LibraryFiles: []string{"/lib/01 - album/01 - first.mp3"}})
	for _, msg := range []tea.Msg{
		tea.WindowSizeMsg{Width: 80, Height: 24},
		tea.KeyMsg{Type: tea.KeyEnter},
		tickMsg{},
		progressMsg{pos: 5, dur: 10},
	} {
		m, _ = update(t, m, msg)
	}

	m.playing = true
	m.nowPlaying = "First Song"
	out := m.View()
	for _, want := range []string{"streambeats", "First Song", "▶", "0:05 / 0:10", "space: pause/resume"} {
		if !strings.Contains(out, want) {
			t.Fatalf("catalog view is missing %q:\n%s", want, out)
		}
	}

	m.mode = ModeLibrary
	out = m.View()
	for _, want := range []string{"Local library (1 files)", "First"} {
		if !strings.Contains(out, want) {
			t.Fatalf("library view is missing %q:\n%s", want, out)
		}
	}
}

// --- end-of-track auto-advance (L13) ---------------------------------------

// playingCatalogModel activates track trackIdx of album one so the model's
// playing* bookkeeping is set through the real key flow.
func playingCatalogModel(t *testing.T, trackIdx int) Model {
	t.Helper()
	m := testModel(Options{
		Player:   &fakePlayer{},
		Resolver: &fakeResolver{url: "https://stream.example/1"},
	})
	m, _ = update(t, m, tea.KeyMsg{Type: tea.KeyEnter}) // albums -> tracks
	for i := 0; i < trackIdx; i++ {
		m, _ = press(t, m, 'j')
	}
	m, cmd := update(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	msg := runCmd(t, cmd)
	loaded, ok := msg.(loadedMsg)
	if !ok || loaded.err != nil {
		t.Fatalf("first play returned %T (%v)", msg, loaded.err)
	}
	// feed the loadedMsg back so the model enters its playing state
	m, _ = update(t, m, msg)
	return m
}

func TestTrackEndAdvancesToNextTrack(t *testing.T) {
	m := playingCatalogModel(t, 0)
	var player *fakePlayer
	// recover the fake to observe what the auto-advance plays
	player = m.player.(*fakePlayer)

	// a finished track surfaces as persistent "property unavailable" polls
	var cmd tea.Cmd
	for i := 0; i <= progressErrorThreshold*2 && cmd == nil; i++ {
		m, cmd = update(t, m, progressMsg{
			err: errors.New("mpv error: property unavailable"),
		})
	}
	if cmd == nil {
		t.Fatal("end of track should return a next-track command")
	}
	if strings.Contains(m.status, "mpv error") {
		t.Fatalf("status = %q, want no scary error during auto-advance", m.status)
	}
	runCmd(t, cmd) // resolve+load next track through the fakes
	if got := strings.Join(player.titles, ","); got != "First Song,Second Song" {
		t.Fatalf("titles = %q, want auto-advance to the next track", got)
	}
}

func TestTrackEndOfAlbumSetsPlaylistEnded(t *testing.T) {
	m := playingCatalogModel(t, 1)
	// the model's bookkeeping should now pin the second track
	m, _ = update(t, m, loadedMsg{title: "Second Song"})
	var cmd tea.Cmd
	for i := 0; i <= progressErrorThreshold; i++ {
		m, cmd = update(t, m, progressMsg{
			err: errors.New("mpv error: property unavailable"),
		})
	}
	if cmd != nil {
		t.Fatalf("last track should not advance, got command %v", cmd)
	}
	if m.playing {
		t.Fatal("playing = true, want false after the playlist ends")
	}
	if !strings.Contains(m.status, "playlist ended") {
		t.Fatalf("status = %q, want the playlist-ended note", m.status)
	}
}

func TestLibraryEndAdvancesAndEnds(t *testing.T) {
	m := testModel(Options{
		Player:       &fakePlayer{},
		LibraryFiles: []string{"/lib/a1.mp3", "/lib/a2.mp3"},
	})
	m, _ = update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m, cmd := update(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	runCmd(t, cmd)
	m, _ = update(t, m, loadedMsg{title: "a1"})
	var nextCmd tea.Cmd
	for i := 0; i <= progressErrorThreshold*2 && nextCmd == nil; i++ {
		m, nextCmd = update(t, m, progressMsg{
			err: errors.New("mpv error: property unavailable"),
		})
	}
	if nextCmd == nil {
		t.Fatal("library playlist should advance to the next file")
	}
	msg := runCmd(t, nextCmd)
	if l, ok := msg.(loadedMsg); !ok || l.title != "Lib / A2" {
		t.Fatalf("library next = %v, want the second file", msg)
	}
	player := m.player.(*fakePlayer)
	_ = player
}
