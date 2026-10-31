#!/bin/bash
# install_test.sh — self-contained harness for scripts/install.sh.
#
# Builds a fake GitHub release on disk (staged JSON + file:// artifact URLs),
# runs the real installer against it with isolated prefixes, and asserts
# observable behavior: bundle layout, wrapper execution, checksum refusal,
# and the raw-content fallback for archives that predate bundling of the
# launcher files. No network access is used; all scratch lives in a mktemp
# directory outside the repository and is removed on exit.
#
# Run:  scripts/tests/install_test.sh

set -u

SCRIPT_UNDER_TEST="$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)/install.sh"

PASS_COUNT=0
FAIL_COUNT=0

say() { printf 'install_test: %s\n' "$*"; }
fail() {
	printf 'install_test: FAIL: %s\n' "$*" >&2
	FAIL_COUNT=$((FAIL_COUNT + 1))
}
pass() {
	printf 'install_test: PASS: %s\n' "$*"
	PASS_COUNT=$((PASS_COUNT + 1))
}

assert_file() {
	if [ -f "$1" ]; then pass "file exists: $2"; else fail "missing file: $1 ($2)"; fi
}

assert_not_file() {
	if [ ! -e "$1" ]; then pass "absent: $2"; else fail "unexpectedly present: $1 ($2)"; fi
}

assert_exec() {
	if [ -x "$1" ]; then pass "executable: $2"; else fail "not executable: $1 ($2)"; fi
}

# stage_fake_release <dir> [include_launcher_assets=1]
# Builds a dist tree shaped like the real archive layout, tars it, writes
# checksums.txt and a release JSON whose browser_download_url entries point
# at file:// paths. Os/arch of the host is used so the installer's own
# platform mapping matches what is staged.
stage_fake_release() {
	local dir="$1" with_assets="${2:-1}"
	local os arch dist disttar sha
	os=$(uname -s | tr '[:upper:]' '[:lower:]')
	arch=$(uname -m | sed 's/^x86_64$/amd64/; s/^aarch64$/arm64/')
	dist="$dir/dist"
	disttar="$dir/streambeats-player_1.1.0_${os}_${arch}.tar.gz"
	mkdir -p "$dist/scripts" "$dist/assets/ghostty" "$dist/bin"

	cat >"$dist/bin/streambeats" <<'EOF'
#!/bin/sh
# fake player: help exits 0 loudly; anything else prints a one-line identity.
[ "$1" = "help" ] && { echo "usage: streambeats [tui|refresh|sync|help]"; exit 0; }
echo "streambeats-fake"
EOF
	chmod 0755 "$dist/bin/streambeats"

	if [ "$with_assets" = "1" ]; then
		printf '#!/bin/sh\necho "quick-panel $*"\n' >"$dist/scripts/streambeats-quick"
		chmod 0755 "$dist/scripts/streambeats-quick"
		printf '#!/bin/sh\ntrue\n' >"$dist/scripts/panel-terminal"
		for side in top bottom left right; do
			printf 'window-mode = %s\n# streambeats fake preset\n' "$side" \
				>"$dist/assets/ghostty/streambeats-$side.conf"
		done
	fi
	printf '# streambeats README (staged)\n' >"$dist/README.md"
	printf 'MIT (staged)\n' >"$dist/LICENSE"

	# The real archive keeps the binary at the ARCHIVE ROOT (not bin/) — the
	# installer is the component that moves it under bin/ of the shared tree.
	cp "$dist/bin/streambeats" "$dist/streambeats"
	# The tar list mirrors goreleaser output: paths preserved, no bin/ dir.
	tar -C "$dist" --owner=0 --group=0 \
		-czf "$disttar" \
		streambeats scripts assets README.md LICENSE \
		2>/dev/null || tar -C "$dist" -czf "$disttar" streambeats README.md LICENSE

	sha=$(sha256sum "$disttar" | awk '{ print $1 }')
	printf '%s  %s\n' "$sha" "$(basename "$disttar")" >"$dir/checksums.txt"

	cat >"$dir/release.json" <<EOF
{
  "release": { "tag_name": "v1.1.0", "name": "staged" },
  "assets": [
    { "name": "$(basename "$disttar")",
      "browser_download_url": "file://$disttar" },
    { "name": "checksums.txt",
      "browser_download_url": "file://$dir/checksums.txt" }
  ]
}
EOF
}

