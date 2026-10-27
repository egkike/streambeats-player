package tui

// Keyboard bindings for the TUI. Keeping them in one place lets the keycap
// row, the status-dual nav hints and the update loop stay in sync (issue #60):
// the visual rows advertise only keys from this table.

import (
	"github.com/charmbracelet/bubbles/key"
)

// keys is the single source of truth for every interactive shortcut.
var keys = struct {
	Up         key.Binding
	Down       key.Binding
	Enter      key.Binding
	Back       key.Binding
	Library    key.Binding
	Config     key.Binding
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
	Config:     key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "settings")),
	Pause:      key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "pause/resume")),
	Stop:       key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "stop")),
	VolumeUp:   key.NewBinding(key.WithKeys("+", "="), key.WithHelp("+", "volume up")),
	QueueNext:  key.NewBinding(key.WithKeys(">", "n"), key.WithHelp("> / n", "next song >>")),
	QueuePrev:  key.NewBinding(key.WithKeys("<", "p"), key.WithHelp("< / p", "prev song <<")),
	VolumeDown: key.NewBinding(key.WithKeys("-", "_"), key.WithHelp("-", "volume down")),
	Quit:       key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
}
