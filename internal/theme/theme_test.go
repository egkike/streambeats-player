package theme

// Tests for the theme registry (Phase A, issue #35, specs S1):
// three themes — Claro (light), Oscuro (dark), System (follow the
// terminal) — resolved deterministically so the TUI never depends on
// runtime background detection inside view code.

import (
	"reflect"
	"testing"
)

// goldenDark is the palette the panel shipped with before theming
// (issue #31 era): titles 205, dim 241, selected 212, playing 42,
// status 214, bar 205. Its render bytes must not regress.
var goldenDark = Palette{
	Title:    "205",
	Dim:      "241",
	Selected: "212",
	Playing:  "42",
	Status:   "214",
	Bar:      "205",
}

func TestLookupCanonicNames(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want Theme
		ok   bool
	}{
		{"lowercase claro", "claro", Claro, true},
		{"lowercase oscuro", "oscuro", Oscuro, true},
		{"lowercase system", "system", System, true},
		{"case-insensitive", "CLARO", Claro, true},
		{"case-insensitive mixed", "System", System, true},
		{"empty is invalid", "", "", false},
		{"unknown name", "pastel", "", false},
		{"symlink-ish injection", "../secret", "", false},
	}
	for _, tt := range tests {
		got, ok := Lookup(tt.in)
		if got != tt.want || ok != tt.ok {
			t.Errorf("Lookup(%q) = (%q, %v), want (%q, %v)", tt.in, got, ok, tt.want, tt.ok)
		}
	}
}

func TestResolveIsDeterministicPerTheme(t *testing.T) {
	// Claro ignores detection: always the light pair, whatever dark says.
	claroLight := Resolve(Claro, false)
	claroDark := Resolve(Claro, true)
	if !reflect.DeepEqual(claroLight, claroDark) {
		t.Errorf("Claro must ignore detection; light %+v vs dark %+v", claroLight, claroDark)
	}

	// Oscuro ignores detection: always the dark pair (byte-parity with the
	// panel as shipped).
	if got := Resolve(Oscuro, false); !reflect.DeepEqual(got, goldenDark) {
		t.Errorf("Oscuro rendered light = %v, want shipped palette %v", got, goldenDark)
	}
	if got := Resolve(Oscuro, true); !reflect.DeepEqual(got, goldenDark) {
		t.Errorf("Oscuro rendered dark = %v, want shipped palette %v", got, goldenDark)
	}

	// The light pair is a different palette, not a copy.
	if reflect.DeepEqual(claroLight, goldenDark) {
		t.Errorf("Claro palette repeats the dark palette; the switch must be visible")
	}
}

func TestResolveSystemFollowsDetection(t *testing.T) {
	light := Resolve(System, false)
	dark := Resolve(System, true)
	if reflect.DeepEqual(light, dark) {
		t.Errorf("System must follow background detection; light %+v == dark %+v", light, dark)
	}
	if !reflect.DeepEqual(dark, goldenDark) {
		t.Errorf("System on a dark terminal = %v, want shipped palette %v", dark, goldenDark)
	}
}

func TestResolveUnknownFallsBackToDefault(t *testing.T) {
	// An unknown theme must never panic and must resolve to the default
	// theme instead (S4: malformed settings never crash the TUI).
	if got := Resolve("", true); !reflect.DeepEqual(got, goldenDark) {
		t.Errorf("Resolve(unknown) = %v, want default palette %v", got, goldenDark)
	}
}

func TestDefaultThemeIsSystem(t *testing.T) {
	if Default != System {
		t.Errorf("Default theme = %q, want system", Default)
	}
}

func TestNamesMatchLowercaseStorage(t *testing.T) {
	// The name persisted in settings.json is the canonical lowercase id.
	names := []Theme{Claro, Oscuro, System}
	for _, th := range names {
		got, ok := Lookup(th.String())
		if !ok || got != th {
			t.Errorf("round trip %q failed: got (%q, %v)", th, got, ok)
		}
	}
}