# run_installer <prefix> [args...] — all overrides point at the staged tree.
# XDG_CONFIG_HOME is staged too: the installer creates the config dir (Phase
# A, issue #35); no scenario may ever touch the real user config.
run_installer() {
	local prefix="$1"; shift
	STREAMBEATS_API_URL="file://$STAGE/api" \
		STREAMBEATS_RAW_URL="file://$STAGE/raw" \
		STREAMBEATS_INSTALL_PREFIX="$prefix" \
		XDG_CONFIG_HOME="$Fx/user-config" \
		"$SCRIPT_UNDER_TEST" --version v1.1.0 "$@"
}

Fx=$(mktemp -d "/tmp/streambeats-test.XXXXXX")
if [ "${STREAMBEATS_TEST_KEEP:-0}" = 1 ]; then trap '' EXIT; fi

# ---------------------------------------------------------------------------
# Scenario 1: happy path against the full archive
# ---------------------------------------------------------------------------
STAGE="$Fx/stage1"
PREFIX="$Fx/prefix1"
mkdir -p "$STAGE/api/releases/tags"
stage_fake_release "$STAGE/artifacts" 1
cp "$STAGE/artifacts/release.json" "$STAGE/api/releases/tags/v1.1.0"
say "scenario 1: full bundle install (stage $STAGE)"
OUT=$(run_installer "$PREFIX" 2>"$Fx/err1")
rc1=$?
if [ "$rc1" = "0" ]; then
	pass "installer exited 0"
else
	fail "installer exited $rc1; stderr: $(cat "$Fx/err1")"
fi
SHARE="$PREFIX/share/streambeats"
assert_file "$SHARE/bin/streambeats" "shared player binary under bin/ (launcher resolves scripts/..)"
assert_not_file "$SHARE/streambeats" "no stray archive-root binary"
assert_file "$SHARE/scripts/streambeats-quick" "quick-terminal launcher installed"
assert_file "$SHARE/scripts/panel-terminal" "shared panel-terminal helper installed (issue #122)"
for side in top bottom left right; do
	assert_file "$SHARE/assets/ghostty/streambeats-$side.conf" "preset $side"
done
assert_file "$SHARE/README.md" "README carried into the shared tree"
assert_file "$PREFIX/bin/streambeats" "wrapper streambeats"
assert_file "$PREFIX/bin/streambeats-quick" "wrapper streambeats-quick"
assert_exec "$PREFIX/bin/streambeats" "wrapper is executable"
assert_exec "$SHARE/scripts/streambeats-quick" "launcher is executable"

FAKE_HELP=$("$PREFIX/bin/streambeats" help 2>&1)
if printf '%s' "$FAKE_HELP" | grep -q "usage: streambeats"; then
	pass "wrapper runs the binary: help"
else
	fail "wrapper did not run the binary (got: $FAKE_HELP)"
fi
QUICK=$("$PREFIX/bin/streambeats-quick" right 2>&1)
if printf '%s' "$QUICK" | grep -q "quick-panel"; then
	pass "quick wrapper runs the launcher: right"
else
	fail "quick wrapper did not run the launcher (got: $QUICK)"
fi
if printf '%s\n' "$OUT" | grep -q "streambeats-quick right"; then
	pass "summary advertises the try-it command"
else
	fail "summary missing try-it command"
fi
if printf '%s\n' "$OUT" | grep -q "streambeats uninstall"; then
	pass "summary advertises the uninstall CLI"
else
	fail "summary missing uninstall CLI advertisement"
fi

