package theme

// Theme registry for the panel (Phase A, issue #35). Three themes:
// Claro (light), Oscuro (dark), System (follow the terminal). Colors are
// plain ANSI color numbers resolved by a pure, deterministic
// theme+dark-background decision, so view code never calls runtime
// detection and stays testable.
//
// The dark pair is byte-identical with the palette the panel shipped
// before theming (issue #31 era): switching to Oscuro or System on a dark
// terminal must render exactly the same bytes as before.

// Theme is a canonical, lowercase theme id persisted in settings.json.
type Theme string

// Supported themes.
const (
	Claro  Theme = "claro"
	Oscuro Theme = "oscuro"
	System Theme = "system"
)

// Default is used when settings are absent or invalid.
const Default Theme = System

// Palette holds one color per panel chrome role. Values are ANSI 256-color
// numbers (the "205"-style values lipgloss.Color accepts).
type Palette struct {
	Title    string
	Dim      string
	Selected string
	Playing  string
	Status   string
	Bar      string
}

// darkPalette is the shipped panel palette (dark background).
var darkPalette = Palette{
	Title:    "205",
	Dim:      "241",
	Selected: "212",
	Playing:  "42",
	Status:   "214",
	Bar:      "205",
}

// lightPalette is the Claro variant, tuned for light-background terminals:
// darker, higher-contrast picks for the same roles.
var lightPalette = Palette{
	Title:    "205",
	Dim:      "239",
	Selected: "5",
	Playing:  "28",
	Status:   "208",
	Bar:      "205",
}

// Lookup maps a raw settings value to a Theme. Empty or unknown values are
// invalid (the caller decides the fallback; Resolve itself never panics).
func Lookup(name string) (Theme, bool) {
	switch Theme(normalizeName(name)) {
	case Claro:
		return Claro, true
	case Oscuro:
		return Oscuro, true
	case System:
		return System, true
	default:
		return "", false
	}
}

// String returns the canonical lowercase id persisted in settings.json.
func (t Theme) String() string { return normalizeName(string(t)) }

// Resolve returns the palette for a theme given whether the terminal has a
// dark background. Claro and Oscuro ignore the detection; System honors it.
// An unknown theme resolves to the default (System) palette instead of
// panicking (robustness contract: malformed settings never crash the TUI).
func Resolve(t Theme, dark bool) Palette {
	switch t {
	case Claro:
		return lightPalette
	case Oscuro:
		return darkPalette
	case System:
		if dark {
			return darkPalette
		}
		return lightPalette
	default:
		return Resolve(Default, dark)
	}
}

// normalizeName lowercases a settings value before matching.
func normalizeName(name string) string {
	b := []byte(name)
	for i := range b {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 'a' - 'A'
		}
	}
	return string(b)
}
