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
	Claro     Theme = "claro"
	Oscuro    Theme = "oscuro"
	System    Theme = "system"
	Vinyl     Theme = "vinyl"
	Synthwave Theme = "synthwave"
	Quiet     Theme = "quiet"
	Cute      Theme = "cute"
	Sexy      Theme = "sexy"
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

// darkPalette is the shipped panel palette, kept as the SYSTEM-on-dark
// look (byte-parity at boot on dark terminals; TestSystemOnDarkKeeps...
// pins it).
var darkPalette = Palette{
	Title:    "205",
	Dim:      "241",
	Selected: "212",
	Playing:  "42",
	Status:   "214",
	Bar:      "205",
}

// artboardPalettes (issue #140 S2): the five owner-approved panel themes,
// each pinned coordinate for coordinate to its design artboard spec card
// (design/streambeats-panel-look/01..05). Values are 24-bit hex strings —
// lipgloss.Color accepts them and Ghostty/Kitty host them in truecolor;
// 256-color terminals degrade gracefully through the SGR negotiation. These
// palettes are authored for DARK surfaces and ignore background detection
// (TestNewThemeIdsAreValid pins detection independence): explicit owner
// choice replaces the legacy automatic light/dark adaptation.
var artboardPalettes = map[Theme]Palette{
	// 01-vinyl-dark.html: warm amber accent, cream-surface intent, moss
	// green playing, coral status.
	Vinyl: {Title: "#e8a33d", Dim: "#8a7a5f", Selected: "#e8a33d", Playing: "#7fae5c", Status: "#d9704f", Bar: "#e8a33d"},
	// 02-synthwave.html: hot magenta accent over violet night, spring
	// green playing, cyan second voice on the status/messages slot.
	Synthwave: {Title: "#ff5ec8", Dim: "#8b7fd4", Selected: "#ff5ec8", Playing: "#6dff9c", Status: "#4fd6e0", Bar: "#ff5ec8"},
	// 03-quiet-ink.html: near-monochrome ink, one sparing blue accent
	// (Bar/status), near-white titles, calm green playing.
	Quiet: {Title: "#e6e9ef", Dim: "#5c6370", Selected: "#eaf0ff", Playing: "#9ece6a", Status: "#7aa2f7", Bar: "#7aa2f7"},
	// 04-gentle-cute.html: soft pink accents straight from gentle-pi's
	// Gentleman-Cute.json (accent/activePink/mint/peach/deepPink). Owner's
	// daily shell theme.
	Cute: {Title: "#F095C8", Dim: "#76616B", Selected: "#FFB1DD", Playing: "#B4E7C7", Status: "#F2B86D", Bar: "#C96AA2"},
	// 05-gentle-sexy.html: hot pink accents straight from gentle-pi's
	// Gentleman-Sexy.json (accent/activePink/violet/champagne/deepPink).
	Sexy: {Title: "#F43888", Dim: "#76616B", Selected: "#FF4F9A", Playing: "#C49BFF", Status: "#E0C27A", Bar: "#BF0F50"},
}

// oscuroPalette is the explicit dark theme: same ambient as the shipped
// panel but with distinct accents (selected/playing moved), so the theme
// switch is VISIBLE at a glance on a dark terminal (issue #40 S2).
var oscuroPalette = Palette{
	Title:    "205",
	Dim:      "241",
	Selected: "99", // violet
	Playing:  "46", // bright green
	Status:   "214",
	Bar:      "205",
}

// lightPalette is the Claro family, tuned for light-background terminals:
// a different hue per role (dark-on-light readable):
var lightPalette = Palette{
	Title:    "18",  // dark teal
	Dim:      "59",  // gray-green
	Selected: "5",   // magenta
	Playing:  "28",  // green
	Status:   "166", // coral
	Bar:      "18",
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
	case Vinyl:
		return Vinyl, true
	case Synthwave:
		return Synthwave, true
	case Quiet:
		return Quiet, true
	case Cute:
		return Cute, true
	case Sexy:
		return Sexy, true
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
		return oscuroPalette
	case System:
		if dark {
			return darkPalette
		}
		return lightPalette
	case Vinyl, Synthwave, Quiet, Cute, Sexy:
		// Artboard themes are tinned for dark surfaces and authored exactly
		// once by the owner: detection is ignored, the palette is constant
		// (issue #140 S2/S7).
		return artboardPalettes[t]
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