assert_file "$SHARE/manifest.txt" "install manifest for the uninstall CLI"
for p in "$SHARE" "$PREFIX/bin/streambeats" "$PREFIX/bin/streambeats-quick" "$PREFIX/bin/streambeats-panel" "$PREFIX/share/applications/streambeats-panel.desktop" "$PREFIX/share/icons/hicolor/scalable/apps/streambeats-panel.svg"; do
	if grep -qxF "$p" "$SHARE/manifest.txt"; then
		pass "manifest lists $p"
	else
		fail "manifest does not list $p"
	fi
done
if grep -qE 'Music|XDG_CACHE|catalog' "$SHARE/manifest.txt"; then
	fail "manifest must not list user data (Music/cache)"
else
	pass "manifest excludes user data"
fi

# ---------------------------------------------------------------------------
# Scenario 2: checksum mismatch refuses BEFORE installing anything
# ---------------------------------------------------------------------------
STAGE="$Fx/stage2"
PREFIX="$Fx/prefix2"
mkdir -p "$STAGE/api/releases/tags"
stage_fake_release "$STAGE/artifacts" 1
# Corrupt the served tarball AFTER checksums.txt was written.
printf '\0corrupt' >>"$STAGE/artifacts/streambeats-player_1.1.0_$(uname -s | tr '[:upper:]' '[:lower:]')_$(uname -m | sed 's/^x86_64$/amd64/').tar.gz"
cp "$STAGE/artifacts/release.json" "$STAGE/api/releases/tags/v1.1.0"
say "scenario 2: checksum mismatch refusal"
run_installer "$PREFIX" 2>"$Fx/err2" >/dev/null
rc2=$?
if [ "$rc2" != "0" ]; then
	pass "refused with non-zero exit ($rc2)"
else
	fail "installer exited 0 on a corrupted tarball"
fi
if grep -q "checksum mismatch" "$Fx/err2"; then
	pass "stderr names the checksum mismatch"
else
	fail "no checksum message in stderr"
fi
assert_not_file "$PREFIX/share/streambeats/bin/streambeats" "no binary installed on checksum failure"
assert_not_file "$PREFIX/bin/streambeats-quick" "no wrapper installed on checksum failure"

# ---------------------------------------------------------------------------
# Scenario 3: archive without bundled launcher assets falls back to raw
# ---------------------------------------------------------------------------
STAGE="$Fx/stage3"
PREFIX="$Fx/prefix3"
mkdir -p "$STAGE/api/releases/tags"
stage_fake_release "$STAGE/artifacts" 0
cp "$STAGE/artifacts/release.json" "$STAGE/api/releases/tags/v1.1.0"
# Raw content at the same tag: launcher + presets that v1.0.0-era archives
# did not carry. Generated fresh here (the asset-less dist tree never
# created them).
mkdir -p "$STAGE/raw/v1.1.0/scripts" "$STAGE/raw/v1.1.0/assets/ghostty" "$STAGE/raw/v1.1.0/assets/kitty"
printf '#!/bin/sh\ntrue\n' >"$STAGE/raw/v1.1.0/scripts/panel-terminal"
printf '#!/bin/sh
echo "quick-panel $*"
' >"$STAGE/raw/v1.1.0/scripts/streambeats-quick"
chmod 0755 "$STAGE/raw/v1.1.0/scripts/streambeats-quick"
# Issue #90: the raw fallback ships BOTH panel families now.
for side in top bottom left right; do
	printf 'edge = %s\n# raw-staged kitty preset\n' "$side" \
		>"$STAGE/raw/v1.1.0/assets/kitty/streambeats-$side.conf"
done
for side in top bottom left right; do
	printf 'window-mode = %s\n# raw-staged preset\n' "$side" \
		>"$STAGE/raw/v1.1.0/assets/ghostty/streambeats-$side.conf"
done
say "scenario 3: raw fallback for launcher assets"
OUT=$(run_installer "$PREFIX" 2>"$Fx/err3")
rc3=$?
if [ "$rc3" = "0" ]; then
	pass "installer exited 0 with raw fallback"
else
	fail "installer exited $rc3; stderr: $(cat "$Fx/err3")"
