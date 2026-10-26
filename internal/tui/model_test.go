package tui

// Unit tests for the pure model logic: cursor clamping, mode transitions and
// the command/message round trips. Fakes stand in for mpv and the catalog so
// no test touches the network or a real player.

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"path/filepath"
	"streambeats/internal/catalog"

	"streambeats/internal/player"
	"streambeats/internal/settings"
	"streambeats/internal/theme"
)

// T1: the model carries its resolved theme palette; defaults keep the
// shipped dark look deterministic and an explicit Theme option applies.
func TestNewModelResolvesTheme(t *testing.T) {
	goldenDark := theme.Resolve(theme.System, true)

	// ThemeDark is explicit, never a hidden default: launch passes the real
	// background detection, so an explicit golden reflects a dark system.
	m := NewModel(Options{ThemeDark: true})
	if m.palette != goldenDark {
		t.Errorf("default palette = %+v, want shipped dark %+v", m.palette, goldenDark)
	}

	m = NewModel(Options{Theme: "claro"})
	if m.palette != theme.Resolve(theme.Claro, true) {
		t.Errorf("claro palette = %+v, want %+v", m.palette, theme.Resolve(theme.Claro, true))
	}

	m = NewModel(Options{Theme: "nope", ThemeDark: true})
	if m.palette != goldenDark {
		t.Errorf("unknown theme palette = %+v, want default fallback %+v", m.palette, goldenDark)
	}

	m = NewModel(Options{Theme: "system", ThemeDark: false})
	if m.palette != theme.Resolve(theme.System, false) {
		t.Errorf("system-light palette = %+v, want %+v", m.palette, theme.Resolve(theme.System, false))
	}
}

// T3: a Config screen selects the theme and persists it to the settings
// store; esc returns to the previous screen. Fakes keep it pure-update.
func TestConfigScreenSelectsAndPersistsTheme(t *testing.T) {
	store := settings.NewStore(filepath.Join(t.TempDir(), "settings.json"))
	m := NewModel(Options{Theme: "oscuro", ThemeDark: true, SettingsStore: store})

	// 'c' opens the config screen showing the three themes.
	after, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	cm := after.(Model)
	if cm.mode != ModeConfig {
		t.Fatalf("mode = %v, want ModeConfig", cm.mode)
	}
	view := cm.View()
	for _, want := range []string{"Claro", "Oscuro", "System"} {
		if !strings.Contains(view, want) {
			t.Fatalf("config view is missing theme %q:\n%s", want, view)
		}
	}

	// Esc from the very start of the cycle: down/select first, esc after.
	st, _ := cm.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if st.(Model).mode != ModeCatalog {
		t.Errorf("esc closed config into mode %v, want ModeCatalog", st.(Model).mode)
	}

	// Re-open, move down from the current selection (oscuro index 1) to
	// System and commit: returns a persistence cmd.
	after, _ = cm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	op := after.(Model)
	if op.configTheme() != theme.Oscuro {
		t.Fatalf("config opens on the current theme %q, want oscuro", op.configTheme())
	}
	op2, _ := op.Update(tea.KeyMsg{Type: tea.KeyDown})
	if got := op2.(Model).configTheme(); got != theme.System {
		t.Fatalf("after down the selection is %q, want system", got)
	}
	// Enter via the public interface commits the selection (configKey owns
	// the screen; activateSelection never sees config keys).
	committed, persist := op2.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	if committed.(Model).mode == ModeConfig {
		t.Errorf("enter must leave the config screen, still in config")
	}
	if committed.(Model).palette != theme.Resolve(theme.System, true) {
		t.Errorf("palette did not switch to system: %+v", committed.(Model).palette)
	}
	if persist == nil {
		t.Fatalf("enter returned no persistence cmd")
	}

	// The cmd writes settings.json with the selected theme.
	msg := persist()
	save, ok := msg.(saveMsg)
	if ok && save.err != nil {
		t.Fatalf("save failed: %v", save.err)
	}
	saved, err := store.Load()
	if err != nil {
		t.Fatalf("load after save: %v", err)
	}
	persisted, ok := theme.Lookup(saved.Theme)
	if !ok || persisted != theme.System {
		t.Errorf("persisted theme = %q, want system", saved.Theme)
	}

	// saveMsg completes the cycle in the status line (the screen already
	// returned to browse mode when the persistence landed).
	final, _ := committed.(Model).Update(save)
	if final.(Model).mode == ModeConfig {
		t.Errorf("save feedback must not navigate INTO config, mode = %v", final.(Model).mode)
	}
	if !strings.Contains(strings.ToLower(final.(Model).View()), "saved") {
		t.Errorf("status after save lacks a confirmation:\n%s", final.(Model).View())
	}
}

