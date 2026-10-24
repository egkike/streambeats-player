#!/bin/bash
# desktop_integration_test.sh — stub-based harness for the KDE desktop
# integration (scripts/streambeats-panel + installer-generated .desktop).
#
# No systemd unit and no real launcher is ever touched: `systemctl`/
# `systemd-run` are stubbed in a leading PATH dir; the scenarios that would
# reach the launcher run inside a FAKE installed tree whose launcher is a
# stub that only records its arguments. desktop-file-validate runs only when
# available (SKIP is never counted as pass).
#
# Run:  scripts/tests/desktop_integration_test.sh

set -u

REPO_ROOT="$(CDPATH='' cd -- "$(dirname -- "$0")/../.." && pwd -P)"
PANEL_UNDER_TEST="$REPO_ROOT/scripts/streambeats-panel"

PASS_COUNT=0
FAIL_COUNT=0

say() { printf 'desktop_integration_test: %s\n' "$*"; }
fail() {
	printf 'desktop_integration_test: FAIL: %s\n' "$*" >&2
	FAIL_COUNT=$((FAIL_COUNT + 1))
}
pass() {
	printf 'desktop_integration_test: PASS: %s\n' "$*"
	PASS_COUNT=$((PASS_COUNT + 1))
}

assert_eq() {
	if [ "$1" = "$2" ]; then
		pass "$3"
	else
		fail "$3 (got: $1, want: $2)"
	fi
}

assert_file() {
	if [ -f "$1" ]; then
		pass "$2"
	else
		fail "$2 (missing: $1)"
	fi
}

Fx=$(mktemp -d "/tmp/streambeats-desktop-test.XXXXXX")
trap 'rm -rf "$Fx"' EXIT

# --- stub infrastructure -----------------------------------------------------
STUB_BIN="$Fx/stub-bin"
mkdir -p "$STUB_BIN"
SYSTEMD_RUN_LOG="$Fx/systemd-run.log"

cat >"$STUB_BIN/systemctl" <<'EOF'
#!/bin/bash
# stub systemctl: `--user is-active --quiet <unit>` returns active when the
# env names that unit or "*" (guard test); everything else fails so the
# spawn path proceeds. show-environment succeeds (systemd_ok()).
ACTIVE="${SYSTEMCTL_STUB_ACTIVE-x}"
if printf '%s\n' "$*" | grep -q "is-active"; then
	unit="${*: -1}"
	if [ "$ACTIVE" = "*" ] || [ "$unit" = "streambeats-panel-${ACTIVE}.service" ]; then
		exit 0
	fi
	exit 3
fi
exit 0
EOF

cat >"$STUB_BIN/systemd-run" <<EOF
#!/bin/bash
printf '%s\n' "\$*" >>"$SYSTEMD_RUN_LOG"
exit 0
EOF

chmod +x "$STUB_BIN/systemctl" "$STUB_BIN/systemd-run"

# --- scenario 1: guard — second launch of the same side must not spawn ------
say "scenario 1: single-instance guard (unit active)"
: >"$SYSTEMD_RUN_LOG"
OUT=$(env PATH="$STUB_BIN:$PATH" SYSTEMCTL_STUB_ACTIVE=right \
	"$PANEL_UNDER_TEST" right 2>&1)
rc1=$?
assert_eq "$rc1" "0" "guard path exits 0"
if printf '%s' "$OUT" | grep -q "already running"; then
	pass "guard notifies instead of duplicating"
else
	fail "guard output missing 'already running' (got: $OUT)"
fi
if [ -s "$SYSTEMD_RUN_LOG" ]; then
	fail "guard path INVOKED systemd-run — the unit lock did not block"
else
	pass "guard path never spawns a second unit"
fi

# --- scenario 2: spawn path — detached unit with per-side name ---------------
say "scenario 2: detached spawn via systemd-run"
: >"$SYSTEMD_RUN_LOG"
env PATH="$STUB_BIN:$PATH" SYSTEMCTL_STUB_ACTIVE= \
	"$PANEL_UNDER_TEST" left >/dev/null 2>&1
