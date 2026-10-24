package tui

// Keyboard bindings for the TUI. Keeping them in one place lets the footer
// help line and the update loop stay in sync.

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
)

// keys is the single source of truth for every interactive shortcut.
var keys = struct {
	Up         key.Binding
	Down       key.Binding
	Enter      key.Binding
	Back       key.Binding
	Library    key.Binding
	Pause      key.Binding
	Stop       key.Binding
	VolumeUp   key.Binding
	VolumeDown key.Binding
	QueueNext  key.Binding
	QueuePrev  key.Binding
	Quit       key.Binding
}{
	Up:         key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k/up", "up")),
	Down:       key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j/down", "down")),
	Enter:      key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open/play")),
	Back:       key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	Library:    key.NewBinding(key.WithKeys("l"), key.WithHelp("l", "library")),
	Pause:      key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "pause/resume")),
	Stop:       key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "stop")),
	VolumeUp:   key.NewBinding(key.WithKeys("+", "="), key.WithHelp("+", "volume up")),
	QueueNext:  key.NewBinding(key.WithKeys(">"), key.WithHelp("> ", "next song >>")),
	QueuePrev:  key.NewBinding(key.WithKeys("<"), key.WithHelp("< ", "prev song <<")),
	VolumeDown: key.NewBinding(key.WithKeys("-", "_"), key.WithHelp("-", "volume down")),
	Quit:       key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
}

// helpLines renders the footer hints, split in two short rows so they survive
// narrow terminals.
func helpLines() []string {
	navigation := []key.Binding{keys.Up, keys.Down, keys.Enter, keys.Back, keys.Library}
	playback := []key.Binding{keys.Pause, keys.Stop, keys.QueueNext, keys.QueuePrev, keys.VolumeUp, keys.VolumeDown, keys.Quit}
	return []string{formatHelp(navigation), formatHelp(playback)}
}

// helpText joins the hint rows (used by tests and docs).
func helpText() string { return strings.Join(helpLines(), "\n") }

// formatHelp renders one row of key hints.
func formatHelp(bindings []key.Binding) string {
	parts := make([]string, 0, len(bindings))
	for _, b := range bindings {
		h := b.Help()
		parts = append(parts, h.Key+": "+h.Desc)
	}
	return strings.Join(parts, "  ")
}