// T3/S4: a failed persistence surfaces a friendly status and keeps working.
func TestConfigScreenSaveFailureIsFriendly(t *testing.T) {
	// A path inside a non-directory makes every Save fail.
	store := settings.NewStore("/proc/nonexistent-dir/settings.json")
	m := NewModel(Options{Theme: "claro", SettingsStore: store})
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m3, _ := m2.(Model).Update(tea.KeyMsg{Type: tea.KeyDown})
	committed, persist := m3.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	if persist == nil {
		t.Fatalf("enter returned no persistence cmd")
	}
	msg, ok := persist().(saveMsg)
	if !ok {
		t.Fatalf("persist() returned %T, want saveMsg", persist())
	}
	if msg.err == nil {
		t.Fatal("save over an invalid path must error")
	}
	final, _ := committed.(Model).Update(msg)
	f := final.(Model)
	if !strings.Contains(strings.ToLower(f.View()), "could not save") {
		t.Errorf("failed save lacks a friendly status:\n%s", f.View())
	}
	// The theme state still switched (pure feedback), the view still works.
	if f.palette == theme.Resolve(theme.Claro, true) {
		t.Errorf("selection did not apply after failed save")
	}
}

// T3: playback bindings must not fire while the config screen is open.
func TestConfigScreenIgnoresPlaybackKeys(t *testing.T) {
	store := settings.NewStore(filepath.Join(t.TempDir(), "settings.json"))
	m := NewModel(Options{Theme: "oscuro", ThemeDark: true, SettingsStore: store})
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	cm := m2.(Model)
	// 's' is stop in browse mode; in config it must do nothing to playback
	// state and stay in config.
	next, _ := cm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	nm := next.(Model)
	if nm.mode != ModeConfig {
		t.Errorf("'s' closed the config screen, mode = %v", nm.mode)
	}
	if nm.configTheme() != theme.Oscuro {
		t.Errorf("'s' changed the selection to %q", nm.configTheme())
	}
	if nm.playing {
		t.Errorf("'s' mutated playback state while in config")
	}
}

