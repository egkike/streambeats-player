package tui

// Detection tests for the panel-terminal choice in the settings screen
// (issue #90, S3/S7). The lookup is injected so no test touches the real
// PATH (project standard: no environment-dependent assertions); detection
// fails closed — a terminal missing from PATH is simply absent from the
// offered rows, never guessed or substituted.

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"streambeats/internal/settings"
)

// lookupFor builds an injected lookup: only the listed binaries resolve.
func lookupFor(installed ...string) func(string) (string, error) {
	ok := make(map[string]struct{}, len(installed))
	for _, bin := range installed {
		ok[bin] = struct{}{}
	}
	return func(bin string) (string, error) {
		if _, hit := ok[bin]; hit {
			return "/fake/" + bin, nil
		}
		return "", exec.ErrNotFound
	}
}

// S3 canonical order: rows always come out ghostty first, kitty second,
// regardless of which lookup hits.
func TestPanelTerminalRowsCanonicalOrder(t *testing.T) {
	both := panelTerminalRows(lookupFor("kitty", "ghostty"))
	if strings.Join(both, ",") != "ghostty,kitty" {
		t.Errorf("both installed = %v, want [ghostty kitty] in canonical order", both)
	}
	kittyOnly := panelTerminalRows(lookupFor("kitty"))
	if strings.Join(kittyOnly, ",") != "kitty" {
		t.Errorf("kitty-only installed = %v, want [kitty]", kittyOnly)
	}
}

// S3/S7 fail closed: a missing terminal never appears as a selectable row.
func TestPanelTerminalRowsExcludeMissing(t *testing.T) {
	ghosttyOnly := panelTerminalRows(lookupFor("ghostty"))
	if strings.Join(ghosttyOnly, ",") != "ghostty" {
		t.Errorf("ghostty-only installed = %v, want [ghostty]; missing rows must be excluded", ghosttyOnly)
	}
}

// S7: with neither terminal installed the selection list is empty (the
// section renders its note; the launcher keeps today's friendly error).
func TestPanelTerminalRowsEmptyWhenNothingInstalled(t *testing.T) {
	none := panelTerminalRows(lookupFor())
	if len(none) != 0 {
		t.Errorf("nothing installed = %v, want no rows (fail closed, friendly error path)", none)
	}
}

// Detection never invents choices: an installed binary outside the offer set
// must not surface as a row.
func TestPanelTerminalRowsIgnoreForeignBinaries(t *testing.T) {
	got := panelTerminalRows(lookupFor("bash", "zsh"))
	if len(got) != 0 {
		t.Errorf("foreign installed binaries = %v, want no rows (no invented choices)", got)
	}
}

// S3: the current persisted choice is selectable when installed and its row
// is recognizable for the (current) marker; validity check rejects unknown
// ids before persistence can store them.
func TestPanelTerminalValidatedAgainstOffer(t *testing.T) {
	valid := []string{"ghostty", "kitty"}
	for _, id := range valid {
		if !panelTerminalValid(id) {
			t.Errorf("panelTerminalValid(%q) = false, want true (canonical choice)", id)
		}
	}
	for _, bad := range []string{"", "wezterm", "alacritty", "GHOSTTY", " kitty"} {
		if panelTerminalValid(bad) {
			t.Errorf("panelTerminalValid(%q) = true, want false (fails closed)", bad)
		}
	}
}

// ---------------------------------------------------------------------------
// Settings-screen menu layer (issue #90 S1/S2/S7): the terminal section rides
// the pure-update model pattern — tea.KeyMsg injection, statuses asserted on
// rendered output, fakes for the store and the lookup. No real PATH, no mpv.

// newTerminalChoiceModel builds a model with both injected fakes in place.
func newTerminalChoiceModel(t *testing.T, store *settings.Store, lookups func(string) (string, error), panelTerminal string) Model {
	t.Helper()
	return NewModel(Options{
		Theme:          "oscuro",
		ThemeDark:      true,
		SettingsStore:  store,
		LibraryRoot:    "/music",
		PanelTerminal:  panelTerminal,
		TerminalLookup: lookups,
	})
}

