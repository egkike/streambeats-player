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
KROTOOL_LOG="$Fx/kdotool.log"

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

# kdotool stub: hermetic replacement for the REAL binary the owner now has
# at ~/.local/bin (a PATH-leading stub keeps every scenario desktop-safe).
# - search: prints KROTOOL_STUB_WINDOW (window id) when set; else exit 1.
# - getactivewindow: prints KROTOOL_STUB_ACTIVE_WINDOW when set; else exit 1.
# - windowactivate/windowminimize/windowstate: record and succeed.
cat >"$STUB_BIN/kdotool" <<EOF
#!/bin/bash
for arg in "\$@"; do
	printf '%s\n' "\$arg" >>"$KROTOOL_LOG"
	case "\$arg" in
		search) [ -n "\${KROTOOL_STUB_WINDOW:-}" ] && printf '%s\n' "\$KROTOOL_STUB_WINDOW" || exit 1; exit 0 ;;
		getactivewindow) [ -n "\${KROTOOL_STUB_ACTIVE_WINDOW:-}" ] && printf '%s\n' "\$KROTOOL_STUB_ACTIVE_WINDOW" || exit 1; exit 0 ;;
		windowactivate|windowstate) ;;
		windowminimize) [ -n "\${KROTOOL_STUB_FAIL_MINIMIZE:-}" ] && exit 1 || exit 0 ;;
	esac
done
exit 0
EOF

# notify-send stub: hermetic desktop notifications. With STUB_BIN leading
# PATH, notify() in streambeats-panel routes through it, so message
# assertions target NOTIFY_LOG and never depend on (or leak to) a real
# desktop daemon on the test machine.
NOTIFY_LOG="$Fx/notify.log"
cat >"$STUB_BIN/notify-send" <<EOF
#!/bin/bash
printf '%s\n' "\$*" >>"$NOTIFY_LOG"
exit 0
EOF

chmod +x "$STUB_BIN/systemctl" "$STUB_BIN/systemd-run" "$STUB_BIN/kdotool" \
	"$STUB_BIN/notify-send"

# ghostty stub (issue #80): records its own XDG_CONFIG_HOME plus the full -e
# argv, so the harness can pin the player's config-home passthrough.
GHOSTTY_LOG="$Fx/ghostty.log"
cat >"$STUB_BIN/ghostty" <<EOF
#!/bin/bash
{
	printf 'env XDG_CONFIG_HOME=%s\n' "\${XDG_CONFIG_HOME:-}"
	printf 'argv:'
	for a in "\$@"; do printf ' %s' "\$a"; done
	printf '\n'
} >>"$GHOSTTY_LOG"
exit 0
EOF
chmod +x "$STUB_BIN/ghostty"

# --- scenario 1: guard — second launch of the same side must not spawn ------
say "scenario 1: single-instance guard (unit active)"
: >"$SYSTEMD_RUN_LOG"
OUT=$(env PATH="$STUB_BIN:$PATH" SYSTEMCTL_STUB_ACTIVE=right \
	"$PANEL_UNDER_TEST" right 2>&1)
rc1=$?
assert_eq "$rc1" "0" "guard path exits 0"
if grep -q "already running" "$NOTIFY_LOG"; then
	pass "guard notifies instead of duplicating"
else
	fail "guard never notified 'already running' (notify log: $(cat "$NOTIFY_LOG" 2>/dev/null))"
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
# the fallback backgrounds the launcher (setsid … &): give it a bounded
# window to write its log instead of racing it (was a flaky assertion)
launch_wait=0
while [ ! -s "$LAUNCH_LOG" ] && [ "$launch_wait" -lt 20 ]; do
	sleep 0.1
	launch_wait=$((launch_wait + 1))
done
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
toggle_execs=$(grep -c '^Exec=.*streambeats-panel toggle' "$DESKTOP_DEST")
if grep -q "Exec=$PREFIX/bin/streambeats-panel toggle right" "$DESKTOP_DEST" &&
	grep -q "Exec=$PREFIX/bin/streambeats-panel toggle top" "$DESKTOP_DEST" &&
	[ "$toggle_execs" -eq 5 ]; then
	pass "Exec entries launch the toggle (default + 4 side actions)"
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

