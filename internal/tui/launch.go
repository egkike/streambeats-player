package tui

// Launcher: wires the catalog, local library and mpv backend into a Bubbletea
// program. Spawning mpv is best effort so the player stays browsable when the
// binary (or its socket) is unavailable.

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	gterm "github.com/charmbracelet/x/term"

	"github.com/charmbracelet/lipgloss"

	"streambeats/internal/catalog"
	"streambeats/internal/player"
	"streambeats/internal/settings"
)

// debugInputLog opens the diagnostic input log when STREAMBEATS_DEBUG_INPUT
// is set: raw input bytes are appended there via the returned tee reader and
// the same file receives one line per tea.Msg through the model hook.
// Disabled by default; everything lives in the user's own cache dir.
func debugInputLog() (*os.File, io.Reader, func(string)) {
	if os.Getenv("STREAMBEATS_DEBUG_INPUT") == "" {
		return nil, nil, nil
	}
	base, err := os.UserCacheDir()
	if err != nil {
		return nil, nil, nil
	}
	path := filepath.Join(base, "streambeats", "input-debug.log")
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, nil, nil
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600) //nolint:gosec // G304: path is the user's own cache dir (os.UserCacheDir + fixed name); never attacker-supplied
	if err != nil {
		return nil, nil, nil
	}
	stamp := func() string { return time.Now().Format("15:04:05.000") }
	logf := func(msg string) {
		fmt.Fprintf(f, "%s MSG %s\n", stamp(), msg)
	}
	fmt.Fprintf(f, "%s === debug-input session ===\n", stamp())
	return f, io.TeeReader(os.Stdin, &debugRawWriter{f: f}), logf
}

// rawStdinState puts the terminal in raw mode for the diagnostic kit (a
// non-File reader hides the TTY from bubbletea, so we own it) and returns
// the saved state for restore. nil when stdin is not a TTY (tests, units).
func rawStdinState() *gterm.State {
	state, err := gterm.MakeRaw(os.Stdin.Fd())
	if err != nil {
		return nil // not a TTY: canonical mode, the kit degrades gracefully
	}
	return state
}

// debugRawWriter logs every chunk of terminal input before bubbletea parses
// it, so a real-device click shows the exact bytes.
type debugRawWriter struct{ f *os.File }

func (w *debugRawWriter) Write(b []byte) (int, error) {
	if len(b) == 0 {
		return 0, nil
	}
	fmt.Fprintf(w.f, "%s RAW % x\n", time.Now().Format("15:04:05.000"), b)
	return len(b), nil
}

// Launch boots the TUI with the given catalog and blocks until the user quits.
// scrapeCmd is non-nil only on a fresh cache (issue #75): the TUI boots with
// an empty list and a live-scrape status; the cmd populates it in place.
func Launch(cat *catalog.Catalog, scrapeCmd tea.Cmd) error {
	// Library root (issue #76): the persisted choice wins; otherwise the XDG
	// user music dir (es systems: Música; en: Music) resolves the default,
	// which is then persisted so later xdg changes cannot surprise the user.
	settingsStore, settingsErr := settings.DefaultStore()
	var saved settings.Settings
	if settingsErr == nil {
		if user, err := settingsStore.Load(); err == nil {
			saved = user
		}
	}
	root := saved.LibraryRoot
	if root == "" {
		root, _ = catalog.MusicDir()
		if settingsErr == nil {
			saved.LibraryRoot = root
			_ = settingsStore.Save(saved) // best-effort; defaults stay resolved
		}
	}

	var dirs, files []string
	if root != "" {
		dirs, files, _ = catalog.ListDir(root)
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
	theme := saved.Theme
	opts := Options{
		Catalog:       cat,
		ScrapeCmd:     scrapeCmd,
		LibraryDir:    root,
		LibDirs:       dirs,
		LibraryFiles:  files,
		LibraryRoot:   root,
		PlayerNote:    playerNote,
		Resolver:      catalog.NewClient(),
		Theme:         theme,
		ThemeDark:     lipgloss.HasDarkBackground(),
		SettingsStore: settingsStore,
		// Terminal-adaptive transport (issue #64): read ONCE here — only the
		// exact "ghostty" keeps the calibrated keycaps; anything else fails
		// closed to the ASCII tokens.
		AsciiTransport: !isGhosttyTerminal(os.Getenv("TERM_PROGRAM")),
	}
	if client != nil {
		opts.Player = client
	}

	// Phase B (issue #37): cell-motion mouse enables the transport button
	// clicks (S1); AllMotion is overkill here (no hover interactions).
	// Diagnostic input kit (STREAMBEATS_DEBUG_INPUT): never on by default.
	debugFile, debugReader, debugLog := debugInputLog()
	if debugFile != nil {
		defer func() { _ = debugFile.Close() }()
		opts.Debug = debugLog
		if st := rawStdinState(); st != nil {
			defer func() { _ = gterm.Restore(os.Stdin.Fd(), st) }()
		}
	}
	// Solo Teclado (issue #60): the keyboard is the sole input — the panel
	// doesn't even request mouse reports from the terminal.
	programOpts := []tea.ProgramOption{tea.WithAltScreen()}
	if debugReader != nil {
		// Tee the terminal input BEFORE bubbletea parses it: the log then
		// holds the exact bytes the real terminal sent per click.
		programOpts = append(programOpts, tea.WithInput(debugReader))
	}
	program := tea.NewProgram(NewModel(opts), programOpts...)
	_, err = program.Run()
	return err
}
