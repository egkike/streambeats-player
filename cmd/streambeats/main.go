package main

// streambeats is a floating quick-terminal TUI music player for the
// StreamBeats catalog (https://streambeats.bandcamp.com/music).
//
// Subcommands:
//
//	streambeats          launch the TUI (default)
//	streambeats refresh  scrape the catalog into the local cache
//	streambeats sync     download every playable album to the local library
//	streambeats help     show usage

import (
	"errors"
	"fmt"
	"io"
	"os"

	"streambeats/internal/catalog"
	"streambeats/internal/tui"
)

const usage = `streambeats — terminal music player for StreamBeats

Usage:
  streambeats            launch the TUI player (default)
  streambeats refresh    scrape the catalog and update the local cache
  streambeats sync       download every playable album to the local library
  streambeats uninstall [--cache]
                          remove everything the installer landed; user data
                          (local library, cache without --cache) stays
  streambeats help       show this help

Keys (in the TUI):
  j/k or up/down   move in the lists
  enter            open the selected album / play the selected track
  esc              go back
  l                toggle the local library list
  space            pause / resume
  s                stop playback
  + / -            volume up / down
  q  or ctrl+c     quit

Files:
  catalog cache  $XDG_CACHE_HOME/streambeats/catalog.json
  local library  ~/Music/StreamBeats
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// command is one parsed CLI subcommand.
type command int

const (
	cmdTUI command = iota
	cmdRefresh
	cmdSync
	cmdUninstall
	cmdHelp
)

// parseCommand is pure: it maps CLI arguments to a subcommand.
func parseCommand(args []string) (command, error) {
	if len(args) == 0 {
		return cmdTUI, nil
	}
	// uninstall carries its own flag set (--cache, issue #9): delegate BEFORE
	// the generic >1-arguments guard so its flags reach the uninstaller
	// (regression for review finding R3-001).
	if args[0] == "uninstall" {
		cmd, _, err := parseUninstallArgs(args)
		return cmd, err
	}
	if len(args) > 1 {
		return cmdTUI, fmt.Errorf("expected at most one command, got %d arguments", len(args))
	}
	switch args[0] {
	case "tui":
		return cmdTUI, nil
	case "refresh":
		return cmdRefresh, nil
	case "sync":
		return cmdSync, nil
	case "help", "-h", "--help":
		return cmdHelp, nil
	default:
		return cmdTUI, fmt.Errorf("unknown command %q", args[0])
	}
}

// run executes the CLI and returns the process exit code.
func run(args []string, stdout, stderr io.Writer) int {
	cmd, err := parseCommand(args)
	if err != nil {
		fmt.Fprintf(stderr, "streambeats: %v\n\n%s", err, usage)
		return 2
	}
	switch cmd {
	case cmdHelp:
		fmt.Fprint(stdout, usage)
		return 0
	case cmdRefresh:
		return runRefresh(stdout, stderr)
	case cmdSync:
		return runSync(stdout, stderr)
	case cmdUninstall:
		_, cache, parseErr := parseUninstallArgs(args)
		if parseErr != nil {
			fmt.Fprintf(stderr, "streambeats: %v\n\n%s", parseErr, usage)
			return 2
		}
		deps := newUninstallDeps()
		prefix, err := uninstallPrefix(deps)
		if err != nil {
			return fail(stderr, err)
		}
		return runUninstall(stdout, stderr, uninstallRequest{Prefix: prefix, Cache: cache, Deps: deps})
	default:
		return runTUI(stdout, stderr, newTUIDeps())
	}
}

// runRefresh scrapes the catalog and persists it in the cache store.
func runRefresh(stdout, stderr io.Writer) int {
	store, err := catalog.DefaultStore()
	if err != nil {
		return fail(stderr, err)
	}
	cat, err := catalog.NewClient().Refresh()
	if err != nil {
		return fail(stderr, fmt.Errorf("refresh catalog: %w", err))
	}
	if err := store.Save(cat); err != nil {
		return fail(stderr, fmt.Errorf("save catalog: %w", err))
	}
	fmt.Fprintf(stdout, "catalog refreshed: %d albums, %d tracks\n", len(cat.Albums), totalTracks(cat))
	fmt.Fprintf(stdout, "cache: %s\n", store.Path)
	return 0
}

// runSync downloads every playable album into the local library, reporting
// per-album downloaded/skipped counts.
func runSync(stdout, stderr io.Writer) int {
	store, err := catalog.DefaultStore()
	if err != nil {
		return fail(stderr, err)
	}
	cat, err := store.Load()
	if errors.Is(err, os.ErrNotExist) {
		fmt.Fprintln(stdout, "no cached catalog yet; scraping it now...")
		cat, err = catalog.NewClient().Refresh()
		if err == nil {
			if saveErr := store.Save(cat); saveErr != nil {
				fmt.Fprintf(stderr, "warning: catalog cache not saved: %v\n", saveErr)
			}
		}
	}
	if err != nil {
		return fail(stderr, fmt.Errorf("load catalog (try `streambeats refresh`): %w", err))
	}

	root, err := catalog.DefaultLibraryPath()
	if err != nil {
		return fail(stderr, err)
	}
	fmt.Fprintf(stdout, "library: %s\n", root)

	client := catalog.NewClient()
	albums := len(cat.Albums)
	downloaded, skipped, failed := 0, 0, 0
	for i, album := range cat.Albums {
		result, err := client.SyncAlbum(root, i+1, album.Slug)
		if err != nil {
			failed++
			fmt.Fprintf(stderr, "[%d/%d] %s: failed: %v\n", i+1, albums, album.Title, err)
			continue
		}
		downloaded += result.Downloaded
		skipped += result.Skipped
		fmt.Fprintf(stdout, "[%d/%d] %02d - %s: downloaded %d, skipped %d\n",
			i+1, albums, i+1, album.Title, result.Downloaded, result.Skipped)
	}
	fmt.Fprintf(stdout, "sync done: %d albums, %d downloaded, %d skipped, %d failed\n",
		albums-failed, downloaded, skipped, failed)
	if failed > 0 {
		return 1
	}
	return 0
}

// tuiDeps bundles theailleurs environment-dependent collaborators of the TUI path
// so tests inject temp stores, a stubbed catalog client and a no-op launch
// (mirror of the uninstallDeps pattern).
type tuiDeps struct {
	store   *catalog.Store
	refresh func() (*catalog.Catalog, error)
	launch  func(*catalog.Catalog) error
}

// newTUIDeps wires the real environment collaborators.
func newTUIDeps() *tuiDeps {
	client := catalog.NewClient()
	return &tuiDeps{
		refresh: client.Refresh,
		launch:  tui.Launch,
	}
}

// runTUI loads the cached catalog and launches the Bubbletea player. On a
// fresh install (issue #17) the same fallback as runSync applies: the cache
// is missing → scrape live, save, launch. launch is a function so tests
// never spawn the terminal fugitive in production either — single clear call.
func runTUI(stdout, stderr io.Writer, deps *tuiDeps) int {
	if deps == nil {
		deps = newTUIDeps()
	}
	var store *catalog.Store
	var err error
	if deps.store != nil {
		store = deps.store
	} else {
		store, err = catalog.DefaultStore()
		if err != nil {
			return fail(stderr, err)
		}
	}
	cat, err := store.Load()
	if errors.Is(err, os.ErrNotExist) {
		fmt.Fprintln(stdout, "no cached catalog yet; scraping it now...")
		cat, err = deps.refresh()
		if err != nil {
			return fail(stderr, fmt.Errorf("refresh catalog: %w", err))
		}
		if saveErr := store.Save(cat); saveErr != nil {
			fmt.Fprintf(stderr, "warning: catalog cache not saved: %v\n", saveErr)
		}
	}
	if err != nil {
		return fail(stderr, fmt.Errorf("load catalog: %w", err))
	}
	if err := deps.launch(cat); err != nil {
		return fail(stderr, fmt.Errorf("run tui: %w", err))
	}
	return 0
}

// totalTracks counts every track in the catalog.
func totalTracks(cat *catalog.Catalog) int {
	if cat == nil {
		return 0
	}
	total := 0
	for _, album := range cat.Albums {
		total += len(album.Tracks)
	}
	return total
}

// fail prints an error and returns a non-zero exit code.
func fail(stderr io.Writer, err error) int {
	fmt.Fprintf(stderr, "streambeats: %v\n", err)
	return 1
}