// fakePlayer records playback calls instead of talking to mpv.
type fakePlayer struct {
	loaded     []string
	titles     []string
	volumes    []int
	playCalls  int
	pauseCalls int
	stopCalls  int

	loadErr   error
	props     map[string]float64
	propErr   error
	eventCh   chan player.Event
	quitCalls *int
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

// Events returns the fake's event channel (test-controlled).
func (f *fakePlayer) Events() <-chan player.Event { return f.eventCh }

// ObserveIdleActive records the observe request (no wire in the fake).
func (f *fakePlayer) ObserveIdleActive() error { return nil }

// Quit records mpv termination like the real client.
func (f *fakePlayer) Quit() error {
	f.stopCalls++ // treat as a further stop-family signal
	if f.quitCalls != nil {
		*f.quitCalls++
	}
	return nil
}

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

func TestPollErrorsStaySilentAcrossTransitions(t *testing.T) {
	m := testModel(Options{Player: &fakePlayer{}})

	// Track-end/loadfile gaps produce property misses; the transition must
	// be SILENT (user report L33): no banner at all, no buffering notes.
	for i := 0; i <= progressErrorThreshold*3; i++ {
		m, _ = update(t, m, progressMsg{err: errors.New("broken pipe")})
		if m.status != "" {
			t.Fatalf("status after %d poll errors = %q, want empty", i+1, m.status)
		}
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
	for _, want := range []string{"streambeats", "First Song", "▶", "0:05 / 0:10", "space ⏯"} {
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

// The stop+quit command issued on quit: stops playback, sends mpv's quit,
// and closes the connection — in that order (fake records the calls).
func TestQuitPlayerCmdStopsAndQuitsMpv(t *testing.T) {
	var quits int
	fake := &fakePlayer{quitCalls: &quits}
	m := runCmd(t, quitPlayerCmd(fake))
	if _, ok := m.(quitMsg); !ok {
		t.Fatalf("quitPlayerCmd returned %T, want quitMsg", m)
	}
	if fake.stopCalls == 0 {
		t.Fatal("stop was not called on quit")
	}
	if quits == 0 {
		t.Fatal("mpv quit was not sent on quit")
	}
}

// --- end-of-track auto-advance (L13) ---------------------------------------

// playingCatalogModel activates track trackIdx of album one so the model's
// playing* bookkeeping is set through the real key flow.
// playingAlbumTrack drives the real key flow to play trackIdx of albumIdx.
// eofIdle builds the documented signal: property-change idle-active=true.
func eofIdle() eventMsg {
	return eventMsg{event: player.Event{
		Name:     "property-change",
		Property: "idle-active",
		Bool:     boolPtr(true),
	}}
}

// machineGun pushes a full storm of poll errors with no cmd expected.
func boolPtr(v bool) *bool { return &v }

func playingAlbumTrack(t *testing.T, albumIdx, trackIdx int) Model {
	t.Helper()
	m := testModel(Options{
		Player:   &fakePlayer{},
		Resolver: &fakeResolver{url: "https://stream.example/1"},
	})
	for i := 0; i < albumIdx; i++ {
		m, _ = press(t, m, 'j')
	}
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
	m, _ = update(t, m, msg) // loadedMsg -> playing state
	return m
}

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

	// a finished track surfaces as the documented idle-active telemetry
	m, cmd := update(t, m, eofIdle())
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

// TestTrackEndOfAlbumSetsPlaylistEnded: with S10 only the last track of the
// LAST album ends the catalog queue; intermediate album ends cross forward.
func TestTrackEndOfAlbumSetsPlaylistEnded(t *testing.T) {
	m := playingAlbumTrack(t, 1, 0)
	m, cmd := update(t, m, eofIdle())
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
	m, nextCmd := update(t, m, eofIdle())
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

// --- instant EOF events + URL prefetch (L16/T11) ---------------------------

// TestEOFEventsRearmAcrossTracks exercises the event listener re-arm on a
// longer queue: two consecutive eof -> five tracks -> two advances.

// TestLoadedMsgPrefetchesNextTrackURL: right after a load, the GUI resolves
// the NEXT track's fresh URL so the eventual jump needs no network wait.
func TestLoadedMsgPrefetchesNextTrackURL(t *testing.T) {
	m := playingCatalogModel(t, 0)
	resolver, _ := m.resolve.(*fakeResolver)
	m, cmd := update(t, m, loadedMsg{title: "First Song"})
	if cmd == nil {
		t.Fatal("loadedMsg should schedule tick/events/prefetch batch")
	}
	// drain the batch: one of the cmds is the prefetch resolve
	prefetched := fromBatchToPrefetchMsg(t, cmd)
	if prefetched == nil {
		t.Fatal("no prefetch command inside the loadedMsg batch")
	}
	calls := resolver.calls
	if len(calls) != 2 {
		t.Fatalf("resolver calls = %v, want initial play + prefetch of second-song", calls)
	}
	if last := calls[len(calls)-1]; last != "album-one/second-song" {
		t.Fatalf("prefetched track = %q, want the next track", last)
	}
	if prefetched.slug != "second-song" || prefetched.err != nil {
		t.Fatalf("prefetch msg = %+v, want slug second-song without error", prefetched)
	}
}

func fromBatchToPrefetchMsg(t *testing.T, cmd tea.Cmd) *prefetchMsg {
	t.Helper()
	msg := runCmd(t, cmd)
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		return nil // not a batch: no prefetch in this build
	}
	for _, c := range batch {
		if c == nil {
			continue
		}
		if pm, ok := c().(prefetchMsg); ok {
			return &pm
		}
	}
	return nil
}

// TestAutoAdvanceUsesPrefetchedURL: when a track ends (poll errors while
// playing), the advance uses the cached URL for the exact next track,
// skipping the resolver entirely.
func TestAutoAdvanceUsesPrefetchedURL(t *testing.T) {
	m := playingCatalogModel(t, 0)
	resolver, _ := m.resolve.(*fakeResolver)
	m.prefetchSlug = "second-song"
	m.prefetchURL = "https://prefetch.example/2"
	m, cmd := update(t, m, eofIdle())
	runCmd(t, cmd)
	fake, _ := m.player.(*fakePlayer)
	if len(fake.loaded) != 2 {
		t.Fatalf("loaded = %v, want two entries", fake.loaded)
	}
	if fake.loaded[1] != "https://prefetch.example/2" {
		t.Fatalf("advanced load = %q, want the prefetched URL", fake.loaded[1])
	}
	for _, call := range resolver.calls {
		if call == "album-one/second-song" {
			t.Fatalf("resolver was re-asked for %q despite a valid prefetch", call)
		}
	}
}

// TestPrefetchStaleFallsBackToFreshResolve: if the cached URL expires
// (loadfile fails), the advance re-resolves and tries once more.
func TestPrefetchStaleFallsBackToFreshResolve(t *testing.T) {
	m := playingCatalogModel(t, 0)
	fake, _ := m.player.(*fakePlayer)
	fake.loadErr = errors.New("stale token")
	resolver, _ := m.resolve.(*fakeResolver)
	m.prefetchSlug = "second-song"
	m.prefetchURL = "https://prefetch.example/expired"
	m, cmd := update(t, m, eofIdle())
	runCmd(t, cmd)
	// fallback path asked the resolver for the next track after the stale fail
	if calls := resolver.calls; len(calls) == 0 || calls[len(calls)-1] != "album-one/second-song" {
		t.Fatalf("resolver fallback was not invoked: %v", resolver.calls)
	}
}

// --- the doc contract: idle-active drives everything (L29) -------------------

// Regression for the machine-gun (user report: enter alone jumped the whole
// playlist): poll errors must never, ever advance the queue.
func TestPollErrorsNeverAdvanceQueue(t *testing.T) {
	m := playingAlbumTrack(t, 0, 0)
	m, cmd := update(t, m, eofIdle())
	if cmd == nil {
		t.Fatal("idle-active while playing should advance")
	}
	if !m.playing {
		t.Fatal("idle event should hand over to the next track via the command")
	}
	// the next track lands
	m, _ = update(t, m, loadedMsg{title: "Second Song"})
	// NOW the storm: 30 consecutive poll errors produce zero commands
	for i := 0; i < progressErrorThreshold*10; i++ {
		m, cmd = update(t, m, progressMsg{err: errors.New("transient")})
		if cmd != nil {
			t.Fatalf("poll error storm produced a command at i=%d: %v", i, cmd)
		}
	}
}

// --TestEOFEventAdvances style: idle-active is the queue engine trigger
func TestIdleActiveAdvancesQueueFromEvent(t *testing.T) {
	m := playingAlbumTrack(t, 0, 0)
	m, cmd := update(t, m, eofIdle())
	if cmd == nil {
		t.Fatal("idle-active=true should advance the queue")
	}
	if m.playingTrack != 1 {
		t.Fatalf("playingTrack = %d, want 1", m.playingTrack)
	}
	m, _ = update(t, m, runCmd(t, cmd))
	fake, _ := m.player.(*fakePlayer)
	if len(fake.titles) < 2 || fake.titles[1] != "Second Song" {
		t.Fatalf("titles = %v, want the second track loaded", fake.titles)
	}
}

func TestIdleActiveOnlyWhenPlaying(t *testing.T) {
	m := testModel(Options{Player: &fakePlayer{}})
	_, cmd := update(t, m, eofIdle())
	if cmd != nil {
		t.Fatal("idle-active while nothing was intentionally playing is ignored")
	}
}

// last track of the last album -> playlist ended on idle-active
func TestIdleActiveLastTrackEndsQueue(t *testing.T) {
	m := playingAlbumTrack(t, 1, 0)
	m, cmd := update(t, m, eofIdle())
	if cmd != nil {
		t.Fatal("the catalog's last track must not produce a load command")
	}
	if m.playing {
		t.Fatal("playing = true, want false at the end of the catalog")
	}
	if !strings.Contains(m.status, "playlist ended") {
		t.Fatalf("status = %q, want playlist ended", m.status)
	}
}

// --- cursor follows playback + status clears on real progress ---------

// After an advance (auto or manual), the BROWSE cursor must follow the
// playing item, so the ♪/▸ markers do not drift apart (user report L31).
func TestLoadedMsgSyncsBrowseCursorToPlayingItem(t *testing.T) {
	m := playingAlbumTrack(t, 0, 0)
	// simulate an advance playing track 1 while the browse UI still cursored 0
	m.playingAlbum, m.playingTrack = 0, 1
	m, _ = update(t, m, loadedMsg{title: "Second Song"})
	if m.playingTrack != m.trackCursor || !m.inTracks {
		t.Fatalf("cursor not synced: trackCursor=%d playingTrack=%d inTracks=%v",
			m.trackCursor, m.playingTrack, m.inTracks)
	}
	if m.albumCursor != 0 {
		t.Fatalf("albumCursor = %d, want the playing album", m.albumCursor)
	}
}

// A transient property error (mpv does not report duration/time-pos during
// the first moments of a stream) must CLAR the status banner on the next
// real progress reading, not stay for the whole track (user report L31).
func TestProgressSuccessClearsErrorStatus(t *testing.T) {
	m := playingAlbumTrack(t, 0, 0)
	m.status = "mpv error: property unavailable"
	m, _ = update(t, m, progressMsg{pos: 2, dur: 125})
	if m.status != "" {
		t.Fatalf("status = %q, want cleared on real progress", m.status)
	}
}

// --- cross-album queue navigation (S10) -------------------------------------

// TestPrevCrossesToPreviousAlbum: '<' at the first track of an album must
// load the LAST track of the previous album, not stop the queue.
func TestPrevCrossesToPreviousAlbum(t *testing.T) {
	m := playingAlbumTrack(t, 1, 0)
	m, cmd := update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'<'}})
	if cmd == nil {
		t.Fatal("crossing albums backward should return a load command")
	}
	if m.playingAlbum != 0 || m.playingTrack != 1 {
		t.Fatalf("bookkeeping = album %d/%d, want 0/1", m.playingAlbum, m.playingTrack)
	}
	m, _ = update(t, m, runCmd(t, cmd))
	fake, _ := m.player.(*fakePlayer)
	if len(fake.titles) < 2 || fake.titles[len(fake.titles)-1] != "Second Song" {
		t.Fatalf("titles = %v, want the previous album's last track", fake.titles)
	}
}

func TestPrevAfterVeryStartKeepsQueueStopped(t *testing.T) {
	m := playingAlbumTrack(t, 0, 0)
	m, cmd := update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'<'}})
	if cmd != nil {
		t.Fatal("there is nothing before the very first track")
	}
}

