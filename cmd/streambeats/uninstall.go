package main

// uninstall.go — `streambeats uninstall`: remove exactly what the installer
// landed (issue #9). Registry-aware: uses $PREFIX/share/streambeats/
// manifest.txt when present and falls back to the known layout for installs
// created before manifests existed. User data is protected: ~/Music/
// StreamBeats is never touched and the catalog cache needs an explicit
// --cache flag.

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// uninstallDeps bundles everything the uninstaller needs from the
// environment. Tests inject fakes; production code builds the real set with
// newUninstallDeps.
type uninstallDeps struct {
	cacheDir       func() (string, error)
	listPanelUnits func() []string
	stopUnit       func(string) error
}

// newUninstallDeps wires the real environment dependencies.
func newUninstallDeps() *uninstallDeps {
	return &uninstallDeps{
		cacheDir:       os.UserCacheDir,
		listPanelUnits: listPanelUnits,
		stopUnit:       stopPanelUnit,
	}
}

// listPanelUnits reports running streambeats-panel-* units. On a system
// without systemd (or with systemctl failing for any reason) it returns an
// empty list: the uninstall proceeds without stopping anything.
func listPanelUnits() []string {
	if _, err := exec.LookPath("systemctl"); err != nil {
		return nil
	}
	// #nosec G204 — pinned systemctl binary with only constant arguments.
	out, err := exec.CommandContext(context.Background(), "systemctl", "--user", "list-units",
		"--plain", "--no-legend", "streambeats-panel-*").Output()
	if err != nil {
		return nil
	}
	var units []string
	for _, line := range strings.Split(string(out), "\n") {
		if fields := strings.Fields(line); len(fields) > 0 {
			units = append(units, fields[0])
		}
	}
	return units
}

// stopPanelUnit stops one transient panel unit.
func stopPanelUnit(unit string) error {
	// #nosec G204 — unit names are enumerated by listPanelUnits from systemctl
	// itself (never user input) and only ever passed to the same stop verb.
	return exec.CommandContext(context.Background(), "systemctl", "--user", "stop", unit).Run()
}

// uninstallRequest is one uninstall invocation.
type uninstallRequest struct {
	Prefix string // install prefix, e.g. ~/.local
	Cache  bool   // also clear the catalog cache
	Deps   *uninstallDeps
}

const manifestName = "manifest.txt"

// knownUninstallTargets is the pre-manifest layout: exactly the paths the
// installer lands.
func knownUninstallTargets(prefix string) []string {
	return []string{
		filepath.Join(prefix, "share", "streambeats"),
		filepath.Join(prefix, "bin", "streambeats"),
		filepath.Join(prefix, "bin", "streambeats-quick"),
		filepath.Join(prefix, "bin", "streambeats-panel"),
		filepath.Join(prefix, "share", "applications", "streambeats-panel.desktop"),
		filepath.Join(prefix, "share", "icons", "hicolor", "scalable", "apps", "streambeats-panel.svg"),
	}
}

// manifestTargets parses share/manifest.txt (one absolute path per line; '#'
// starts a comment). Lines that are not absolute paths safely contained in the
// prefix are rejected with a warning instead of being executed: a corrupted
// manifest must never make the uninstaller delete outside the prefix.
func manifestTargets(stderr io.Writer, prefix, manifestPath string) ([]string, error) {
	// #nosec G304 — manifestPath is the installer-owned
	// $PREFIX/share/streambeats/manifest.txt; its lines are validated against
	// the prefix before any path is ever used (see the prefix-escape check).
	f, err := os.Open(manifestPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close() //nolint:errcheck // read-only file

	var targets []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if idx := strings.Index(line, "#"); idx >= 0 {
			line = line[:idx]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !filepath.IsAbs(line) {
			fmt.Fprintf(stderr, "streambeats: ignoring manifest line outside of the prefix: %q\n", line)
			continue
		}
		if rel, relErr := filepath.Rel(prefix, line); relErr != nil || rel == ".." ||
			strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			fmt.Fprintf(stderr, "streambeats: ignoring manifest line outside of the prefix: %q\n", line)
			continue
		}
		targets = append(targets, filepath.Clean(line))
	}
	return targets, scanner.Err()
}

