package tui

// Launcher: wires the catalog, local library and mpv backend into a Bubbletea
// program. Spawning mpv is best effort so the player stays browsable when the
// binary (or its socket) is unavailable.

import (
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"

	"streambeats/internal/catalog"
	"streambeats/internal/player"
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
		defer func() { _ = client.Close() }()
	}
	if proc != nil {
		defer stopProcess(proc)
	}

	opts := Options{
		Catalog:      cat,
		LibraryFiles: files,
		LibraryRoot:  root,
		PlayerNote:   playerNote,
		Resolver:     catalog.NewClient(),
	}
	if client != nil {
		opts.Player = client
	}

	program := tea.NewProgram(NewModel(opts), tea.WithAltScreen())
	_, err = program.Run()
	return err
}