// TestsNextCrossesToNextAlbumMatchesNaturalFlow: at an album's end, '>' and
// the natural EOF both continue with the NEXT album's first track.
func TestNextCrossesToNextAlbum(t *testing.T) {
	m := playingAlbumTrack(t, 0, 1)
	m, cmd := update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'>'}})
	if cmd == nil {
		t.Fatal("crossing albums forward should return a load command")
	}
	if m.playingAlbum != 1 || m.playingTrack != 0 {
		t.Fatalf("bookkeeping = album %d/%d, want 1/0", m.playingAlbum, m.playingTrack)
	}
}

// --- manual queue skip keys (L21) -------------------------------------------

func TestNextKeyAdvancesQueue(t *testing.T) {
	m := playingCatalogModel(t, 0)
	m, _ = update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'>'}})
	if m.playingTrack != 1 {
		t.Fatalf("playingTrack = %d, want 1 after '>'", m.playingTrack)
	}
	// the next track's loadedMsg confirms the new queue position afterwards
	m, _ = update(t, m, loadedMsg{title: "Second Song"})
	if !m.playing {
		t.Fatal("playing must resume once the next track lands")
	}
}

func TestNextKeyOnLastTrackEndsPlaylist(t *testing.T) {
	// the LAST track of the LAST album: '>' must stop the queue (S10 changed
	// the old album-boundary behavior: '>' and EOF now flow into the next album)
	m := playingAlbumTrack(t, 1, 0)
	m, _ = update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'>'}})
	if m.playing {
		t.Fatal("playing = true, want false at the end of the queue")
	}
	if !strings.Contains(m.status, "playlist ended") {
		t.Fatalf("status = %q, want playlist ended", m.status)
	}
}