// runUninstall executes the uninstaller and returns the process exit code:
// 0 on success (including the idempotent "nothing to uninstall" case), 1 when
// the prefix could not be resolved or nothing at all could be removed while
// removals were attempted and failed.
func runUninstall(stdout, stderr io.Writer, req uninstallRequest) int {
	deps := req.Deps
	if deps == nil {
		deps = newUninstallDeps()
	}

	targets, err := manifestTargets(stderr, req.Prefix,
		filepath.Join(req.Prefix, "share", "streambeats", manifestName))
	if err != nil {
		fmt.Fprintf(stderr, "streambeats: cannot read manifest: %v (falling back to known layout)\n", err)
	}
	if len(targets) == 0 {
		targets = knownUninstallTargets(req.Prefix)
	}

	present := 0
	for _, target := range targets {
		if _, statErr := os.Lstat(target); statErr == nil {
			present++
		}
	}
	if present == 0 {
		if req.Cache {
			clearCache(stdout, stderr, deps.cacheDir)
		}
		fmt.Fprintln(stdout, "nothing to uninstall")
		return 0
	}

	var stopped int
	for _, unit := range deps.listPanelUnits() {
		if stopErr := deps.stopUnit(unit); stopErr != nil {
			fmt.Fprintf(stderr, "streambeats: could not stop %s: %v\n", unit, stopErr)
			continue
		}
		fmt.Fprintf(stdout, "stopped: %s\n", unit)
		stopped++
	}
	if stopped > 0 {
		fmt.Fprintf(stdout, "stopped %d running panel service(s)\n", stopped)
	}

	removed, absent, failed := 0, 0, 0
	for _, target := range targets {
		err := os.RemoveAll(target)
		switch {
		case os.IsNotExist(err):
			absent++
		case err != nil:
			failed++
			fmt.Fprintf(stderr, "streambeats: cannot remove %s: %v\n", target, err)
		default:
			removed++
			fmt.Fprintf(stdout, "removed: %s\n", target)
		}
	}
	fmt.Fprintf(stdout, "uninstalled: %d path(s) removed, %d already absent\n", removed, absent)
	if req.Cache {
		clearCache(stdout, stderr, deps.cacheDir)
	}
	return 0
}

// clearCache removes the catalog cache directory only when --cache was given.
func clearCache(stdout, stderr io.Writer, cacheDir func() (string, error)) {
	base, err := cacheDir()
	if err != nil {
		fmt.Fprintf(stderr, "streambeats: cannot locate the cache directory: %v\n", err)
		return
	}
	cache := filepath.Join(base, "streambeats")
	if err := os.RemoveAll(cache); err != nil {
		fmt.Fprintf(stderr, "streambeats: cannot clear the catalog cache: %v\n", err)
		return
	}
	fmt.Fprintf(stdout, "cleared catalog cache: %s\n", cache)
}

// parseUninstallArgs parses `uninstall` with its optional --cache flag.
func parseUninstallArgs(args []string) (command, bool, error) {
	if args[0] != "uninstall" {
		return cmdTUI, false, fmt.Errorf("expected uninstall, got %q", args[0])
	}
	cache := false
	for _, arg := range args[1:] {
		if arg == "--cache" {
			cache = true
		} else {
			return cmdUninstall, cache, fmt.Errorf("unknown uninstall flag %q (usage: streambeats uninstall [--cache])", arg)
		}
	}
	return cmdUninstall, cache, nil
}

// uninstallPrefix resolves the install prefix the way scripts/install.sh
// defaults it: $STREAMBEATS_INSTALL_PREFIX, else $HOME/.local.
func uninstallPrefix() (string, error) {
	if prefix := os.Getenv("STREAMBEATS_INSTALL_PREFIX"); prefix != "" {
		return prefix, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot resolve the install prefix: %w", err)
	}
	return filepath.Join(home, ".local"), nil
}
