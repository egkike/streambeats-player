package tui

// Panel-terminal choice support (issue #90): the settings screen offers the
// terminals able to host the player panel, only when they are actually
// installed. Detection is injectable so tests never touch the real PATH
// (project standard) and fails closed: a missing binary never yields an
// offered row — the launcher keeps its own friendly-error path.
//
// The terminal ids below are the ONLY values persisted into
// settings.json (panelTerminal); the launcher (POSIX sh) resolves them with
// the precedence flag > panelTerminal > builtin order. Kitty 0.42+ provides
// the native Quake-style quick access terminal these presets rely on
// (wlr-layer-shell, the same Wayland requirement as Ghostty).

import "os/exec"

// panelTerminalChoices is the canonical offer order: ghostty first because
// its rendering path carries the calibrated keycaps (issue #64), kitty is
// the fail-soft alternative.
var panelTerminalChoices = []string{"ghostty", "kitty"}

// panelTerminalLookPath mirrors exec.LookPath's signature exactly so tests
// can inject a fake without a second abstraction.
type panelTerminalLookPath func(string) (string, error)

// defaultPanelTerminalLookup is the production lookup.
func defaultPanelTerminalLookup(bin string) (string, error) {
	return exec.LookPath(bin)
}

// panelTerminalValid enforces the persisted-value whitelist: an exact,
// case-sensitive match against the offer set; anything else fails closed.
func panelTerminalValid(id string) bool {
	switch id {
	case "ghostty", "kitty":
		return true
	}
	return false
}

// panelTerminalRows returns the offer set restricted to the terminals
// installed on this machine, always in canonical order. A lookup error
// simply drops the candidate — detection never substitutes or guesses.
func panelTerminalRows(lookup panelTerminalLookPath) []string {
	rows := make([]string, 0, len(panelTerminalChoices))
	for _, id := range panelTerminalChoices {
		if _, err := lookup(id); err != nil {
			continue
		}
		rows = append(rows, id)
	}
	return rows
}
