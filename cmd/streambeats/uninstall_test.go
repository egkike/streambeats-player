package main

// Unit tests for `streambeats uninstall`. Nothing touches real systemd, the
// user's real HOME, or the network: every filesystem interaction runs inside a
// t.TempDir prefix, and the panel-unit and cache-dir dependencies are injected.

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// installTree builds a fake installed layout inside prefix, mirroring
// scripts/install.sh: the bundle under share/ plus the wrapper, launcher, panel
// wrapper, generated .desktop and hicolor icon. manifest records the exact
// paths installed; extraPaths are files NOT managed by the installer.
func installTree(t *testing.T, manifest []string, extraPaths ...string) string {
	t.Helper()
	share := filepath.Join(prefixBuilt, "share", "streambeats")
	manifestDirs := map[string]string{share: "directory"} // registry dir targets
	files := map[string]string{
		filepath.Join(share, "bin", "streambeats"):                                                           "#!/bin/sh\nexec echo tui\n",
		filepath.Join(share, "scripts", "streambeats-quick"):                                                 "#!/bin/sh\n",
		filepath.Join(share, "assets", "ghostty", "right.conf"):                                              "windows = \"streambeats-panel-right\"\n",
		filepath.Join(prefixBuilt, "bin", "streambeats"):                                                     "#!/bin/sh wrapper\n",
		filepath.Join(prefixBuilt, "bin", "streambeats-quick"):                                               "#!/bin/sh wrapper\n",
		filepath.Join(prefixBuilt, "bin", "streambeats-panel"):                                               "#!/bin/sh wrapper\n",
		filepath.Join(prefixBuilt, "share", "applications", "streambeats-panel.desktop"):                     "[Desktop Entry]\n",
		filepath.Join(prefixBuilt, "share", "icons", "hicolor", "scalable", "apps", "streambeats-panel.svg"): "<svg/>",
	}
	for _, p := range manifest {
		q := normalizeRelative(p)
		if _, dir := manifestDirs[q]; dir {
			if err := os.MkdirAll(q, 0o755); err != nil {
				t.Fatal(err)
			}
			continue
		}
		files[q] = "#!/bin/sh managed\n"
	}
	for _, p := range extraPaths {
		files[filepath.Join(fakeRoot, p)] = "USER FILE\n"
	}
	if len(manifest) > 0 {
		if err := os.MkdirAll(share, 0o755); err != nil {
			t.Fatal(err)
		}
		lines := "# streambeats install manifest\n" + strings.Join(manifest, "\n") + "\n"
		if err := os.WriteFile(filepath.Join(share, "manifest.txt"), []byte(lines), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for path, content := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return prefixBuilt
}

// normalizeRelative maps manifest lines (paths under the install prefix) onto
// the fake tree that installTree writes. Manifest lines are absolute as the
// real installer records them; tests keep everything under a temp root.
func normalizeRelative(p string) string {
	return filepath.Join(fakeRoot, strings.TrimPrefix(p, fakeRoot))
}

var fakeRoot string
var prefixBuilt string

func testEnv(t *testing.T) (stdout, stderr *bytes.Buffer, deps *uninstallDeps, stopped *[]string) {
	t.Helper()
	fakeRoot = t.TempDir()
	prefixBuilt = filepath.Join(fakeRoot, "local")
	stopCalls := &[]string{}
	deps = &uninstallDeps{
		cacheDir:       func() (string, error) { return filepath.Join(fakeRoot, "cache"), nil },
		listPanelUnits: func() []string { return []string{"streambeats-panel-right.service"} },
		stopUnit:       func(unit string) error { *stopCalls = append(*stopCalls, unit); return nil },
	}
	return &bytes.Buffer{}, &bytes.Buffer{}, deps, stopCalls
}

func TestUninstallRemovesExactlyManifestPaths(t *testing.T) {
	stdout, stderr, deps, _ := testEnv(t)
	installTree(t, []string{
		prefixBuilt + "/share/streambeats",
		prefixBuilt + "/bin/streambeats",
		prefixBuilt + "/bin/streambeats-quick",
		prefixBuilt + "/bin/streambeats-panel",
		prefixBuilt + "/share/applications/streambeats-panel.desktop",
		prefixBuilt + "/share/icons/hicolor/scalable/apps/streambeats-panel.svg",
	}, "/foreign.txt", filepath.Join(prefixBuilt, "share", "streambeats", "notes.txt"))

	code := runUninstall(stdout, stderr, uninstallRequest{Prefix: prefixBuilt, Deps: deps})
	if code != 0 {
		t.Fatalf("uninstall exit code = %d, want 0 (stderr: %s)", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(fakeRoot, "foreign.txt")); err != nil {
		t.Fatalf("uninstaller deleted a path outside its manifest/plan: %v", err)
	}
	if _, err := os.Stat(filepath.Join(prefixBuilt, "share", "streambeats", "notes.txt")); err == nil {
		t.Fatal("file not listed in the manifest survived inside a removed share dir")
	}
	for _, want := range []string{
		filepath.Join(prefixBuilt, "share", "streambeats"),
		filepath.Join(prefixBuilt, "bin", "streambeats-panel"),
		filepath.Join(prefixBuilt, "share", "icons", "hicolor", "scalable", "apps", "streambeats-panel.svg"),
	} {
		if _, err := os.Stat(want); !os.IsNotExist(err) {
			t.Fatalf("manifest path still present after uninstall: %s", want)
		}
	}
	for _, keep := range []string{filepath.Join(prefixBuilt, "bin"), filepath.Join(prefixBuilt, "share")} {
		if _, err := os.Stat(keep); err != nil {
			t.Fatalf("uninstaller removed a parent directory it did not install: %s", keep)
		}
	}
	if !strings.Contains(stdout.String(), "uninstalled") {
		t.Fatalf("stdout missing removal summary:\n%s", stdout.String())
	}
}

func TestUninstallFallsBackToKnownLayoutWithoutManifest(t *testing.T) {
	stdout, stderr, deps, _ := testEnv(t)
	installTree(t, nil /* no manifest */, "/foreign.txt")

	code := runUninstall(stdout, stderr, uninstallRequest{Prefix: prefixBuilt, Deps: deps})
	if code != 0 {
		t.Fatalf("uninstall exit code = %d, want 0 (stderr: %s)", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(fakeRoot, "foreign.txt")); err != nil {
		t.Fatalf("fallback uninstaller deleted a foreign path: %v", err)
	}
	for _, want := range []string{
		filepath.Join(prefixBuilt, "share", "streambeats"),
		filepath.Join(prefixBuilt, "bin", "streambeats"),
		filepath.Join(prefixBuilt, "bin", "streambeats-panel"),
		filepath.Join(prefixBuilt, "share", "applications", "streambeats-panel.desktop"),
	} {
		if _, err := os.Stat(want); !os.IsNotExist(err) {
			t.Fatalf("known-layout path still present after uninstall: %s", want)
		}
	}
}

func TestUninstallStopsRunningPanelUnits(t *testing.T) {
	stdout, stderr, deps, stopped := testEnv(t)
	installTree(t, nil)

	code := runUninstall(stdout, stderr, uninstallRequest{Prefix: prefixBuilt, Deps: deps})
	if code != 0 {
		t.Fatalf("uninstall exit code = %d, want 0", code)
	}
	if len(*stopped) != 1 || (*stopped)[0] != "streambeats-panel-right.service" {
		t.Fatalf("expected the panel unit stopped before deletion, got %v", *stopped)
	}
	if !strings.Contains(stdout.String(), "streambeats-panel-right.service") {
		t.Fatalf("stdout should report the stopped unit:\n%s", stdout.String())
	}
}

func TestUninstallNothingToUninstallIsIdempotent(t *testing.T) {
	stdout, stderr, deps, _ := testEnv(t)
	if err := os.MkdirAll(filepath.Join(prefixBuilt, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}

	code := runUninstall(stdout, stderr, uninstallRequest{Prefix: prefixBuilt, Deps: deps})
	if code != 0 {
		t.Fatalf("nothing-installed exit code = %d, want 0 (stderr: %s)", code, stderr.String())
	}
	if !strings.Contains(strings.ToLower(stdout.String()), "nothing to uninstall") {
		t.Fatalf("stdout = %q, want a nothing-to-uninstall summary", stdout.String())
	}
}

func TestUninstallKeepsUserDataByDefault(t *testing.T) {
	stdout, stderr, deps, _ := testEnv(t)
	installTree(t, nil)
	cache := filepath.Join(fakeRoot, "cache", "streambeats")
	if err := os.MkdirAll(cache, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cache, "catalog.json"), []byte("[]"), 0o644); err != nil {
		t.Fatal(err)
	}

	code := runUninstall(stdout, stderr, uninstallRequest{Prefix: prefixBuilt, Deps: deps})
	if code != 0 {
		t.Fatalf("uninstall exit code = %d, want 0", code)
	}
	if _, err := os.Stat(filepath.Join(cache, "catalog.json")); err != nil {
		t.Fatalf("catalog cache was removed without --cache: %v", err)
	}
}

func TestUninstallCacheFlagClearsOnlyTheCache(t *testing.T) {
	stdout, stderr, deps, _ := testEnv(t)
	installTree(t, nil)
	cache := filepath.Join(fakeRoot, "cache", "streambeats")
	if err := os.MkdirAll(cache, 0o755); err != nil {
		t.Fatal(err)
	}

	code := runUninstall(stdout, stderr, uninstallRequest{Prefix: prefixBuilt, Cache: true, Deps: deps})
	if code != 0 {
		t.Fatalf("uninstall exit code = %d, want 0", code)
	}
	if _, err := os.Stat(filepath.Join(fakeRoot, "cache", "streambeats")); !os.IsNotExist(err) {
		t.Fatal("--cache did not clear the catalog cache")
	}
	if _, err := os.Stat(filepath.Join(fakeRoot, "cache")); err != nil {
		t.Fatalf("--cache removed the whole cache root, want only streambeats/: %v", err)
	}
	if !strings.Contains(stdout.String(), "cache") {
		t.Fatalf("stdout should mention the cache clearing:\n%s", stdout.String())
	}
}

func TestUninstallRejectsDotDotManifestLines(t *testing.T) {
	stdout, stderr, deps, _ := testEnv(t)
	share := filepath.Join(prefixBuilt, "share", "streambeats")
	installTree(t, nil)
	if err := os.WriteFile(filepath.Join(share, "manifest.txt"), []byte("../../host-owned\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	hostFile := filepath.Join(fakeRoot, "host-owned")
	if err := os.WriteFile(hostFile, []byte("SECRET"), 0o644); err != nil {
		t.Fatal(err)
	}

	code := runUninstall(stdout, stderr, uninstallRequest{Prefix: prefixBuilt, Deps: deps})
	if code != 0 {
		t.Fatalf("uninstall exit code = %d, want 0 (path escape is skipped, not fatal)", code)
	}
	if _, err := os.Stat(hostFile); err != nil {
		t.Fatalf("manifest ..-line executed outside the prefix: %v", err)
	}
	if !strings.Contains(stderr.String(), "manifest line") {
		t.Fatalf("stderr should warn about the rejected manifest line:\n%s", stderr.String())
	}
}

func TestParseUninstallCommand(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantCmd command
		wantErr bool
	}{
		{"plain uninstall", []string{"uninstall"}, cmdUninstall, false},
		{"uninstall cache flag", []string{"uninstall", "--cache"}, cmdUninstall, false},
		{"uninstall bogus flag", []string{"uninstall", "--registry"}, cmdUninstall, true},
		{"uninstall extra args", []string{"uninstall", "--cache", "extra"}, cmdUninstall, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd, flags, err := parseUninstallArgs(tc.args)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseUninstallArgs(%v) = %v, want an error", tc.args, cmd)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseUninstallArgs(%v) unexpected error: %v", tc.args, err)
			}
			if cmd != tc.wantCmd || flags != (tc.args[len(tc.args)-1] == "--cache") {
				t.Fatalf("parseUninstallArgs(%v) = %v --cache=%v, mismatch", tc.args, cmd, flags)
			}
		})
	}
}

// Regression for review finding R3-001: --cache must reach the uninstaller
// end-to-end (parseCommand delegates before the generic >1-arguments check).
func TestUninstallCacheFlagReachesUninstallerR31(t *testing.T) {
	stdout, stderr, _, _ := testEnv(t)
	t.Setenv("STREAMBEATS_INSTALL_PREFIX", prefixBuilt)
	// Isolation (review R3-001 fix): run() builds PRODUCTION dependencies, so
	// the catalog cache this end-to-end run touches MUST live under a temp
	// XDG root instead of the developer's real cache. Assert the cleared cache
	// directly so the --cache path is observed end to end.
	cacheRoot := filepath.Join(fakeRoot, "xdg-cache")
	t.Setenv("XDG_CACHE_HOME", cacheRoot)
	cache := filepath.Join(cacheRoot, "streambeats")
	if err := os.MkdirAll(cache, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cache, "catalog.json"), []byte("[]"), 0o644); err != nil {
		t.Fatal(err)
	}

	code := run([]string{"uninstall", "--cache"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("uninstall --cache exit code = %d, want 0 (stderr: %s)", code, stderr.String())
	}
	if out := strings.ToLower(stdout.String() + stderr.String()); !strings.Contains(out, "cleared catalog cache") {
		t.Fatalf("--cache never reached the uninstaller; stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if _, err := os.Stat(cache); !os.IsNotExist(err) {
		t.Fatalf("--cache did not clear the isolated catalog cache: %v", err)
	}
}

// Pin the sibling behavior that shares the parser: other commands keep
// rejecting extra arguments with exit code 2.
func TestOtherCommandsStillRejectExtraArguments(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"refresh", "sync"}, &stdout, &stderr); code != 2 {
		t.Fatalf("refresh sync exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "expected at most one command") {
		t.Fatalf("sibling parse behavior changed: %q", stderr.String())
	}
}

// Regression for review finding R4-001 (lineage 3): a custom-prefix install
// must be removed by the plain `streambeats uninstall` — the running binary
// knows where it lives. StreambeatsInstallPrefix/e HOME are deliberately NOT
// set for the prefix resolution: only the fake executable path is known.
func TestUninstallResolvesPrefixFromRunningExecutableR401(t *testing.T) {
	stdout, stderr, deps, _ := testEnv(t)
	t.Setenv("STREAMBEATS_INSTALL_PREFIX", "") // no env override on purpose
	t.Setenv("HOME", filepath.Join(fakeRoot, "home-under-test"))
	if err := os.MkdirAll(filepath.Join(prefixBuilt, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	fakeExe := filepath.Join(prefixBuilt, "share", "streambeats", "bin", "streambeats")
	if err := os.MkdirAll(filepath.Dir(fakeExe), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fakeExe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	installTree(t, []string{
		prefixBuilt + "/share/streambeats",
		prefixBuilt + "/bin/streambeats",
		prefixBuilt + "/bin/streambeats-quick",
	}, "/foreign.txt")
	deps.execPath = func() (string, error) { return fakeExe, nil }

	// run() builds production deps, so the run-uninstall seam takes the
	// resolved prefix directly (the run() parse path is covered by R31).
	prefix, err := uninstallPrefix(deps)
	if err != nil {
		t.Fatalf("uninstallPrefix: %v", err)
	}
	if prefix != prefixBuilt {
		t.Fatalf("prefix resolved to %q, want the custom prefix %q (R4-001)", prefix, prefixBuilt)
	}
	code := runUninstall(stdout, stderr, uninstallRequest{Prefix: prefix, Deps: deps})
	if code != 0 {
		t.Fatalf("uninstall exit code = %d, want 0 (stderr: %s)", code, stderr.String())
	}
	if strings.Contains(strings.ToLower(stdout.String()), "nothing to uninstall") {
		t.Fatalf("custom-prefix install survived uninstall (R4-001):\n%s", stdout.String())
	}
	for _, want := range []string{
		filepath.Join(prefixBuilt, "share", "streambeats"),
		filepath.Join(prefixBuilt, "bin", "streambeats-quick"),
	} {
		if _, err := os.Stat(want); !os.IsNotExist(err) {
			t.Fatalf("custom-prefix path still present: %s", want)
		}
	}
	if _, err := os.Stat(filepath.Join(fakeRoot, "foreign.txt")); err != nil {
		t.Fatalf("foreign path was removed: %v", err)
	}
}

// --- Quality sweep (issue #21): truthfulness, contract and hardening rules.

// S1 (R3-002): a target that is already absent must not be reported as
// "removed"; the uninstaller never lies about what it deleted.
func TestUninstallDoesNotCountAbsentPathsAsRemovedS1(t *testing.T) {
	stdout, stderr, deps, _ := testEnv(t)
	installTree(t, []string{
		prefixBuilt + "/share/streambeats",
		prefixBuilt + "/bin/streambeats",
		prefixBuilt + "/bin/streambeats-quick",
		prefixBuilt + "/bin/streambeats-panel",
		prefixBuilt + "/share/applications/streambeats-panel.desktop",
		prefixBuilt + "/share/icons/hicolor/scalable/apps/streambeats-panel.svg",
	})
	icon := filepath.Join(prefixBuilt, "share", "icons", "hicolor", "scalable", "apps", "streambeats-panel.svg")
	if err := os.Remove(icon); err != nil {
		t.Fatal(err)
	}

	code := runUninstall(stdout, stderr, uninstallRequest{Prefix: prefixBuilt, Deps: deps})
	if code != 0 {
		t.Fatalf("uninstall exit code = %d, want 0 (stderr: %s)", code, stderr.String())
	}
	if got := stdout.String(); strings.Contains(got, "removed: "+icon) {
		t.Fatalf("absent path reported as removed (S1):\n%s", got)
	}
	if got, want := stdout.String(), "5 path(s) removed, 1 already absent"; !strings.Contains(got, want) {
		t.Fatalf("stdout = %q, want summary %q", got, want)
	}
}

// S2 (R3-003): when a removal was attempted and failed, runUninstall exits 1
// as its own doc contract states — not the old unconditional 0.
func TestUninstallExitCodeOneOnFailedRemovalS2(t *testing.T) {
	stdout, stderr, deps, _ := testEnv(t)
	installTree(t, []string{
		prefixBuilt + "/share/streambeats",
		prefixBuilt + "/bin/streambeats",
		prefixBuilt + "/bin/streambeats-quick",
		prefixBuilt + "/bin/streambeats-panel",
		prefixBuilt + "/share/applications/streambeats-panel.desktop",
		prefixBuilt + "/share/icons/hicolor/scalable/apps/streambeats-panel.svg",
	})
	bin := filepath.Join(prefixBuilt, "share", "streambeats", "bin")
	if err := os.Chmod(bin, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(bin, 0o755) })

	code := runUninstall(stdout, stderr, uninstallRequest{Prefix: prefixBuilt, Deps: deps})
	if code != 1 {
		t.Fatalf("failed-removal exit code = %d, want 1 (stdout: %s)", code, stdout.String())
	}
}

// S3 (R4-001): the summary line reports the failed count too, so a partial
// uninstall is visible in one glance without scanning stderr lines.
func TestUninstallSummaryReportsFailedCountS3(t *testing.T) {
	stdout, stderr, deps, _ := testEnv(t)
	installTree(t, []string{
		prefixBuilt + "/share/streambeats",
		prefixBuilt + "/bin/streambeats",
		prefixBuilt + "/bin/streambeats-quick",
		prefixBuilt + "/bin/streambeats-panel",
		prefixBuilt + "/share/applications/streambeats-panel.desktop",
		prefixBuilt + "/share/icons/hicolor/scalable/apps/streambeats-panel.svg",
	})
	bin := filepath.Join(prefixBuilt, "share", "streambeats", "bin")
	if err := os.Chmod(bin, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(bin, 0o755) })

	_ = runUninstall(stdout, stderr, uninstallRequest{Prefix: prefixBuilt, Deps: deps})
	if got, want := stdout.String(), "5 path(s) removed, 0 already absent, 1 failed"; !strings.Contains(got, want) {
		t.Fatalf("stdout = %q, want summary %q", got, want)
	}
	if !strings.Contains(stderr.String(), "streambeats: cannot remove") {
		t.Fatalf("stderr should still name the failing target:\n%s", stderr.String())
	}
}

// S4 (R4-002): the uninstaller names the target set it uses — manifest with
// its entry count, or the explicit known-layout fallback notice.
func TestUninstallNamesTargetSourceS4(t *testing.T) {
	t.Run("manifest present", func(t *testing.T) {
		stdout, stderr, deps, _ := testEnv(t)
		installTree(t, []string{
			prefixBuilt + "/share/streambeats",
			prefixBuilt + "/bin/streambeats",
		})
		code := runUninstall(stdout, stderr, uninstallRequest{Prefix: prefixBuilt, Deps: deps})
		if code != 0 {
			t.Fatalf("exit code = %d (stderr: %s)", code, stderr.String())
		}
		if got := stdout.String() + stderr.String(); !strings.Contains(got, "install manifest") || !strings.Contains(got, "2 entries") {
			t.Fatalf("stdout should name the manifest source with its count:\n%s", got)
		}
	})
	t.Run("no manifest", func(t *testing.T) {
		stdout, stderr, deps, _ := testEnv(t)
		installTree(t, nil)
		code := runUninstall(stdout, stderr, uninstallRequest{Prefix: prefixBuilt, Deps: deps})
		if code != 0 {
			t.Fatalf("exit code = %d (stderr: %s)", code, stderr.String())
		}
		if got := stdout.String() + stderr.String(); !strings.Contains(got, "known install layout") {
			t.Fatalf("stdout should announce the known-layout fallback:\n%s", got)
		}
	})
	t.Run("manifest unreadable falls back", func(t *testing.T) {
		stdout, stderr, deps, _ := testEnv(t)
		installTree(t, nil)
		manifest := filepath.Join(prefixBuilt, "share", "streambeats", "manifest.txt")
		_ = os.MkdirAll(filepath.Dir(manifest), 0o755)
		_ = os.WriteFile(manifest, []byte{0}, 0o644) // binary-garbled manifest
		code := runUninstall(stdout, stderr, uninstallRequest{Prefix: prefixBuilt, Deps: deps})
		_ = code
		if got := stdout.String() + stderr.String(); !strings.Contains(got, "known install layout") {
			t.Fatalf("garbled manifest should announce the known-layout fallback:\n%s", got)
		}
	})
}

// S5 (R3-001): the two manifest rejections (relative line vs outside-prefix
// line) print DIFFERENT warning messages, each naming its actual reason.
func TestUninstallManifestRejectionMessagesDistinguishReasonS5(t *testing.T) {
	t.Run("relative line", func(t *testing.T) {
		stdout, stderr, deps, _ := testEnv(t)
		share := filepath.Join(prefixBuilt, "share", "streambeats")
		installTree(t, []string{prefixBuilt + "/share/streambeats"})
		if err := os.WriteFile(filepath.Join(share, "manifest.txt"), []byte("relative/steal\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		_ = runUninstall(stdout, stderr, uninstallRequest{Prefix: prefixBuilt, Deps: deps})
		if got := stderr.String(); !strings.Contains(got, "not an absolute path") {
			t.Fatalf("relative rejection should say so:\n%s", got)
		}
	})
	t.Run("outside-prefix line", func(t *testing.T) {
		stdout, stderr, deps, _ := testEnv(t)
		share := filepath.Join(prefixBuilt, "share", "streambeats")
		installTree(t, []string{prefixBuilt + "/share/streambeats"})
		if err := os.WriteFile(filepath.Join(share, "manifest.txt"), []byte(prefixBuilt+"/../host-owned\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		_ = runUninstall(stdout, stderr, uninstallRequest{Prefix: prefixBuilt, Deps: deps})
		if got := stderr.String(); !strings.Contains(got, "outside the prefix") {
			t.Fatalf("outside-prefix rejection should say so:\n%s", got)
		}
	})
}

// S6 (R1-manifest-path-scope hardening): a manifest path that resolves
// through a symlink outside the prefix is rejected with a warning and never
// touched; the rest of the manifest still runs.
func TestUninstallRejectsSymlinkEscapeOutsidePrefixS6(t *testing.T) {
	stdout, stderr, deps, _ := testEnv(t)
	outside := filepath.Join(fakeRoot, "elsewhere")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	secret := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(secret, []byte("KEEP"), 0o644); err != nil {
		t.Fatal(err)
	}
	installTree(t, []string{
		prefixBuilt + "/share/streambeats",
		prefixBuilt + "/bin/streambeats",
	})
	link := filepath.Join(prefixBuilt, "bin", "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(prefixBuilt, "share", "streambeats", "manifest.txt"),
		[]byte("# streambeats install manifest\n"+prefixBuilt+"/bin/escape\n"+prefixBuilt+"/bin/streambeats\n"), 0o644)

	code := runUninstall(stdout, stderr, uninstallRequest{Prefix: prefixBuilt, Deps: deps})
	if code != 0 {
		t.Fatalf("symlink rejection is a warning, not an error: exit %d (stderr: %s)", code, stderr.String())
	}
	if _, err := os.Stat(link); err != nil {
		t.Fatalf("in-prefix symlink itself must survive a scope rejection: %v", err)
	}
	if _, err := os.Stat(secret); err != nil {
		t.Fatalf("manifest symlink executed outside the prefix: %v", err)
	}
	if _, err := os.Stat(filepath.Join(prefixBuilt, "bin", "streambeats")); !os.IsNotExist(err) {
		t.Fatalf("the rest of the manifest should still remove its targets: %v", err)
	}
	if got := stderr.String(); !strings.Contains(got, "outside the prefix") {
		t.Fatalf("stderr should warn about the symlink escape:\n%s", got)
	}
}

// R3-001 correction (native review): an Lstat error that is NOT an absence
// (e.g. permission denial, EACCES) must not be treated as "already absent".
// The removal attempt must run and its failure must surface: exit 1, the
// target reported in stderr, and counted as removed-attempted, absent-failed.
func TestUninstallLstatPermissionErrorSurfacesAsFailureR301(t *testing.T) {
	stdout, stderr, deps, _ := testEnv(t)
	installTree(t, []string{
		prefixBuilt + "/share/streambeats",
		prefixBuilt + "/bin/locked/target",
		prefixBuilt + "/bin/streambeats",
	})
	locked := filepath.Join(prefixBuilt, "bin", "locked")
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })

	code := runUninstall(stdout, stderr, uninstallRequest{Prefix: prefixBuilt, Deps: deps})
	if code != 1 {
		t.Fatalf("uninspectable-target exit code = %d, want 1 (stdout: %s) (stderr: %s)", code, stdout.String(), stderr.String())
	}
	if got := stdout.String(); strings.Contains(got, "2 path(s) removed, 1 already absent") || !strings.Contains(got, ", 1 failed") {
		t.Fatalf("uninspectable target must be a FAILED removal, not an absent one:\n%s", got)
	}
	if !strings.Contains(stderr.String(), "cannot remove") {
		t.Fatalf("stderr must name the removal failure:\n%s", stderr.String())
	}
}

// R4-001 correction (native review, lineage 2): with a SYMLINKED install
// prefix, targets resolve to the prefix's real path while the old containment
// check compared against the unresolved prefix — every valid target was
// dropped as "outside" and the uninstall reported success without removing
// anything. The containment must compare against the resolved prefix.
func TestUninstallSymlinkedPrefixKeepsItsTargetsR401(t *testing.T) {
	stdout, stderr, deps, _ := testEnv(t)
	realPrefix := filepath.Join(fakeRoot, "local-real")
	linkPrefix := filepath.Join(fakeRoot, "local-link")
	if err := os.MkdirAll(realPrefix, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(realPrefix, linkPrefix); err != nil {
		t.Fatal(err)
	}
	prefixBuilt = linkPrefix // install through the symlinked prefix
	installTree(t, []string{
		linkPrefix + "/share/streambeats",
		linkPrefix + "/bin/streambeats",
	})

	code := runUninstall(stdout, stderr, uninstallRequest{Prefix: linkPrefix, Deps: deps})
	if code != 0 {
		t.Fatalf("symlinked-prefix uninstall exit code = %d, want 0 (stderr: %s)", code, stderr.String())
	}
	if strings.Contains(strings.ToLower(stdout.String()), "nothing to uninstall") {
		t.Fatalf("symlinked-prefix install survived uninstall (R4-001):\n%s", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(realPrefix, "share", "streambeats")); !os.IsNotExist(err) {
		t.Fatalf("manifest target under the real prefix was not removed: %v", err)
	}
}

// --- Second-sweep follow-ups (issue #27, from the APPROVED receipt).

// R3-001 follow-up: when the install prefix itself cannot be resolved, the
// containment falls back to the lexical check — the fallback must announce
// itself instead of silently re-enabling the weakness R4-001 removed.
func TestUninstallPrefixResolutionFallbackAnnouncesItselfR301b(t *testing.T) {
	stdout, stderr, deps, _ := testEnv(t)
	ghostPrefix := filepath.Join(fakeRoot, "gone") // never created: EvalSymlinks fails

	code := runUninstall(stdout, stderr, uninstallRequest{Prefix: ghostPrefix, Deps: deps})
	if code != 0 {
		t.Fatalf("ghost-prefix exit code = %d, want 0 (stderr: %s)", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "containment check") {
		t.Fatalf("prefix-resolution fallback must warn it downgraded to lexical containment:\n%s", stderr.String())
	}
}

// R4-001 follow-up: a manifest that EXISTS but yields zero usable targets
// must announce its fallback to the known layout, not silently imply the
// manifest was used.
func TestUninstallUnusableManifestAnnouncesFallbackR401b(t *testing.T) {
	stdout, stderr, deps, _ := testEnv(t)
	share := filepath.Join(prefixBuilt, "share", "streambeats")
	installTree(t, []string{prefixBuilt + "/share/streambeats"})
	if err := os.WriteFile(filepath.Join(share, "manifest.txt"),
		[]byte("# streambeats install manifest\nrelative/line\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	code := runUninstall(stdout, stderr, uninstallRequest{Prefix: prefixBuilt, Deps: deps})
	if code != 0 {
		t.Fatalf("exit code = %d (stderr: %s)", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "manifest present but yielded no usable targets") {
		t.Fatalf("unusable manifest fallback must announce itself:\n%s", stderr.String())
	}
	if !strings.Contains(stdout.String(), "known install layout") {
		t.Fatalf("fallback notice for the known layout missing:\n%s", stdout.String())
	}
}
