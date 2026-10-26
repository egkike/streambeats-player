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

	// Oscuro ignores detection: always its own dark accents (issue #40 S2,
	// visibly different from the shipped System-dark look).
	if !reflect.DeepEqual(Resolve(Oscuro, false), Resolve(Oscuro, true)) {
		t.Errorf("Oscuro must ignore detection")
	}
	if osc := Resolve(Oscuro, true); osc == goldenDark {
		t.Errorf("Oscuro repeats the shipped palette; the switch must be visible")
	}

	// The light pair is a different palette, not a copy.
	if reflect.DeepEqual(claroLight, goldenDark) {
		t.Errorf("Claro palette repeats the shipped palette; the switch must be visible")
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

// T4 (issue #40 S2): the owner must SEE the theme switch. System keeps the
// shipped dark pair at boot; Oscuro carries distinct accents so changing the
// theme is visible even on a dark terminal; Claro moves every role.
func TestSystemOnDarkKeepsTheShippedPalette(t *testing.T) {
	if got := Resolve(System, true); !reflect.DeepEqual(got, goldenDark) {
		t.Errorf("System on a dark terminal = %+v, want the shipped panel look %+v", got, goldenDark)
	}
}

func TestOscuroAccentsDifferFromSystemDark(t *testing.T) {
	sysDark := Resolve(System, true)
	osc := Resolve(Oscuro, true)
	if osc == sysDark {
		t.Fatalf("Oscuro repeats the System-dark palette; the switch is a no-op to the eye")
	}
	var diffs int
	if osc.Title != sysDark.Title {
		diffs++
	}
	if osc.Dim != sysDark.Dim {
		diffs++
	}
	if osc.Selected != sysDark.Selected {
		diffs++
	}
	if osc.Playing != sysDark.Playing {
		diffs++
	}
	if osc.Status != sysDark.Status {
		diffs++
	}
	if osc.Bar != sysDark.Bar {
		diffs++
	}
	if diffs < 2 {
		t.Errorf("only %d accents differ between Oscuro and System-dark; need at least 2 visible ones", diffs)
	}
}

func TestClaroDiffersFromOscuroInEveryRole(t *testing.T) {
	claro := Resolve(Claro, false)
	osc := Resolve(Oscuro, false)
	for name, a := range map[string][2]string{
		"Title":    {claro.Title, osc.Title},
		"Dim":      {claro.Dim, osc.Dim},
		"Selected": {claro.Selected, osc.Selected},
		"Playing":  {claro.Playing, osc.Playing},
		"Status":   {claro.Status, osc.Status},
		"Bar":      {claro.Bar, osc.Bar},
	} {
		if a[0] == a[1] {
			t.Errorf("role %s identical across Claro/Oscuro: %q", name, a[0])
		}
	}
}