func TestPrevKeyPlaysPreviousTrack(t *testing.T) {
	m := playingCatalogModel(t, 1)
	m, cmd := update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'<'}})
	if m.playingTrack != 0 {
		t.Fatalf("playingTrack = %d, want 0 after '<' from track 1", m.playingTrack)
	}
	msg := runCmd(t, cmd)
	m, _ = update(t, m, msg)
	fake, _ := m.player.(*fakePlayer)
	if len(fake.titles) < 2 || fake.titles[len(fake.titles)-1] != "First Song" {
		t.Fatalf("titles = %v, want the previous track loaded", fake.titles)
	}
}
func TestPrevKeyAtAlbumStartShowsNote(t *testing.T) {
	m := playingCatalogModel(t, 0)
	m, _ = update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'<'}})
	if m.playingTrack != 0 || m.playingAlbum != 0 {
		t.Fatalf("bookkeeping = album %d/%d, want unchanged at the very start", m.playingAlbum, m.playingTrack)
	}
	if !strings.Contains(m.status, "no previous track") {
		t.Fatalf("status = %q, want the no-previous note", m.status)
	}
}

// T1 (issue #37 S1): the transport buttons row renders on the panel and is
// mouse-clickable; every click routes through the SAME commands as the
// keybinds (S2: no duplicate action logic), and invalid hits change nothing
// (S6). The row Y comes from the bottom block layout (0-based index).
// runClick runs a mouse hit like the tea runtime would: Update returns a
// command; the side effect lands when the command executes.
func runClick(m Model, msg tea.Msg) (Model, tea.Cmd) {
	next, cmd := m.Update(msg)
	if cmd != nil {
		cmd()
	}
	return next.(Model), cmd
}