# --- scenario 6: toggle mode (two-press doctrine) ----------------------------
# toggle <side> keeps the unit guard: inactive unit -> spawn; active unit ->
# resolve by ACTIVE window: panel focused = visible -> minimize (audio kept);
# otherwise -> activate (restores minimized, raises visible-unfocused).
say "scenario 6: toggle mode"
: >"$KROTOOL_LOG"

# 6a: unit active, panel window exists, focus elsewhere -> windowactivate
OUT=$(env PATH="$STUB_BIN:$PATH" SYSTEMCTL_STUB_ACTIVE=right KROTOOL_STUB_WINDOW=W-TEST \
	"$PANEL_UNDER_TEST" toggle right 2>&1)
rc_toggle=$?
assert_eq "$rc_toggle" "0" "toggle (unfocused) exits 0"
if grep -q 'windowactivate' "$KROTOOL_LOG"; then
	pass "toggle (unfocused) activated the panel"
else
	fail "toggle (unfocused) never activated the panel (log: $(cat "$KROTOOL_LOG" 2>/dev/null))"
fi
if grep -qE '^windowminimize' "$KROTOOL_LOG"; then
	fail "toggle (unfocused) wrongly minimized the visible panel"
fi

# 6b: focus ON the panel -> minimize
: >"$KROTOOL_LOG"
OUT=$(env PATH="$STUB_BIN:$PATH" SYSTEMCTL_STUB_ACTIVE=right KROTOOL_STUB_WINDOW=W-TEST \
	KROTOOL_STUB_ACTIVE_WINDOW=W-TEST \
	"$PANEL_UNDER_TEST" toggle right 2>&1)
rc_focus=$?
assert_eq "$rc_focus" "0" "toggle (focused) exits 0"
if grep -qE '^windowminimize' "$KROTOOL_LOG"; then
	pass "toggle (focused) minimized the panel"
else
	fail "toggle (focused) never minimized the panel (log: $(cat "$KROTOOL_LOG" 2>/dev/null))"
fi
if grep -q 'windowactivate' "$KROTOOL_LOG"; then
	fail "toggle (focused) wrongly activated"
fi
if grep -q "panel minimized" "$NOTIFY_LOG"; then
	pass "toggle (focused) explains music keeps playing"
else
	fail "toggle (focused) missing 'panel minimized' notification (log: $(cat "$NOTIFY_LOG" 2>/dev/null))"
fi

# 6c: unit active but no panel window anywhere -> explain, never fatal
: >"$KROTOOL_LOG"
OUT=$(env PATH="$STUB_BIN:$PATH" SYSTEMCTL_STUB_ACTIVE=right KROTOOL_STUB_WINDOW= \
	"$PANEL_UNDER_TEST" toggle right 2>&1)
rc_ghost=$?
assert_eq "$rc_ghost" "0" "toggle (no window) exits 0"
if grep -q "no window found" "$NOTIFY_LOG"; then
	pass "toggle (no window) explains the ghost state"
else
	fail "toggle (no window) message wrong (notify log: $(cat "$NOTIFY_LOG" 2>/dev/null))"
fi

# 6d: minimize command fails -> tell the caller, never fake success (R3-001)
: >"$KROTOOL_LOG"
rc_fail=0
env PATH="$STUB_BIN:$PATH" SYSTEMCTL_STUB_ACTIVE=right KROTOOL_STUB_WINDOW=W-TEST \
	KROTOOL_STUB_ACTIVE_WINDOW=W-TEST KROTOOL_STUB_FAIL_MINIMIZE=1 \
	"$PANEL_UNDER_TEST" toggle right >/dev/null 2>&1 || rc_fail=$?
if [ "$rc_fail" -ne 0 ]; then
	pass "toggle (minimize fails) exits nonzero, not faking success"
else
	fail "toggle (minimize fails) exited 0 with the toggle unapplied"
fi
if grep -q "could not minimize" "$NOTIFY_LOG"; then
	pass "toggle (minimize fails) notifies the failure"
else
	fail "toggle (minimize fails) is silent (notify log: $(cat "$NOTIFY_LOG" 2>/dev/null))"
fi

: >"$SYSTEMD_RUN_LOG"
: >"$KROTOOL_LOG"
rc6e=0
env PATH="$STUB_BIN:$PATH" SYSTEMCTL_STUB_ACTIVE= \
	"$PANEL_UNDER_TEST" toggle right >/dev/null 2>&1 || rc6e=$?