rc2=$?
assert_eq "$rc2" "0" "spawn path exits 0"
assert_file "$SYSTEMD_RUN_LOG" "systemd-run was invoked"
INV_LINE=$(head -n 1 "$SYSTEMD_RUN_LOG")
if printf '%s' "$INV_LINE" | grep -q -- "--unit=streambeats-panel-left.service"; then
	pass "unit name keyed per side (streambeats-panel-left)"
else
	fail "unit name wrong: $INV_LINE"
fi
if printf '%s' "$INV_LINE" | grep -q -- "--user.*--collect"; then
	pass "detached + collect flags present"
else
	fail "missing --user/--collect: $INV_LINE"
fi
if printf '%s' "$INV_LINE" | grep -q -- "streambeats-quick.* left"; then
	pass "launcher invoked with the requested side"
else
	fail "launcher/arguments wrong: $INV_LINE"
fi

# --- scenario 3: no systemd — degraded but alive -----------------------------
say "scenario 3: no systemd in a fake installed tree"
: >"$SYSTEMD_RUN_LOG"   # scenario 2 wrote its entries; this scenario must not see them
FAKE_ROOT="$Fx/fake-share"
mkdir -p "$FAKE_ROOT/scripts"
cp "$PANEL_UNDER_TEST" "$FAKE_ROOT/scripts/streambeats-panel"
chmod 0755 "$FAKE_ROOT/scripts/streambeats-panel"
LAUNCH_LOG="$Fx/launch.log"
cat >"$FAKE_ROOT/scripts/streambeats-quick" <<EOF
#!/bin/bash
printf '%s\n' "\$*" >>"$LAUNCH_LOG"
exit 0
EOF
chmod 0755 "$FAKE_ROOT/scripts/streambeats-quick"

# PATH with everything the panel needs EXCEPT systemd-run/systemctl.
NODIR="$Fx/nosystemd"
mkdir -p "$NODIR"
for tool in bash sh dirname readlink mv chmod setsid grep head tr sed mkenv; do
	tool_path=$(command -v "$tool" 2>/dev/null) &&
		ln -s "$tool_path" "$NODIR/$tool"
done

OUT=$(PATH="$NODIR" "$FAKE_ROOT/scripts/streambeats-panel" right 2>&1)
rc3=$?
assert_eq "$rc3" "0" "fallback path exits 0"
if printf '%s' "$OUT" | grep -qE "systemd user session unavailable|background"; then
	pass "fallback explains the degraded mode"
else
	fail "fallback message wrong: $OUT"
fi
if [ -s "$LAUNCH_LOG" ]; then
	LAUNCH_LINE=$(head -n 1 "$LAUNCH_LOG")
	if printf '%s' "$LAUNCH_LINE" | grep -q '^right$'; then
		pass "fallback launched the fake launcher with the side"
	else
		fail "fallback launcher arguments wrong: $LAUNCH_LINE"
	fi
else
	fail "fallback never reached the launcher"
fi
if [ -s "$SYSTEMD_RUN_LOG" ]; then
	fail "fallback invoked systemd-run"
else
	pass "fallback stayed off systemd"
fi

# --- scenario 4: invalid side ------------------------------------------------
say "scenario 4: argument validation"
if env PATH="$STUB_BIN:$PATH" SYSTEMCTL_STUB_ACTIVE= \
	"$PANEL_UNDER_TEST" diagonal >/dev/null 2>&1; then
	fail "invalid side accepted"
else
	pass "invalid side rejected"
fi

# --- scenario 5: installer-generated .desktop entry --------------------------
say "scenario 5: generated .desktop"
STAGE="$Fx/stage"
PREFIX="$Fx/prefix"
mkdir -p "$STAGE/api/releases/tags" "$STAGE/artifacts/dist"
dist="$STAGE/artifacts/dist"
mkdir -p "$dist/scripts" "$dist/assets/ghostty" "$dist/assets/desktop" \
	"$dist/assets/icons" "$dist/assets/kwin"
printf '#!/bin/sh\necho "fake-quick $*"\n' >"$dist/scripts/streambeats-quick"
printf '#!/bin/sh\necho "fake-panel $*"\n' >"$dist/scripts/streambeats-panel"
chmod 0755 "$dist/scripts/streambeats-quick" "$dist/scripts/streambeats-panel"
cat >"$dist/streambeats" <<'EOF'
#!/bin/sh
[ "$1" = "help" ] && { echo "usage: streambeats"; exit 0; }
echo fake-player
EOF
chmod 0755 "$dist/streambeats"
for side in top bottom left right; do
	printf 'window-mode = %s\n' "$side" >"$dist/assets/ghostty/streambeats-$side.conf"