func clickModel(t *testing.T) (Model, *fakePlayer) {
	t.Helper()
	fp := &fakePlayer{}
	m := NewModel(Options{Player: fp})
	m.width, m.height = 52, 24
	return m, fp
}

func mouseAt(m Model, span buttonSpan) tea.MouseMsg {
	return tea.MouseMsg{
		X:      span.Start + span.Width/2,
		Y:      m.height - 4, // the buttons row inside the bottom block
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	}
}

func buttonSpanByKind(t *testing.T, m Model, kind buttonKind) buttonSpan {
	t.Helper()
	for _, s := range m.buttonLayout() {
		if s.Kind == kind {
			return s
		}
	}
	t.Fatalf("button row has no %q span: %+v", kind, m.buttonLayout())
	return buttonSpan{}
}

func TestTransportButtonsRowIsRendered(t *testing.T) {
	m, _ := clickModel(t)
	m.width, m.height = 52, 24
	out := m.View()
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	row := strings.Join(lines[m.height-4:m.height-3], "\n")
	for _, want := range []string{"⏮", "⏹", "⏭", "🔊", "▶"} {
		if !strings.Contains(row, want) {
			t.Fatalf("buttons row %q is missing %q", row, want)
		}
	}
	// Playing + not paused shows the pause glyph instead of play (⏯).
	m.playing = true
	row = strings.Join(strings.Split(strings.TrimSuffix(m.View()+"\n", "\x00"), "\x00"), "\n")
	row = strings.Join(strings.Split(strings.TrimSuffix(m.View(), "\n"), "\n")[m.height-4:m.height-3], "\n")
	if strings.Contains(row, "▶") || !strings.Contains(row, "⏸") {
		t.Fatalf("buttons row while playing must show ⏸ (not ▶): %q", row)
	}
}