fi
SHARE="$PREFIX/share/streambeats"
assert_file "$SHARE/scripts/streambeats-quick" "launcher fetched from raw"
assert_file "$SHARE/scripts/panel-terminal" "helper fetched from raw (issue #122)"
assert_file "$SHARE/assets/ghostty/streambeats-right.conf" "preset fetched from raw"
assert_file "$SHARE/assets/kitty/streambeats-right.conf" "kitty preset fetched from raw (issue #90)"
QUICK=$("$PREFIX/bin/streambeats-quick" left 2>&1)
if printf '%s' "$QUICK" | grep -q "quick-panel"; then
	pass "quick wrapper runs the launcher after fallback"
else
	fail "quick wrapper broken after fallback (got: $QUICK)"
fi

# ---------------------------------------------------------------------------
# Scenario 4: hostile prefixes cannot inject commands into wrappers
# (regression for the reviewer-found R1-001; uses the STUB launcher, so no
# real process except sh is spawned)
# ---------------------------------------------------------------------------
STAGE="$Fx/stage_hostile"
PREFIX="$Fx/night\$(printf INJECTED)'-wire"
mkdir -p "$STAGE/api/releases/tags"
stage_fake_release "$STAGE/artifacts" 1
cp "$STAGE/artifacts/release.json" "$STAGE/api/releases/tags/v1.1.0"
say "scenario 4: hostile prefix injection refusal"
run_installer "$PREFIX" 2>"$Fx/err_hostile"
rcH=$?
if [ "$rcH" = "0" ]; then
	pass "installer tolerated a hostile prefix literally"
else
	fail "installer exited $rcH on a hostile prefix; stderr: $(cat "$Fx/err_hostile")"
fi
WRC="$PREFIX/bin/streambeats-quick"
assert_file "$WRC" "wrapper exists under the literal hostile prefix"
if grep -qE '\"\$\(' "$WRC"; then
	fail "wrapper embeds the prefix in double quotes: $WRC"
else
	pass "wrapper keeps the prefix in single quotes (no $ or backtick live)"
fi
WOUT=$("$WRC" left 2>&1)
rcW=$?
if [ "$rcW" = "0" ] && printf '%s' "$WOUT" | grep -q "quick-panel left"; then
	pass "wrapper runs through the hostile path (/bin/sh executed it as data)"
	if printf '%s' "$WOUT" | grep -q "INJECTED"; then
		fail "command substitution leaked into execution"
	fi
else
	fail "wrapper did not run on the hostile prefix (rc=$rcW, got: $WOUT)"
fi

# ---------------------------------------------------------------------------
# Scenario 6: PIPE MODE — bash receives the script on stdin (the README
# one-liner). Regression for the silent main-guard bug the owner found:
# BASH_SOURCE[0] is unset under `bash -s < script`, so the guard MUST use
# the $0 fallback. Asserts real installation + visible progress output.
# ---------------------------------------------------------------------------
STAGE="$Fx/pipe_mode"
PREFIX="$Fx/pipe_prefix"
mkdir -p "$STAGE/api/releases/tags"
stage_fake_release "$STAGE/artifacts" 1
cp "$STAGE/artifacts/release.json" "$STAGE/api/releases/tags/v1.1.0"
say "scenario 6: pipe mode (bash -s < script) reaches main"
STREAMBEATS_API_URL="file://$STAGE/api" \
	STREAMBEATS_RAW_URL="file://$STAGE/rawpipe" \
	STREAMBEATS_INSTALL_PREFIX="$PREFIX" \
	bash -s -- --version v1.1.0 <"$SCRIPT_UNDER_TEST" >"$Fx/pipe_out" 2>"$Fx/pipe_err"
rcP=$?
if [ "$rcP" = "0" ]; then
	pass "pipe mode exits 0"
else
	fail "pipe mode exited $rcP; stderr: $(cat "$Fx/pipe_err")"
fi
if grep -q "resolving release version" "$Fx/pipe_out"; then
	pass "installer emitted progress output in pipe mode"
else
	fail "NO output in pipe mode — main() never ran (the silent guard bug)"
