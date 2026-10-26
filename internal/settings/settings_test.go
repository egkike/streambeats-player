package settings

// Tests for the persistent settings store (Phase A, issue #35, specs S2/S4):
// theme choice lives at the streambeats config dir, atomic save, and absent
// or malformed files must never crash the caller (fast fail path).

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	return NewStore(filepath.Join(t.TempDir(), "settings.json"))
}

func TestDefaultPathLivesInStreambeatsConfigDir(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	p, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}
	if want := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "streambeats", "settings.json"); p != want {
		t.Errorf("DefaultPath = %q, want %q", p, want)
	}
}

func TestLoadMissingFileYieldsDefaults(t *testing.T) {
	s, err := testStore(t).Load()
	if err != nil {
		t.Fatalf("Load on missing file: %v", err)
	}
	if s != (Settings{}) {
		t.Errorf("Load on missing file = %+v, want zero settings", s)
	}
}

func TestLoadMalformedFileErrors(t *testing.T) {
	st := testStore(t)
	if err := os.WriteFile(st.Path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Load(); err == nil {
		t.Fatal("Load on malformed file must return an error so the caller falls back to defaults")
	}
}

func TestSaveRoundTripsTheme(t *testing.T) {
	st := testStore(t)
	if err := st.Save(Settings{Theme: "oscuro"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := st.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Theme != "oscuro" {
		t.Errorf("Theme = %q, want oscuro", got.Theme)
	}
}

func TestSaveOverwritesPreviousContent(t *testing.T) {
	st := testStore(t)
	if err := st.Save(Settings{Theme: "claro"}); err != nil {
		t.Fatal(err)
	}
	if err := st.Save(Settings{Theme: "system"}); err != nil {
		t.Fatal(err)
	}
	got, err := st.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Theme != "system" {
		t.Errorf("Theme = %q, want the later save (system)", got.Theme)
	}
}

func TestSaveLeavesNoTempResidue(t *testing.T) {
	st := testStore(t)
	if err := st.Save(Settings{Theme: "claro"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(st.Path + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("temp file residue after atomic commit: %v", err)
	}
}

func TestSaveCreatesConfigDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "streambeats")
	st := NewStore(filepath.Join(dir, "settings.json"))
	if err := st.Save(Settings{Theme: "claro"}); err != nil {
		t.Fatalf("Save must create the config dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "settings.json")); err != nil {
		t.Errorf("settings file missing after Save: %v", err)
	}
}

func TestSaveRejectsInsecurePermissionsDrift(t *testing.T) {
	// Settings hold no secrets, but the store precedent (catalog cache,
	// 0600) must hold: file perms stay owner-only.
	st := testStore(t)
	if err := st.Save(Settings{Theme: "claro"}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(st.Path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("settings mode = %o, want 600", got)
	}
}

func TestLoadIgnoresUnknownKeysButKeepsTheme(t *testing.T) {
	// Forward compatibility: extra fields must not break old binaries.
	st := testStore(t)
	raw := `{"theme": "claro", "future_option": 7}`
	if err := os.WriteFile(st.Path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := st.Load()
	if err != nil {
		t.Fatalf("Load with unknown keys: %v", err)
	}
	if s.Theme != "claro" {
		t.Errorf("Theme = %q, want claro", s.Theme)
	}
}

func TestSaveWritesReadableJSON(t *testing.T) {
	st := testStore(t)
	if err := st.Save(Settings{Theme: "system"}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(st.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"theme": "system"`) {
		t.Errorf("file does not contain the theme field:\n%s", raw)
	}
}