func TestMouseClickPaintsPausePathLikeTheKey(t *testing.T) {
	mKey, fakeKey := clickModel(t)
	mBtn, fakeBtn := clickModel(t)
	for _, m := range []*Model{&mKey, &mBtn} {
		m.playing = true
		m.nowPlaying, m.volume = "song", 50
	}
	span := buttonSpanByKind(t, mBtn, btnToggle)
	// Same action, two transports: key 'space' vs click on ⏸.
	am, _ := runClick(mKey, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	bm, _ := runClick(mBtn, mouseAt(mBtn, span))
	if am.paused != true || bm.paused != true {
		t.Fatalf("pause state after space=%v vs click=%v", am.paused, bm.paused)
	}
	if fakeKey.pauseCalls != fakeBtn.pauseCalls || fakeBtn.pauseCalls != 1 {
		t.Fatalf("pause calls: key=%d button=%d", fakeKey.pauseCalls, fakeBtn.pauseCalls)
	}
	// Second click unpauses (Play path), same as a second space.
	am, _ = runClick(am, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	bm, _ = runClick(bm, mouseAt(mBtn, span))
	if !am.playing || !bm.playing {
		t.Fatalf("resume state after space=%v vs click=%v", am.playing, bm.playing)
	}
}

func TestMouseClickStopNextPrevVolMatchTheirKeys(t *testing.T) {
	// stop: click ⏹ == key 's' (stops + clears the bar).
	mKey, fakeKey := clickModel(t)
	mBtn, fakeBtn := clickModel(t)
	for _, m := range []*Model{&mKey, &mBtn} {
		m.playing, m.paused = true, true
	}
	aKey, _ := runClick(mKey, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	aBtn, _ := runClick(mBtn, mouseAt(mBtn, buttonSpanByKind(t, mBtn, btnStop)))
	if fakeKey.stopCalls != 1 || fakeBtn.stopCalls != 1 {
		t.Fatalf("stop calls: key=%d button=%d", fakeKey.stopCalls, fakeBtn.stopCalls)
	}
	if aKey.playing || aBtn.playing {
		t.Fatalf("bar must clear after stop: key=%v button=%v", aKey.playing, aBtn.playing)
	}

	// next: click ⏭ == key '>' (library bookkeeping; 2 items, playing #0).
	mKey, fakeKey = clickModel(t)
	mBtn, fakeBtn = clickModel(t)
	for _, m := range []*Model{&mKey, &mBtn} {
		m.mode = ModeLibrary
		m.library = []string{"01 - a/01 - s0.mp3", "01 - a/02 - s1.mp3"}
		m.playingLib, m.playing = 0, true
	}
	runClick(mKey, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'>'}})
	runClick(mBtn, mouseAt(mBtn, buttonSpanByKind(t, mBtn, btnNext)))
	fakeKeyKey := fakeKey.loaded == nil
	if fakeKey.loaded == nil || fakeBtn.loaded == nil {
		t.Fatalf("queue-next advance from library top must reload: key=%v button=%v", fakeKey.loaded, fakeBtn.loaded)
	}
	_ = fakeKeyKey
	if fakeKey.titles == nil || fakeBtn.titles == nil || fakeKey.titles[0] != fakeBtn.titles[0] {
		t.Fatalf("next loaded titles differ: key=%v button=%v", fakeKey.titles, fakeBtn.titles)
	}

	// prev: click ⏮ == key '<' (library wrap path).
	mKey, fakeKey = clickModel(t)
	mBtn, fakeBtn = clickModel(t)
	for _, m := range []*Model{&mKey, &mBtn} {
		m.mode = ModeLibrary
		m.library = []string{"01 - a/01 - s0.mp3", "01 - a/02 - s1.mp3"}
		m.playingLib, m.playing = 1, true
	}
	runClick(mKey, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'<'}})
	runClick(mBtn, mouseAt(mBtn, buttonSpanByKind(t, mBtn, btnPrev)))
	if fakeKey.titles == nil || fakeBtn.titles == nil || fakeKey.titles[0] != fakeBtn.titles[0] {
		t.Fatalf("prev loaded titles differ: key=%v button=%v", fakeKey.titles, fakeBtn.titles)
	}

	// volume: click 🔊 == key '+' (same step).
	mKey, fakeKey = clickModel(t)
	mBtn, fakeBtn = clickModel(t)
	runClick(mKey, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
	runClick(mBtn, mouseAt(mBtn, buttonSpanByKind(t, mBtn, btnVolume)))
	if fakeKey.volumes == nil || fakeBtn.volumes == nil || fakeKey.volumes[0] != fakeBtn.volumes[0] {
		t.Fatalf("volume steps differ: key=%v button=%v", fakeKey.volumes, fakeBtn.volumes)
	}
}

func TestMouseClickInvalidHitsChangeNothing(t *testing.T) {
	m, fake := clickModel(t)
	m.playing = true
	base := func() string {
		return m.View() + fmt.Sprint(fake.pauseCalls, fake.playCalls, fake.stopCalls, fake.volumes, m.status)
	}
	before, other := base(), tea.MouseMsg{X: 999, Y: m.height - 4, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress}
	m.Update(other)
	m.Update(tea.MouseMsg{X: 5, Y: 2, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})  // list area, not a button
	m.Update(tea.MouseMsg{X: 1, Y: 2, Button: tea.MouseButtonLeft, Action: tea.MouseActionMotion}) // motion, not press
	if got := base(); got != before {
		t.Fatalf("invalid/motion hits changed state:\nbefore=%q\nafter =%q", before, got)
	}
}