fi
ART_DIR="$PREFIX/share/streambeats"
if [ -f "$ART_DIR/bin/streambeats" ]; then
	pass "pipe mode installed the bundle"
else
	fail "pipe mode installed NOTHING (artifacts: $(ls "$STAGE"))"
fi
if [ -f "$PREFIX/bin/streambeats-quick" ]; then
	pass "pipe mode installed the launcher wrapper"
else
	fail "pipe mode missing $PREFIX/bin/streambeats-quick"
fi

# ---------------------------------------------------------------------------
# Scenario 7: argument hygiene
# ---------------------------------------------------------------------------
STAGE="$Fx/stage4"
mkdir -p "$STAGE"
say "scenario 7: argument hygiene"
if "$SCRIPT_UNDER_TEST" --bogus >/dev/null 2>&1; then
	fail "--bogus accepted"
else
	pass "unknown argument rejected"
fi
if "$SCRIPT_UNDER_TEST" --version >/dev/null 2>&1; then
	fail "--version without value accepted"
else
	pass "--version without value rejected"
fi



# ---------------------------------------------------------------------------
# Scenario 8: DARWIN BRANCH — the freedesktop story (.desktop, hicolor icon,
# KWin rules) is SKIPPED on macOS, the manifest still records the canonical
# targets (uninstall reports them as already absent), and the completion echo
# carries the per-OS hints (issue #10). STREAMBEATS_FAKE_OS is the documented
# test-only override letting this Linux harness exercise the Darwin branch;
# the macOS CI job runs the real branch natively.
# ---------------------------------------------------------------------------
STAGE="$Fx/darwin"
PREFIX="$Fx/darwin_prefix"
mkdir -p "$STAGE/api/releases/tags"
stage_fake_release "$STAGE/artifacts" 1
cp "$STAGE/artifacts/release.json" "$STAGE/api/releases/tags/v1.1.0"
say "scenario 8: darwin branch (freedesktop skipped, per-OS hints)"
OUTD=$(STREAMBEATS_FAKE_OS=darwin run_installer "$PREFIX" 2>"$Fx/err8")
rcD=$?
if [ "$rcD" = "0" ]; then
	pass "darwin-mode installer exits 0"
else
	fail "darwin-mode installer exited $rcD; stderr: $(cat "$Fx/err8")"
fi
SHARED="$PREFIX/share/streambeats"
for f in "$SHARED/bin/streambeats" "$SHARED/scripts/streambeats-quick" \
	"$PREFIX/bin/streambeats" "$PREFIX/bin/streambeats-quick" \
	"$SHARED/assets/ghostty/streambeats-right.conf" "$SHARED/manifest.txt"; do
	assert_file "$f" "darwin bundle keeps the generic install layout"
done
for f in "$PREFIX/share/applications/streambeats-panel.desktop" \
	"$PREFIX/share/icons/hicolor/scalable/apps/streambeats-panel.svg"; do
	assert_not_file "$f" "darwin mode never generates freedesktop entries"
done
if [ -d "$SHARED/assets/kwin" ]; then
	fail "kwin rules dir created on darwin"
else
	pass "kwin rules dir skipped on darwin"
fi
if printf '%s\n' "$OUTD" | grep -q "macOS"; then
	pass "completion echo carries the macOS floating-path hint"
else
	fail "completion echo missing the macOS hint"
fi
if printf '%s\n' "$OUTD" | grep -q "zprofile"; then
	pass "PATH hint points macOS users at ~/.zprofile"
else
	fail "PATH hint missing the zsh guidance"
fi
for p in "$SHARED" "$PREFIX/bin/streambeats" "$PREFIX/bin/streambeats-quick" \
	"$PREFIX/bin/streambeats-panel" \
	"$PREFIX/share/applications/streambeats-panel.desktop" \
	"$PREFIX/share/icons/hicolor/scalable/apps/streambeats-panel.svg"; do
	if grep -qxF "$p" "$SHARED/manifest.txt"; then
		pass "darwin manifest lists $p"
	else
		fail "darwin manifest does not list $p"
	fi
done