// S1/S3: the section lists ONLY the installed terminals, in canonical order,
// marking the persisted one as current; a missing binary never gets a row.
func TestConfigScreenTerminalSectionListsInstalledOnly(t *testing.T) {
	store := settings.NewStore(filepath.Join(t.TempDir(), "settings.json"))
	m := newTerminalChoiceModel(t, store, lookupFor("ghostty", "kitty"), "ghostty")
	after, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	view := after.(Model).View()
	if !strings.Contains(view, "ghostty") || !strings.Contains(view, "kitty") {
		t.Errorf("both installed: settings view lacks the terminal rows:\n%s", view)
	}
	if !strings.Contains(view, "ghostty  (current)") {
		t.Errorf("persisted panelTerminal must carry the (current) marker:\n%s", view)
	}

	// Now kitty alone: ghostty must disappear from the screen entirely.
	m = newTerminalChoiceModel(t, store, lookupFor("kitty"), "")
	after, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	view = after.(Model).View()
	if strings.Contains(view, "ghostty") {
		t.Errorf("uninstalled ghostty must not offer a row (fail closed):\n%s", view)
	}
	if !strings.Contains(view, "kitty") {
		t.Errorf("installed kitty must offer a row:\n%s", view)
	}
}

// S7: neither installed → no rows, a visible note, no invented choices.
func TestConfigScreenTerminalSectionEmptyOfferShowsNote(t *testing.T) {
	store := settings.NewStore(filepath.Join(t.TempDir(), "settings.json"))
	m := newTerminalChoiceModel(t, store, lookupFor(), "")
	after, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	view := after.(Model).View()
	if strings.Contains(view, "ghostty") || strings.Contains(view, "kitty") {
		t.Errorf("nothing installed: no terminal rows may render:\n%s", view)
	}
	if !strings.Contains(strings.ToLower(view), "no panel terminal") {
		t.Errorf("empty offer must show a friendly note:\n%s", view)
	}
}

// S2: committing a terminal change persists it and the status line tells the
// user the change applies after restarting the app.
func TestConfigScreenTerminalCommitNoticesRestart(t *testing.T) {
	store := settings.NewStore(filepath.Join(t.TempDir(), "settings.json"))
	m := newTerminalChoiceModel(t, store, lookupFor("ghostty", "kitty"), "")
	opened, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	// Themes occupy indices 0..2; the terminal rows start right after. From
	// the current theme (index 1) two-down reaches the ghostty row.
	d1, _ := opened.(Model).Update(tea.KeyMsg{Type: tea.KeyDown})
	d2, _ := d1.(Model).Update(tea.KeyMsg{Type: tea.KeyDown})
	commit, persist := d2.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	cm := commit.(Model)
	if cm.panelTerminal != "ghostty" {
		t.Errorf("committed panel terminal = %q, want ghostty", cm.panelTerminal)
	}
	if persist == nil {
		t.Fatalf("terminal commit returned no persistence cmd")
	}
	msg := persist()
	save, ok := msg.(saveMsg)
	if ok && save.err != nil {
		t.Fatalf("save failed: %v", save.err)
	}
	// The persistence feedback carries the restart notice (S2).
	delivered, _ := cm.Update(msg)
	if !strings.Contains(strings.ToLower(delivered.(Model).View()), "restart") {
		t.Errorf("committed change must announce the restart requirement:\n%s", delivered.(Model).View())
	}
	saved, err := store.Load()
	if err != nil {
		t.Fatalf("load after save: %v", err)
	}
	if saved.PanelTerminal != "ghostty" {
		t.Errorf("persisted panelTerminal = %q, want ghostty", saved.PanelTerminal)
	}
}

// S2/S7: re-committing the current terminal is a no-op — no restart notice.
func TestConfigScreenTerminalRecommitCurrentIsNoOp(t *testing.T) {
	store := settings.NewStore(filepath.Join(t.TempDir(), "settings.json"))
	m := newTerminalChoiceModel(t, store, lookupFor("ghostty", "kitty"), "ghostty")
	opened, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	d1, _ := opened.(Model).Update(tea.KeyMsg{Type: tea.KeyDown})
	d2, _ := d1.(Model).Update(tea.KeyMsg{Type: tea.KeyDown})
	commit, persist := d2.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	cm := commit.(Model)
	if strings.Contains(strings.ToLower(cm.View()), "restart") {
		t.Errorf("re-committing the current terminal must NOT announce a restart:\n%s", cm.View())
	}
	if persist != nil {
		t.Errorf("no-op commit must not fire a save cmd")
	}
}