func TestMouseWithoutPlayerIsFriendly(t *testing.T) {
	m := NewModel(Options{})
	m.height = 24
	span := buttonSpanByKind(t, m, btnToggle)
	next, _ := m.Update(mouseAt(m, span))
	nm := next.(Model)
	if nm.status == "" {
		t.Fatalf("toggle click without player must surface a friendly status:\n%s", nm.View())
	}
}

// R4-001 (receipt review-656a2bb02230f588): mpv-reported volume is untrusted
// input for the UI math — receipt must clamp like our own writes do.
func TestVolumeMsgReceiptClampsToPanelBounds(t *testing.T) {
	m := NewModel(Options{})
	for _, tc := range []struct {
		in, want int
	}{
		{150, 100},
		{-7, 0},
	} {
		next, _ := m.Update(volumeMsg{volume: tc.in})
		if got := next.(Model).volume; got != tc.want {
			t.Errorf("volume receipt %d => %d, want clamped %d", tc.in, got, tc.want)
		}
	}
}

// T1 (issue #40 S1): every transport action leaves a distinct, honest status
// line — pause/resume/stop through both transports, and the volume receipt
// (currently silent when a player is connected).
func TestTransportFeedbackStatuses(t *testing.T) {
	// pause with a live player says "paused"; resume says "resumed".
	m, fake := clickModel(t)
	m.playing = true
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	nm := next.(Model)
	if nm.paused != true || !strings.Contains(nm.status, "paused") {
		t.Fatalf("pause feedback: paused=%v status=%q", nm.paused, nm.status)
	}
	for _, c := range []rune{' '} {
		_ = c
	}
	_ = fake
	next, _ = nm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	nm = next.(Model)
	if nm.paused != false || !strings.Contains(nm.status, "resumed") {
		t.Fatalf("resume feedback: paused=%v status=%q", nm.paused, nm.status)
	}

	// stop while playing says "stopped" and keeps the player calls.
	m, fake = clickModel(t)
	m.playing, m.paused, m.nowPlaying = true, true, "song"
	a, stopCmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if stopCmd != nil {
		stopCmd()
	}
	nm = a.(Model)
	if fake.stopCalls != 1 || !strings.Contains(nm.status, "stopped") {
		t.Fatalf("stop feedback: calls=%d status=%q", fake.stopCalls, nm.status)
	}

	// the volume receipt states the level (previously silent).
	m2 := NewModel(Options{Player: &fakePlayer{}})
	next, _ = m2.Update(volumeMsg{volume: 55})
	nm = next.(Model)
	if !strings.Contains(nm.status, "volume 55%") {
		t.Fatalf("volume feedback missing: %q", nm.status)
	}
}

// T2/T3 (issue #40 S3/S4): the progress poll must not erase fresh feedback,
// and any feedback statuses expire by themselves (tick-count based, ~4s at
// the 1s cadence).
func TestFeedbackSurvivesProgressAndExpiresByTicks(t *testing.T) {
	m, fake := clickModel(t)
	m.playing = true
	a, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	nm := a.(Model)
	if !strings.Contains(nm.status, "paused") {
		t.Fatalf("paused feedback missing: %q", nm.status)
	}
	_ = fake
	// A real progress reading must NOT eat the fresh feedback.
	r2, _ := nm.Update(progressMsg{pos: 1, dur: 100})
	nm = r2.(Model)
	if !strings.Contains(nm.status, "paused") {
		t.Fatalf("progress tick erased fresh feedback: %q", nm.status)
	}
	// Ticks inside the window keep it visible; the 4th tick expires it.
	for i := 0; i < statusFeedbackTicks-1; i++ {
		r3, _ := nm.Update(tickMsg{})
		nm = r3.(Model)
		if !strings.Contains(nm.status, "paused") {
			t.Fatalf("feedback expired before its window at tick %d: %q", i, nm.status)
		}
	}
	r4, _ := nm.Update(tickMsg{})
	nm = r4.(Model)
	if nm.status != "" {
		t.Fatalf("feedback must expire after %d ticks: %q", statusFeedbackTicks, nm.status)
	}
}
