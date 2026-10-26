package tui

// Launcher: wires the catalog, local library and mpv backend into a Bubbletea
// program. Spawning mpv is best effort so the player stays browsable when the
// binary (or its socket) is unavailable.

import (
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/charmbracelet/lipgloss"

	"streambeats/internal/catalog"
	"streambeats/internal/player"
	"streambeats/internal/settings"
)

// Launch boots the TUI with the given catalog and blocks until the user quits.
func Launch(cat *catalog.Catalog) error {
	root, rootErr := catalog.DefaultLibraryPath()

	var files []string
	if rootErr == nil {
		if scanned, err := catalog.ScanLibrary(root); err == nil {
			files = scanned
		}
	}

	var (
		client     *player.Client
		proc       *exec.Cmd
		playerNote string
	)
	socketPath, err := DefaultSocketPath()
	if err != nil {
		playerNote = friendlyError("audio disabled", err)
	} else {
		proc, client, err = SpawnMPV(socketPath)
		if err != nil {
			playerNote = friendlyError("audio disabled", err)
		}
	}
	if client != nil {
		// Doc-based registration (DOCS/man/ipc.rst): observe idle-active so
		// property-change events surface the authoritative track end.
		_ = client.ObserveIdleActive()
		defer func() { _ = client.Close() }()
	}
	if proc != nil {
		defer stopProcess(proc)
	}

	// Theme resolution (Phase A): the persisted choice wins; an absent or
	// malformed settings file falls back to System on the detected
	// background (S4: the TUI never crashes over settings). The store goes
	// on Options so the Config screen can also persist (R3-config-not-
	// persisted, review lineage review-d1aeb96ca49e64a5): loading without
	// wiring the store back in breaks the write path at the junction point.
	settingsStore, settingsErr := settings.DefaultStore()
	theme := ""
	if settingsErr == nil {
		if user, err := settingsStore.Load(); err == nil {
			theme = user.Theme
		}
	}
	opts := Options{
		Catalog:       cat,
		LibraryFiles:  files,
		LibraryRoot:   root,
		PlayerNote:    playerNote,
		Resolver:      catalog.NewClient(),
		Theme:         theme,
		ThemeDark:     lipgloss.HasDarkBackground(),
		SettingsStore: settingsStore,
	}
	if client != nil {
		opts.Player = client
	}

	// Phase B (issue #37): cell-motion mouse enables the transport button
	// clicks (S1); AllMotion is overkill here (no hover interactions).
	program := tea.NewProgram(NewModel(opts), tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err = program.Run()
	return err
}