// A theme save generalizes to full-settings persistence: an existing
// panelTerminal and libraryRoot survive a theme change (the latent clobber
// guard; prior behavior lost siblings).
func TestThemeSavePreservesTerminalAndRootSiblings(t *testing.T) {
	store := settings.NewStore(filepath.Join(t.TempDir(), "settings.json"))
	if err := store.Save(settings.Settings{Theme: "oscuro", LibraryRoot: "/music", PanelTerminal: "kitty"}); err != nil {
		t.Fatal(err)
	}
	m := newTerminalChoiceModel(t, store, lookupFor("ghostty", "kitty"), "kitty")
	opened, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	d1, _ := opened.(Model).Update(tea.KeyMsg{Type: tea.KeyDown})
	commit, persist := d1.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	if persist == nil {
		t.Fatalf("theme commit returned no persistence cmd")
	}
	msg := persist()
	if save, ok := msg.(saveMsg); ok && save.err != nil {
		t.Fatalf("save failed: %v", save.err)
	}
	saved, err := store.Load()
	if err != nil {
		t.Fatalf("load after save: %v", err)
	}
	if saved.Theme != "system" || saved.PanelTerminal != "kitty" || saved.LibraryRoot != "/music" {
		t.Errorf("theme save clobbered siblings: %+v", saved)
	}
	_ = commit
}

// ---------------------------------------------------------------------------
// Issue #116 (owner live report): the save feedback (terminal commit notice
// and theme saved) vanished almost immediately — the saveMsg handler set
// m.status directly WITHOUT arming the expiry window (statusClearTick), so
// the very next tick/progress reading wiped it. The rule is ONE: save
// feedback arms its window exactly like every other feedback (issue #40
// doctrine, setStatus). Pure-update assertions, no clock.

// The notice arms the window: statusClearTick must be strictly in the
// future after the saveMsg lands.
func TestSaveNoticeArmsExpiryWindow(t *testing.T) {
	store := settings.NewStore(filepath.Join(t.TempDir(), "settings.json"))
	m := newTerminalChoiceModel(t, store, lookupFor("ghostty", "kitty"), "")
	opened, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	d1, _ := opened.(Model).Update(tea.KeyMsg{Type: tea.KeyDown})
	d2, _ := d1.(Model).Update(tea.KeyMsg{Type: tea.KeyDown})
	commit, persist := d2.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	if persist == nil {
		t.Fatalf("terminal commit returned no persistence cmd")
	}
	m2, _ := update(t, commit.(Model), persist())
	if m2.statusClearTick <= m2.tick {
		t.Fatalf("save notice did not arm its expiry window: statusClearTick=%d tick=%d", m2.statusClearTick, m2.tick)
	}
}

// The notice survives the full feedback window against BOTH wipers: the
// tick handler and the progress reading (which fires every second while
// playing — the owner's scenario).
func TestSaveNoticeSurvivesFeedbackWindow(t *testing.T) {
	store := settings.NewStore(filepath.Join(t.TempDir(), "settings.json"))
	m := newTerminalChoiceModel(t, store, lookupFor("ghostty", "kitty"), "")
	opened, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	d1, _ := opened.(Model).Update(tea.KeyMsg{Type: tea.KeyDown})
	d2, _ := d1.(Model).Update(tea.KeyMsg{Type: tea.KeyDown})
	commit, persist := d2.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	if persist == nil {
		t.Fatalf("terminal commit returned no persistence cmd")
	}
	m2, _ := update(t, commit.(Model), persist())
	for i := 0; i < statusFeedbackTicks-1; i++ {
		next, _ := update(t, m2, tickMsg{})
		next, _ = update(t, next, progressMsg{pos: 1.2, dur: 20})
		m2 = next
	}
	if !strings.Contains(strings.ToLower(m2.View()), "restart") {
		t.Fatalf("restart notice vanished within its feedback window (ticks=%d):\n%s", statusFeedbackTicks-1, m2.View())
	}
}
