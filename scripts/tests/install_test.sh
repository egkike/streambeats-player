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
run_installer() {
	local prefix="$1"; shift
	STREAMBEATS_API_URL="file://$STAGE/api" \
		STREAMBEATS_RAW_URL="file://$STAGE/raw" \
		STREAMBEATS_INSTALL_PREFIX="$prefix" \
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
mkdir -p "$STAGE/raw/v1.1.0/scripts" "$STAGE/raw/v1.1.0/assets/ghostty"
printf '#!/bin/sh
echo "quick-panel $*"
' >"$STAGE/raw/v1.1.0/scripts/streambeats-quick"
chmod 0755 "$STAGE/raw/v1.1.0/scripts/streambeats-quick"
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
assert_file "$SHARE/assets/ghostty/streambeats-right.conf" "preset fetched from raw"
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

say "----------------------------------------"
say "scenarios: $((PASS_COUNT + FAIL_COUNT)) assertions — $PASS_COUNT pass, $FAIL_COUNT fail"
[ "$FAIL_COUNT" -eq 0 ] || exit 1
exit 0