# Scenario 9: OPTIONAL-DESKTOP-DIRS DEGRADE — the core install must never
# die because the freedesktop directories cannot be created (issue #21/S7:
# "desktop integration is optional by design"). Pre-create
# $PREFIX/share/icons as a FILE so mkdir -p .../icons/hicolor/... fails with
# ENOTDIR: the installer warns and skips the integration instead of dying.
STAGE="$Fx/stage9"
PREFIX="$Fx/prefix9"
mkdir -p "$STAGE/api/releases/tags"
stage_fake_release "$STAGE/artifacts" 1
cp "$STAGE/artifacts/release.json" "$STAGE/api/releases/tags/v1.1.0"
mkdir -p "$PREFIX/share"
: >"$PREFIX/share/icons"
say "scenario 9: optional desktop dirs degrade (icons path blocked by a file)"
OUT9=$(run_installer "$PREFIX" 2>"$Fx/err9")
rc9=$?
if [ "$rc9" = "0" ]; then
	pass "installer exited 0 despite blocked desktop dirs"
else
	fail "installer died for an OPTIONAL story (rc=$rc9); stderr: $(cat "$Fx/err9")"
fi
if grep -q "desktop integration skipped" "$OUT9" "$Fx/err9" 2>/dev/null; then
	pass "skip is announced (warn, not silent)"
else
	fail "no 'desktop integration skipped' notice in output"
fi
# R2-001: the degrade warning must name BOTH dirs the guarded mkdir can fail
# on (the message may not suggest only the icons dir was the cause).
if grep -q "assets/kwin" "$OUT9" "$Fx/err9" 2>/dev/null; then
	pass "degrade warning names both guarded directories"
else
	fail "degrade warning hides the kwin dir from the skip notice"
fi
SHARE9="$PREFIX/share/streambeats"
assert_file "$SHARE9/bin/streambeats" "core binary installed despite skipped integration"
assert_file "$SHARE9/manifest.txt" "manifest written for the uninstall CLI"
if grep -qxF "$SHARE9" "$SHARE9/manifest.txt"; then
	pass "manifest still lists the share root"
else
	fail "manifest lost the share root entry"
fi

# ---------------------------------------------------------------------------
# Scenario 10: CONFIG DIR SHIPS (Phase A issue #35 S3) — the installer
# creates the streambeats config dir and advertises the settings file as
# user data; the manifest NEVER lists it (uninstall never deletes user
# config; preservation itself is pinned in cmd/streambeats/uninstall_test.go).
# ---------------------------------------------------------------------------
STAGE="$Fx/stage10"
PREFIX="$Fx/prefix10"
mkdir -p "$STAGE/api/releases/tags"
stage_fake_release "$STAGE/artifacts" 1
cp "$STAGE/artifacts/release.json" "$STAGE/api/releases/tags/v1.1.0"
say "scenario 10: config dir ships with the install (Phase A)"
OUT10="$Fx/out10"
ERR10="$Fx/err10"
run_installer "$PREFIX" >"$OUT10" 2>"$ERR10" || {
	fail "scenario 10 installer exited non-zero"
	say "stderr: $(cat "$ERR10")"
	exit 1
}
CFG10="$Fx/user-config/streambeats"
if [ -d "$CFG10" ]; then
	pass "config dir created at XDG_CONFIG_HOME/streambeats"
else
	fail "config dir not created ($CFG10 missing)"
fi
if grep -q "settings.json" "$OUT10"; then
	pass "summary advertises the settings file path"
else
	fail "summary missing the settings.json advertisement"
fi
if grep -q "user data" "$OUT10" $Fx/err10 2>/dev/null; then
	pass "settings advertised as user data (uninstall keeps it)"
else
	fail "settings not advertised as user data"
fi
# The manifest must NOT list the config dir: uninstall deletes manifest
# paths only, and user config must never ride on that list.
SHARE10="$PREFIX/share/streambeats"
if grep -q "$CFG10" "$SHARE10/manifest.txt" 2>/dev/null; then
	fail "manifest lists the config dir (uninstall would delete user data)"