done
cp "$REPO_ROOT/assets/desktop/streambeats-panel.desktop.in" "$dist/assets/desktop/"
cp "$REPO_ROOT/assets/icons/streambeats-panel.svg" "$dist/assets/icons/"
cp "$REPO_ROOT/assets/kwin/window-rules.md" "$dist/assets/kwin/"
printf '# staged README\n' >"$dist/README.md"
tar -C "$dist" --owner=0 --group=0 -czf "$STAGE/artifacts/streambeats-player_1.1.0_linux_amd64.tar.gz" \
	. 2>/dev/null ||
	tar -C "$dist" -czf "$STAGE/artifacts/streambeats-player_1.1.0_linux_amd64.tar.gz" .
sha=$(sha256sum "$STAGE/artifacts/streambeats-player_1.1.0_linux_amd64.tar.gz" | awk '{ print $1 }')
printf '%s  streambeats-player_1.1.0_linux_amd64.tar.gz\n' "$sha" >"$STAGE/artifacts/checksums.txt"
cat >"$STAGE/api/releases/tags/v1.1.0" <<EOF
{"assets":[{"name":"streambeats-player_1.1.0_linux_amd64.tar.gz","browser_download_url":"file://$STAGE/artifacts/streambeats-player_1.1.0_linux_amd64.tar.gz"},{"name":"checksums.txt","browser_download_url":"file://$STAGE/artifacts/checksums.txt"}]}
EOF

STREAMBEATS_API_URL="file://$STAGE/api" \
	STREAMBEATS_RAW_URL="file://$STAGE/rawabsent" \
	STREAMBEATS_INSTALL_PREFIX="$PREFIX" \
	"$REPO_ROOT/scripts/install.sh" --version v1.1.0 >"$Fx/install_out" 2>"$Fx/install_err"
rc5=$?
if [ "$rc5" != 0 ]; then
	say "install stderr tail: $(tail -3 "$Fx/install_err")"
	say "install stdout tail: $(tail -3 "$Fx/install_out")"
fi
assert_eq "$rc5" "0" "install with desktop assets exits 0"
DESKTOP_DEST="$PREFIX/share/applications/streambeats-panel.desktop"
assert_file "$DESKTOP_DEST" "generated .desktop exists"
assert_file "$PREFIX/share/icons/hicolor/scalable/apps/streambeats-panel.svg" "icon installed"
assert_file "$PREFIX/bin/streambeats-panel" "third wrapper installed"
assert_file "$PREFIX/share/streambeats/assets/kwin/window-rules.md" "kwin rules doc installed"
if grep -q "@PREFIX@" "$DESKTOP_DEST" 2>/dev/null; then
	fail "template placeholder leaked into the generated .desktop"
else
	pass "no @PREFIX@ placeholder in the generated .desktop"
fi
if grep -q "Exec=$PREFIX/bin/streambeats-panel right" "$DESKTOP_DEST" &&
	grep -q "Exec=$PREFIX/bin/streambeats-panel top" "$DESKTOP_DEST"; then
	pass "Exec entries carry the absolute prefix (default + side actions)"
else
	fail "Exec entries wrong: $(grep '^Exec=' "$DESKTOP_DEST" | head -2)"
fi
if command -v desktop-file-validate >/dev/null 2>&1; then
	if desktop-file-validate "$DESKTOP_DEST"; then
		pass "desktop-file-validate accepts the generated entry"
	else
		fail "desktop-file-validate rejected: $(desktop-file-validate "$DESKTOP_DEST" 2>&1)"
	fi
else
	say "SKIP: desktop-file-validate unavailable in this environment (excluded, not counted as pass)"
fi

say "----------------------------------------"
say "scenarios: $((PASS_COUNT + FAIL_COUNT)) assertions — $PASS_COUNT pass, $FAIL_COUNT fail"
[ "$FAIL_COUNT" -eq 0 ] || exit 1
exit 0
