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
	execPath       func() (string, error)
}

// newUninstallDeps wires the real environment dependencies.
func newUninstallDeps() *uninstallDeps {
	return &uninstallDeps{
		cacheDir:       os.UserCacheDir,
		listPanelUnits: listPanelUnits,
		stopUnit:       stopPanelUnit,
		execPath:       os.Executable,
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
// manifest must never make the uninstaller delete outside the prefix. The
// second return describes the source that produced the targets, for the S4
// observability notice; an empty string means no manifest exists at all.
func manifestTargets(stderr io.Writer, prefix, manifestPath string) ([]string, string, error) {
	// #nosec G304 — manifestPath is the installer-owned
	// $PREFIX/share/streambeats/manifest.txt; its lines are validated against
	// the prefix before any path is ever used (see the prefix-escape check).
	f, err := os.Open(manifestPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, "", nil // no manifest on disk; caller announces the fallback
		}
		return nil, "", err
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
			fmt.Fprintf(stderr, "streambeats: ignoring manifest line that is not an absolute path: %q\n", line)
			continue
		}
		if rel, relErr := filepath.Rel(prefix, line); relErr != nil || rel == ".." ||
			strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			fmt.Fprintf(stderr, "streambeats: ignoring manifest line outside the prefix: %q\n", line)
			continue
		}
		targets = append(targets, filepath.Clean(line))
	}
	return targets, manifestName, scanner.Err()
}

// symlinkEscapeFilter drops targets whose symlink-resolved location escapes
// the prefix (R1-manifest-path-scope defense-in-depth: the lexical Rel check
// cannot see through in-tree symlinks). Rejected entries are left untouched
// with a warning; absent targets are kept because the removal loop counts
// them as already absent.
func symlinkEscapeFilter(stderr io.Writer, prefix string, targets []string) []string {
	// R4-001 correction: with a symlinked install prefix, targets resolve to
	// the prefix's real path but the containment comparison would use the
	// unresolved prefix and drop every valid target. Resolve the prefix once;
	// a resolution failure falls back to the lexical prefix (as before).
	resolvedPrefix, perr := filepath.EvalSymlinks(prefix)
	if resolvedPrefix == "" || perr != nil {
		// R3-001 follow-up: this fallback re-enables the lexical-only
		// containment the R4-001 fix removed — it must never happen silently.
		fmt.Fprintf(stderr, "streambeats: cannot resolve the install prefix (%v) — using the lexical containment check on %s\n", perr, prefix)
		resolvedPrefix = prefix
	}
	kept := make([]string, 0, len(targets))
	for _, target := range targets {
		resolved, err := filepath.EvalSymlinks(target)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				kept = append(kept, target)
				continue
			}
			// R3-001 correction: an uninspectable path (permission denial) is
			// NOT absence — keep it so the removal attempt surfaces the failure
			// (exit 1), instead of silently dropping it as "already absent".
			fmt.Fprintf(stderr, "streambeats: manifest path cannot be resolved: %s: %v\n", target, err)
			// R3-001: containment is unknowable for an uninspectable path —
			// keep the lexically validated target and let the removal attempt
			// below surface the real failure (empty `resolved` would fail Rel).
			resolved = target
		}
		if rel, relErr := filepath.Rel(resolvedPrefix, resolved); relErr != nil || rel == ".." ||
			strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			fmt.Fprintf(stderr, "streambeats: ignoring manifest path resolving outside the prefix: %s -> %s\n", target, resolved)
			continue
		}
		kept = append(kept, target)
	}
	return kept
}

// runUninstall executes the uninstaller and returns the process exit code:
// 0 on success (including the idempotent "nothing to uninstall" case), 1 when
// the prefix could not be resolved or removals were attempted and any of them
// failed (S2 — the contract this function documents is enforced, not merely
// described).
func runUninstall(stdout, stderr io.Writer, req uninstallRequest) int {
	deps := req.Deps
	if deps == nil {
		deps = newUninstallDeps()
	}

	fallbackNotice := fmt.Sprintf("using targets: known install layout — %d paths", len(knownUninstallTargets(req.Prefix)))
	targets, source, err := manifestTargets(stderr, req.Prefix,
		filepath.Join(req.Prefix, "share", "streambeats", manifestName))
	if err != nil {
		fmt.Fprintf(stderr, "streambeats: cannot read manifest: %v (falling back to known layout)\n", err)
		targets, source = nil, ""
	}
	switch {
	case source != "" && len(targets) > 0:
		fmt.Fprintf(stdout, "using targets: install manifest — %d entries\n", len(targets))
	case source != "" && err == nil:
		// R4-001 follow-up: the manifest exists but parsed to nothing usable
		// (all lines rejected) — announce the fallback instead of letting the
		// earlier per-line warnings imply the manifest governed the removal.
		fmt.Fprintf(stderr, "streambeats: manifest present but yielded no usable targets (falling back to known layout)\n")
		targets = knownUninstallTargets(req.Prefix)
		fmt.Fprintln(stdout, fallbackNotice)
	default:
		targets = knownUninstallTargets(req.Prefix)
		fmt.Fprintln(stdout, fallbackNotice)
	}
	targets = symlinkEscapeFilter(stderr, req.Prefix, targets)

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
		// S1 (R3-002) with the R3-001 boundary: only a genuine ENOENT counts
		// the target as already absent; any other stat failure still reaches
		// RemoveAll so its error surfaces as a counted failure.
		if _, statErr := os.Lstat(target); statErr != nil && errors.Is(statErr, os.ErrNotExist) {
			absent++
			continue
		}
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
	fmt.Fprintf(stdout, "uninstalled: %d path(s) removed, %d already absent, %d failed\n", removed, absent, failed)
	if req.Cache {
		clearCache(stdout, stderr, deps.cacheDir)
	}
	if failed > 0 {
		return 1 // S2: partial-failure uninstaller must not report success
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

// uninstallPrefix resolves the install prefix, in priority order:
// $STREAMBEATS_INSTALL_PREFIX, then the prefix of the RUNNING binary (the
// installed binary lives at $PREFIX/share/streambeats/bin/streambeats, so a
// custom-prefix install removes itself without extra env — review finding
// R4-001), then the installer default $HOME/.local.
func uninstallPrefix(deps *uninstallDeps) (string, error) {
	if prefix := os.Getenv("STREAMBEATS_INSTALL_PREFIX"); prefix != "" {
		return prefix, nil
	}
	if deps == nil {
		deps = newUninstallDeps()
	}
	if exe, exeErr := deps.execPath(); exeErr == nil {
		if prefix := prefixFromBinaryPath(exe); prefix != "" {
			return prefix, nil
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot resolve the install prefix: %w", err)
	}
	return filepath.Join(home, ".local"), nil
}

// prefixFromBinaryPath reverses the installed layout
// <prefix>/share/streambeats/bin/<name> and only accepts the layout the
// installer actually builds (a bin/ folder inside a streambeats/ share dir);
// anything else (dev builds, arbitrary look-alikes) yields "".
func prefixFromBinaryPath(exe string) string {
	bin := filepath.Dir(exe)
	appDir := filepath.Dir(bin)
	shareDir := filepath.Dir(appDir)
	if filepath.Base(bin) != "bin" || filepath.Base(appDir) != "streambeats" || filepath.Base(shareDir) != "share" {
		return ""
	}
	return filepath.Dir(shareDir)
}