else
	pass "manifest does not list the config dir"
fi

# ---------------------------------------------------------------------------
# Scenario 11: kdotool advisory (issue #70) — panel toggle dependency.
# Linux + ghostty present + kdotool missing → the installer names both
# install paths (distro package; upstream prebuilt). kdotool present, or the
# darwin branch (no freedesktop integration), stay silent (S3/S5).
say "scenario 11: kdotool advisory (panel toggle dependency)"
FAKEBIN="$Fx/fakebin"
mkdir -p "$FAKEBIN"
printf '#!/bin/sh\nexit 0\n' >"$FAKEBIN/ghostty"
chmod 0755 "$FAKEBIN/ghostty"

STAGE="$Fx/stage11"
mkdir -p "$STAGE/api/releases/tags"
stage_fake_release "$STAGE/artifacts" 1
cp "$STAGE/artifacts/release.json" "$STAGE/api/releases/tags/v1.1.0"

# 11a: linux + ghostty + kdotool missing -> advisory. Hermeticity: the
# owner's real kdotool (~/.local/bin) sits in PATH, and `command -v` skips
# non-executable shadows — so build KDFREE_PATH by dropping every PATH
# directory that holds an executable kdotool, then lead with FAKEBIN.
KDFREE_PATH=""
for d in $(printf '%s' "$PATH" | tr ':' ' '); do
	[ -x "$d/kdotool" ] || KDFREE_PATH="$KDFREE_PATH:$d"
done
KDFREE_PATH="${KDFREE_PATH#:}"
PREFIXA="$Fx/prefix11a"
rc11a=0
if OUT11A=$(PATH="$FAKEBIN:$KDFREE_PATH" run_installer "$PREFIXA" 2>&1); then
	pass "advisory scenario exits 0 (kdotool missing)"
else
	rc11a=$?
	fail "advisory scenario exited $rc11a"
fi
if printf '%s' "$OUT11A" | grep -q "kdotool"; then
	pass "advisory names kdotool when missing (linux + ghostty)"
else
	fail "no kdotool advisory on linux + ghostty (out: $OUT11A)"
fi
if printf '%s' "$OUT11A" | grep -q "jinliu/kdotool"; then
	pass "advisory names the upstream prebuilt path"
else
	fail "advisory omits the upstream prebuilt path"
fi

# 11b: kdotool present -> silent (executable FAKEBIN shadow decides)
printf '#!/bin/sh\nexit 0\n' >"$FAKEBIN/kdotool"
chmod 0755 "$FAKEBIN/kdotool"
PREFIXB="$Fx/prefix11b"
rc11b=0
if OUT11B=$(PATH="$FAKEBIN:$KDFREE_PATH" run_installer "$PREFIXB" 2>&1); then
	pass "kdotool-present scenario exits 0"
else
	rc11b=$?
	fail "kdotool-present scenario exited $rc11b"
fi
if printf '%s' "$OUT11B" | grep -q "kdotool"; then
	fail "advisory fired despite kdotool being present"
else
	pass "no kdotool advisory when kdotool is present"
fi

# 11c: darwin branch -> silent even with kdotool missing
rm -f "$FAKEBIN/kdotool"
PREFIXC="$Fx/prefix11c"
rc11c=0
if OUT11C=$(PATH="$FAKEBIN:$PATH" STREAMBEATS_FAKE_OS=darwin \
		run_installer "$PREFIXC" 2>&1); then
	pass "darwin advisory scenario exits 0"
else
	rc11c=$?
	fail "darwin advisory scenario exited $rc11c"
fi
if printf '%s' "$OUT11C" | grep -q "kdotool"; then
	fail "darwin branch fired the kdotool advisory (S5 violated)"
else
	pass "darwin branch stays silent about kdotool"
fi

say "----------------------------------------"
say "scenarios: $((PASS_COUNT + FAIL_COUNT)) assertions — $PASS_COUNT pass, $FAIL_COUNT fail"
[ "$FAIL_COUNT" -eq 0 ] || exit 1
exit 0