assert_eq "$rc6e" "0" "toggle (no unit) exits 0"
if grep -q -e "--unit=streambeats-panel-right.service" "$SYSTEMD_RUN_LOG"; then
	pass "toggle (no unit) spawned the detached unit"
else
	fail "toggle (no unit) did not spawn (systemd-run log: $(cat "$SYSTEMD_RUN_LOG"))"
fi
if [ -s "$KROTOOL_LOG" ]; then
	fail "toggle (no unit) invoked kdotool — it must spawn instead"
else
	pass "toggle (no unit) never touches kdotool"
fi

# 6e: toggle with NO unit -> spawns exactly like plain <side> (issue #73 S2:
# Exec=toggle is safe by construction — first press spawns)
: >"$SYSTEMD_RUN_LOG"
: >"$KROTOOL_LOG"
rc6e=0
env PATH="$STUB_BIN:$PATH" SYSTEMCTL_STUB_ACTIVE= \
	"$PANEL_UNDER_TEST" toggle right >/dev/null 2>&1 || rc6e=$?
assert_eq "$rc6e" "0" "toggle (no unit) exits 0"
if grep -q -e "--unit=streambeats-panel-right.service" "$SYSTEMD_RUN_LOG"; then
	pass "toggle (no unit) spawned the detached unit"
else
	fail "toggle (no unit) did not spawn (systemd-run log: $(cat "$SYSTEMD_RUN_LOG"))"
fi
if [ -s "$KROTOOL_LOG" ]; then
	fail "toggle (no unit) invoked kdotool — it must spawn instead"
else
	pass "toggle (no unit) never touches kdotool"
fi

# --- scenario 7: player config-home passthrough (issue #80) ------------------
# The panel's ghostty needs the virtual XDG (preset-only, #57), but the
# PLAYER it spawns must see the USER's real config home — otherwise settings
# and user-dirs resolution break inside the panel.
say "scenario 7: player config-home passthrough"
QUICK_UNDER_TEST="$REPO_ROOT/scripts/streambeats-quick"
# 7a: user WITHOUT XDG_CONFIG_HOME -> the child runs with it unset
if OUT7A=$(env -u XDG_CONFIG_HOME PATH="$STUB_BIN:$PATH" \
	"$QUICK_UNDER_TEST" right 2>&1); then
	pass "quick (no user xdg) exits 0"
else
	fail "quick (no user xdg) exited nonzero: $OUT7A"
fi
GVIRT=$(grep '^env ' "$GHOSTTY_LOG" | tail -1)
if printf '%s' "$GVIRT" | grep -q 'XDG_CONFIG_HOME=.*streambeats-ghostty'; then
	pass "ghostty still runs with the virtual preset XDG"
else
	fail "ghostty lost the virtual XDG: $GVIRT"
fi
GARG=$(grep '^argv:' "$GHOSTTY_LOG" | tail -1)
if printf '%s' "$GARG" | grep -q -- '-e env -u XDG_CONFIG_HOME'; then
	pass "player child runs with XDG_CONFIG_HOME unset (never the /tmp virtual)"
else
	fail "player child passthrough wrong (no user xdg): $GARG"
fi

# 7b: user WITH XDG_CONFIG_HOME -> the child keeps THEIR value
: >"$GHOSTTY_LOG"
if OUT7B=$(env XDG_CONFIG_HOME=/custom/user-cfg PATH="$STUB_BIN:$PATH" \
	"$QUICK_UNDER_TEST" right 2>&1); then
	pass "quick (custom user xdg) exits 0"
else
	fail "quick (custom user xdg) exited nonzero: $OUT7B"
fi
GARG=$(grep '^argv:' "$GHOSTTY_LOG" | tail -1)
if printf '%s' "$GARG" | grep -q -- '-e env XDG_CONFIG_HOME=/custom/user-cfg'; then
	pass "player child keeps the user's custom XDG_CONFIG_HOME"
else
	fail "player child passthrough wrong (custom xdg): $GARG"
fi

say "----------------------------------------"
say "scenarios: $((PASS_COUNT + FAIL_COUNT)) assertions — $PASS_COUNT pass, $FAIL_COUNT fail"
[ "$FAIL_COUNT" -eq 0 ] || exit 1
exit 0
