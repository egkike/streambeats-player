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
