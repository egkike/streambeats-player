#!/bin/bash
# launcher_terminal_test.sh — stub-based harness for the streambeats-quick
# panel-terminal resolution (issue #90).
#
# A sandbox repo keeps the launcher isolated from the developer machine: its
# own scripts/ and assets/ trees, a stub player, and a leading-PATH stub dir
# where ghostty / kitty / kitten only record their argv. The precedence chain
# (--terminal flag > settings.json panelTerminal > builtin ghostty then
# kitty) is asserted on those argv logs.
#
# Run:  scripts/tests/launcher_terminal_test.sh

set -u

REPO_ROOT="$(CDPATH='' cd -- "$(dirname -- "$0")/../.." && pwd -P)"

PASS_COUNT=0
FAIL_COUNT=0

say() { printf 'launcher_terminal_test: %s\n' "$*"; }
pass() { printf 'launcher_terminal_test: PASS: %s\n' "$*"; PASS_COUNT=$((PASS_COUNT + 1)); }
fail() { printf 'launcher_terminal_test: FAIL: %s\n' "$*" >&2; FAIL_COUNT=$((FAIL_COUNT + 1)); }
assert_eq() {
	if [ "$1" = "$2" ]; then pass "$3"; else fail "$3 (got: $1, want: $2)"; fi
}

Fx=$(mktemp -d "/tmp/streambeats-launcher-test.XXXXXX")
trap 'rm -rf "$Fx"' EXIT

STUBS="$Fx/stubs"
mkdir "$STUBS"

# The launcher under test: a copy so the sandbox tree owns scripts/ and
# assets/, exactly like an installed prefix mirror.
mkdir -p "$Fx/scripts" "$Fx/assets/ghostty" "$Fx/assets/kitty"
cp "$REPO_ROOT/scripts/streambeats-quick" "$Fx/scripts/streambeats-quick"
for side in top bottom left right; do
	cp "$REPO_ROOT/assets/ghostty/streambeats-$side.conf" "$Fx/assets/ghostty/" 2>/dev/null
	printf '# sandbox kitty preset (%s)\n' "$side" >"$Fx/assets/kitty/streambeats-$side.conf"
done
LAUNCHER="$Fx/scripts/streambeats-quick"

# A stub player at sandbox bin/streambeats (the launcher's preferred path).
mkdir "$Fx/bin"
cat >"$Fx/bin/streambeats" <<'EOF'
#!/bin/sh
echo "player ran"
EOF
chmod 0755 "$Fx/bin/streambeats"

# Stubs: each records argv for a given tag then exits 0.
stub() {
	name=$1 tag=$2
	cat >"$STUBS/$name" <<EOF
#!/bin/sh
echo "$tag argv: \$*" >> "$Fx/$tag.log"
exit 0
EOF
	chmod 0755 "$STUBS/$name"
}
# ghostty stub: `+help` is the CLI-action listing (the zero-side-effect
# probe the launcher reads; the control file drops the quick action for a
# pre-1.4-style stub); every other invocation is a spawn, recorded in full.
cat >"$STUBS/ghostty" <<EOF
#!/bin/sh
if [ "\$1" = "+help" ]; then
	if [ -f "$Fx/no_quick_action" ]; then
		printf '%s\n' '+new-window' '+new-tab'
	else
		printf '%s\n' '+new-window' '+new-tab' '+toggle-quick-terminal'
	fi
	exit 0
fi
echo "GHOSTTY argv: \$*" >> "$Fx/GHOSTTY.log"
exit 0
EOF
chmod 0755 "$STUBS/ghostty"

# gdbus stub: records every call (dest + action) and plays the failure
# modes the quick-mode scenarios need via control files in $Fx:
#   gdbus_fail_first : call #1 (the pre-spawn toggle probe) fails
#			     ServiceUnknown; later calls succeed (claim settled)
#   gdbus_uncertain	 : every call fails with a transport-level error
# Default: the name is claimed and the call succeeds.
stub gdbus GDBUS
cat >"$STUBS/gdbus" <<EOF
#!/bin/sh
echo "GDBUS argv: \$*" >> "$Fx/GDBUS.log"
n=\$(( \$(cat "$Fx/gdbus_calls.count" 2>/dev/null || echo 0) + 1 ))
echo "\$n" > "$Fx/gdbus_calls.count"
if [ -f "$Fx/gdbus_uncertain" ]; then
	printf 'connection reset by peer\n' >&2
	exit 1
fi
if [ -f "$Fx/gdbus_refuter" ]; then
	case "\$n" in
	1)
		printf 'failure: org.freedesktop.DBus.Error.ServiceUnknown: The name org.streambeats.panel.right was not provided by any .service files\n' >&2
		exit 1
		;;
	2)
		printf 'failure: GDBus.Error:org.gtk.GDBus.UnregisteredGActionError: No such action toggle-quick-terminal\n' >&2
		exit 1
		;;
	*)
		exit 0
		;;
	esac
fi
if [ -f "$Fx/gdbus_fail_first" ] && [ "\$n" -le 1 ]; then
	printf 'failure: org.freedesktop.DBus.Error.ServiceUnknown: The name org.streambeats.panel.right was not provided by any .service files\n' >&2
	exit 1
fi
exit 0
EOF
chmod 0755 "$STUBS/gdbus"
stub kitty KITTY
stub kitten KITTEN

latest() { tail -1 "$Fx/$1.log" 2>/dev/null; }
count() { [ -f "$Fx/$1.log" ] && wc -l <"$Fx/$1.log" | tr -d ' ' || echo 0; }

# run_launcher: runs the launcher in a controlled env; args are its arguments.
run_launcher() {
	env -u XDG_CONFIG_HOME PATH="$STUBS:/usr/bin:/bin" \
		XDG_CONFIG_HOME="$Fx/xdg" \
		"$LAUNCHER" "$@" >/dev/null 2>"$Fx/stderr.log"
}

# error paths (die) must NOT succeed:
run_launcher_expect_fail() {
	if env -u XDG_CONFIG_HOME PATH="$STUBS:/usr/bin:/bin" \
		XDG_CONFIG_HOME="$Fx/xdg" \
		"$LAUNCHER" "$@" >"$Fx/out.log" 2>"$Fx/stderr.log"; then
		return 1
	fi
	return 0
}

write_settings() {
	printf '%s' "$1" >"$Fx/xdg/streambeats/settings.json"
}
mkxdg() { mkdir -p "$Fx/xdg/streambeats"; }

# --- scenario 1: the explicit flag wins over the PATH ------------------------
say "scenario 1: --terminal=kitty beats ghostty-in-PATH"
mkxdg
: >"$Fx/GHOSTTY.log"
if run_launcher --terminal=kitty right; then pass "terminal=kitty (flag) exits 0"; else fail "terminal=kitty (flag) exited nonzero: $(cat "$Fx/stderr.log")"; fi
K=$(latest KITTEN)
if printf '%s' "$K" | grep -q 'quick-access-terminal'; then
	pass "kitten quick-access-terminal invoked by the flag"
else
	fail "kitten did not run for --terminal=kitty: $K"
fi
if printf '%s' "$K" | grep -q "$Fx/assets/kitty/streambeats-right.conf"; then
	pass "the kitty side preset rides the spawn"
else
	fail "kitty preset missing from the spawn: $K"
fi
assert_eq "$(count GHOSTTY)" 0 "ghostty stub never invoked under --terminal=kitty (flag precedence)"

# --- scenario 2: settings.json panelTerminal beats the builtin order ---------
say "scenario 2: settings.json panelTerminal=kitty beats builtin ghostty"
: >"$Fx/KITTEN.log"
mkxdg
write_settings '{"theme":"oscuro","panelTerminal":"kitty"}'
if run_launcher right; then pass "settings-driven runner exits 0"; else fail "settings-driven runner exited nonzero: $(cat "$Fx/stderr.log")"; fi
K=$(latest KITTEN)
if printf '%s' "$K" | grep -q 'quick-access-terminal'; then
	pass "settings.json panelTerminal drives a kitty spawn"
else
	fail "settings.json panelTerminal ignored: $K"
fi
assert_eq "$(count GHOSTTY)" 0 "no ghostty spawn after settings.json kitty"

# --- scenario 3: the flag wins over settings.json -----------------------------
say "scenario 3: --terminal=ghostty overrides settings kitty"
: >"$Fx/KITTEN.log"
mkxdg
write_settings '{"theme":"oscuro","panelTerminal":"kitty"}'
if run_launcher --terminal=ghostty right; then pass "flag>settings runner exits 0"; else fail "flag>settings runner exited nonzero: $(cat "$Fx/stderr.log")"; fi
G="$(latest GHOSTTY)"
if [ -z "$G" ]; then
	pass "flag>settings fast-path toggles a claimed host (no spawn)"
else
	fail "flag>settings spawned although the host was already claimed: $G"
fi
D="$(latest GDBUS)"
if printf '%s' "$D" | grep -q 'toggle-quick-terminal'; then
	pass "flag>settings toggles via the gdbus quick action"
else
	fail "gdbus toggle lost under flag>settings: $D"
fi
assert_eq "$(count KITTEN)" 0 "kitty never spawned while the flag says ghostty"

# --- scenario 4: malformed / foreign settings fall through fail-closed --------
say "scenario 4: settings fail-through (malformed + foreign value)"
mkxdg
write_settings '{not json at all'
if run_launcher right; then pass "malformed settings falls through"; else fail "malformed settings crashed: $(cat "$Fx/stderr.log")"; fi
assert_eq "$(count KITTEN)" 0 "kitty untouched under malformed settings"
G=$(latest GHOSTTY)
if [ -z "$G" ]; then pass "malformed settings keeps the builtin ghostty path (toggle only)"; else fail "malformed settings spawned a second host: $G"; fi
if [ -n "$(latest GDBUS)" ]; then
	pass "malformed settings rides the quick-mode host"
else
	fail "malformed settings quick host lost"
fi

: >"$Fx/GHOSTTY.log"
mkxdg
write_settings '{"theme":"oscuro","panelTerminal":"wezterm"}'
if run_launcher right; then pass "foreign panelTerminal ignored"; else fail "foreign panelTerminal crashed: $(cat "$Fx/stderr.log")"; fi
assert_eq "$(count KITTEN)" 0 "wezterm never offered (whitelist only)"

# --- scenario 5: builtin order: ghostty first ---------------------------------
say "scenario 5: builtin resolution picks ghostty when both exist"
mkxdg
rm -f "$Fx/xdg/streambeats/settings.json"
: >"$Fx/KITTEN.log"
: >"$Fx/GHOSTTY.log"
if run_launcher right; then pass "no flag, no settings exits 0"; else fail "builtin runner exited nonzero: $(cat "$Fx/stderr.log")"; fi
assert_eq "$(count KITTEN)" 0 "builtin order starts with ghostty, kitty never touched"
G=$(latest GHOSTTY)
if [ -z "$G" ]; then pass "builtin order stays on the ghostty quick host (toggle only)"; else fail "builtin order spawned a second host: $G"; fi
if [ -n "$(latest GDBUS)" ]; then
	pass "builtin order runs the ghostty quick host"
else
	fail "builtin ghostty quick host missing"
fi

say "scenario 5b: kitty-only PATH auto-resolves kitty"
ONLY="$Fx/onlykitty"
mkdir "$ONLY"
cp "$STUBS/kitty" "$STUBS/kitten" "$ONLY/"
for bin in grep tail wc dirname env uname; do cp "$(command -v "$bin")" "$ONLY/" 2>/dev/null; done
if env -u XDG_CONFIG_HOME PATH="$ONLY" XDG_CONFIG_HOME="$Fx/xdg" \
	"$LAUNCHER" right >/dev/null 2>"$Fx/stderr.log"; then
	if [ -n "$(latest KITTEN)" ]; then pass "kitty-only PATH resolves kitty"; else fail "kitty resolved but no kitten log"; fi
else
	fail "kitty-only path exited nonzero: $(cat "$Fx/stderr.log")"
fi

# --- scenario 6: unsupported explicit terminal is rejected --------------------
say "scenario 6: --terminal=wezterm rejected with usage"
if run_launcher_expect_fail --terminal=wezterm right; then
	pass "--terminal=wezterm fails closed"
else
	fail "--terminal=wezterm must be rejected (exit != 0)"
fi

# --- scenario 7: flag + side compose ------------------------------------------
say "scenario 7: --terminal=kitty bottom uses the bottom preset"
: >"$Fx/KITTEN.log"
if run_launcher --terminal=kitty bottom; then pass "kitty bottom exits 0"; else fail "kitty bottom exited nonzero: $(cat "$Fx/stderr.log")"; fi
K=$(latest KITTEN)
if printf '%s' "$K" | grep -q "$Fx/assets/kitty/streambeats-bottom.conf"; then
	pass "side composition reaches the kitty preset"
else
	fail "bottom preset lost for kitty: $K"
fi

# --- scenario 8: the ghostty quick-mode stderr contract ---------------------
# (issue #134: the ghostty host rides the native quick terminal, kitty
# parity). The summary keeps the plain 'side=... preset=... player=' shape
# with NO terminal= token; the quick note prints ONCE (kitty prints its
# relay note once); no 'unavailable' fallback marker on the happy path.
say "scenario 8: ghostty quick-mode stderr contract (summary + single note)"
: >"$Fx/GHOSTTY.log"
: >"$Fx/GDBUS.log"; : >"$Fx/gdbus_calls.count"
rm -f "$Fx/ghostty_stderr.log"
if env -u XDG_CONFIG_HOME PATH="$STUBS:/usr/bin:/bin" XDG_CONFIG_HOME="$Fx/xdg" \
	"$LAUNCHER" right 2>"$Fx/ghostty_stderr.log" >/dev/null; then
	STDERR="$Fx/ghostty_stderr.log"
	SUM=$(grep -c '^streambeats-quick: side=' "$STDERR")
	assert_eq "$SUM" 1 "quick path prints the summary line exactly once (kitty parity)"
	NOTES=$(grep -c 'rides the native quick' "$STDERR")
	assert_eq "$NOTES" 1 "quick note prints once"
	if grep -q 'unavailable' "$STDERR"; then
		fail "fallback marker printed on the quick-mode happy path"
	else
		pass "no fallback marker on the quick-mode happy path"
	fi
else
	fail "quick-mode stderr scenario exited nonzero: $(cat "$Fx/ghostty_stderr.log")"
fi

# --- scenario 9: settings.json read anchoring (issue #92, R2/R3/R4) ----------
# The read resolves ONLY a top-level panelTerminal key. A hand-nested JSON
# object must never inject a panel terminal inside the key's own object, and
# the machine-written shapes (Marshalled 2-space pretty and minified flat)
# must both keep working.
say "scenario 9: anchored settings read (nested JSON is out of contract)"

# 9a RED target: a nested "panelTerminal" key must NOT resolve kitty.
: >"$Fx/KITTEN.log"
write_settings '{
  "theme": {
    "panelTerminal": "kitty"
  }
}'
if run_launcher right; then pass "nested panelTerminal fixture exits 0"; else fail "nested fixture crashed: $(cat "$Fx/stderr.log")"; fi
assert_eq "$(count KITTEN)" 0 "nested panelTerminal must never resolve kitty (anchored read)"
G=$(latest GHOSTTY)
G=$(latest GDBUS)
if [ -n "$G" ]; then pass "nested fixture falls through to builtin ghostty"; else fail "nested fixture lost the builtin fallback"; fi

# 9b pin: the TUI's machine shape (2-space MarshalIndent) keeps resolving.
: >"$Fx/KITTEN.log"
write_settings '{
  "theme": "oscuro",
  "panelTerminal": "kitty"
}'
if run_launcher right; then pass "pretty machine shape exits 0"; else fail "pretty machine shape crashed: $(cat "$Fx/stderr.log")"; fi
if printf '%s' "$(latest KITTEN)" | grep -q 'quick-access-terminal'; then
	pass "pretty machine shape still resolves kitty"
else
	fail "anchoring broke the machine-written pretty shape: $(latest KITTEN)"
fi

# 9c pin: hand-minified flat JSON keeps resolving too.
: >"$Fx/KITTEN.log"
write_settings '{"theme":"oscuro","panelTerminal":"kitty"}'
if run_launcher right; then pass "minified flat shape exits 0"; else fail "minified flat shape crashed: $(cat "$Fx/stderr.log")"; fi
if printf '%s' "$(latest KITTEN)" | grep -q 'quick-access-terminal'; then
	pass "minified flat shape still resolves kitty"
else
	fail "anchoring broke the minified shape: $(latest KITTEN)"
fi

# 9d RED target: a NESTED key inside a hand-minified JSON is equally out of
# contract — the anchor must read depth, not just indentation.
: >"$Fx/KITTEN.log"
: >"$Fx/GHOSTTY.log"
write_settings '{"theme":{"panelTerminal":"kitty"},"note":"x"}'
if run_launcher right; then pass "1-line nested fixture exits 0"; else fail "1-line nested fixture crashed: $(cat "$Fx/stderr.log")"; fi
assert_eq "$(count KITTEN)" 0 "nested minified panelTerminal must never resolve kitty"
G=$(latest GDBUS)
if [ -n "$G" ]; then pass "nested minified falls through to builtin ghostty"; else fail "nested minified lost the builtin fallback"; fi

# --- scenario 10: quick-mode spawn-if-not-claimed (first press) --------------
# gdbus_fail_first simulates 'nothing claimed the name yet': call #1 fails
# ServiceUnknown (no instance), the launcher spawns the headless dedicated
# host with the per-side app-id and toggles inside the wait loop.
say "scenario 10: quick host spawn on first press with claim settling"
: >"$Fx/GHOSTTY.log"; : >"$Fx/GDBUS.log"; : >"$Fx/gdbus_calls.count"; rm -f "$Fx/gdbus_uncertain"
rm -f "$Fx/ghostty_stderr.log"
touch "$Fx/gdbus_fail_first"
if env -u XDG_CONFIG_HOME PATH="$STUBS:/usr/bin:/bin" XDG_CONFIG_HOME="$Fx/xdg" \
	"$LAUNCHER" right 2>"$Fx/ghostty_stderr.log" >/dev/null; then
	pass "first press (no claim) exits 0 after spawn+toggle"
else
	fail "first press failed: $(cat "$Fx/ghostty_stderr.log")"
fi
G="$(latest GHOSTTY)"
if printf '%s' "$G" | grep -q 'gtk-single-instance=true' && printf '%s' "$G" | grep -q 'initial-window=false' && printf '%s' "$G" | grep -q -- '--class=org.streambeats.panel.right'; then
	pass "spawn claims the per-side app-id headless"
else
	fail "quick host spawn lost: $G"
fi
D=$(count GDBUS)
if [ "$D" -ge 2 ]; then pass "probe toggle + wait-loop toggle ran ($D gdbus calls)"; else fail "wait-loop did not retry (gdbus calls: $D)"; fi
if printf '%s' "$(latest GDBUS)" | grep -q 'toggle-quick-terminal'; then
	pass "wait-loop toggle reached the quick action"
else
	fail "wait-loop toggle lost: $(latest GDBUS)"
fi
if ! printf '%s' "$G" | grep -q -- '-e env'; then
	pass "quick host spawn carries no -e child (command comes from the config)"
else
	fail "quick host spawn still passes -e (config command ignored): $G"
fi
CONFDIR=$(sed -n 's|^streambeats-quick: note: temp config kept at ||p' "$Fx/ghostty_stderr.log" | tail -1)
if [ -n "$CONFDIR" ] && [ -x "$CONFDIR/run-player.sh" ]; then
	pass "per-run player wrapper generated and executable"
else
	fail "player wrapper missing at $CONFDIR"
fi
if [ -n "$CONFDIR" ] && grep -q "command = $CONFDIR/run-player.sh" "$CONFDIR/ghostty/config"; then
	pass "generated config points command at the wrapper"
else
	fail "generated config lost the command wiring: $CONFDIR/ghostty/config"
fi
if [ -n "$CONFDIR" ] && grep -q '^class = org.streambeats.panel.right' "$CONFDIR/ghostty/config"; then
	pass "generated config pins the per-side app-id class"
else
	fail "generated config lost the per-side class: $CONFDIR"
fi
if [ -n "$CONFDIR" ] && grep -q '^initial-window = false' "$CONFDIR/ghostty/config"; then
	pass "generated config stays headless"
else
	fail "generated config lost initial-window=false: $CONFDIR"
fi
if [ -n "$CONFDIR" ] && grep -q 'STREAMBEATS_PANEL_USER_XDG' "$CONFDIR/run-player.sh"; then
	pass "wrapper restores the player's real XDG home (issue #80 contract)"
else
	fail "wrapper lost the XDG reset: $CONFDIR/run-player.sh"
fi
NOTES=$(grep -c 'rides the native quick' "$Fx/ghostty_stderr.log")
assert_eq "$NOTES" 1 "quick note prints once on the first-press path"
rm -f "$Fx/gdbus_fail_first"

# --- scenario 11: later press: claim exists, toggle only, no spawn -----------
say "scenario 11: later press toggles the quick terminal (no spawn)"
: >"$Fx/GHOSTTY.log"; : >"$Fx/GDBUS.log"; : >"$Fx/gdbus_calls.count"
rm -f "$Fx/ghostty_stderr.log"
if env -u XDG_CONFIG_HOME PATH="$STUBS:/usr/bin:/bin" XDG_CONFIG_HOME="$Fx/xdg" \
	"$LAUNCHER" right 2>"$Fx/ghostty_stderr.log" >/dev/null; then
	pass "press on claimed name exits 0"
else
	fail "claimed-name press failed: $(cat "$Fx/ghostty_stderr.log")"
fi
assert_eq "$(count GHOSTTY)" 0 "later press never respawns a host"
assert_eq "$(count GDBUS)" 1 "later press is exactly one toggle"

# --- scenario 12: ghostty predates the quick action → dedicated fallback -----
say "scenario 12: 1.3.1-style ghostty falls back to the dedicated window"
: >"$Fx/GHOSTTY.log"; : >"$Fx/GDBUS.log"; : >"$Fx/gdbus_calls.count"; rm -f "$Fx/gdbus_uncertain"
touch "$Fx/no_quick_action"
rm -f "$Fx/ghostty_stderr.log"
if env -u XDG_CONFIG_HOME PATH="$STUBS:/usr/bin:/bin" XDG_CONFIG_HOME="$Fx/xdg" \
	"$LAUNCHER" right 2>"$Fx/ghostty_stderr.log" >/dev/null; then
	pass "dedicated fallback exits 0"
else
	fail "dedicated fallback failed: $(cat "$Fx/ghostty_stderr.log")"
fi
G="$(latest GHOSTTY)"
if printf '%s' "$G" | grep -q -- '-e env' && printf '%s' "$G" | grep -q 'gtk-single-instance=false'; then
	pass "fallback keeps the exact dedicated-window spawn (legacy contract)"
else
	fail "dedicated spawn changed: $G"
fi
assert_eq "$(count GDBUS)" 0 "fallback never talks D-Bus"
NOTES=$(grep -c 'unavailable' "$Fx/ghostty_stderr.log")
assert_eq "$NOTES" 2 "fallback notes print twice (doubled-note doctrine preserved)"
rm -f "$Fx/no_quick_action"

# --- scenario 13: uncertain D-Bus failure fails loud, never spawns 2nd panel -
say "scenario 13: uncertain gdbus failure dies loud (no dual panels)"
: >"$Fx/GHOSTTY.log"; : >"$Fx/GDBUS.log"; : >"$Fx/gdbus_calls.count"
touch "$Fx/gdbus_uncertain"
if run_launcher_expect_fail right; then
	pass "uncertain gdbus failure fails closed (nonzero exit)"
else
	fail "uncertain gdbus failure must exit nonzero"
fi
assert_eq "$(count GHOSTTY)" 0 "uncertain failure never spawns a second panel"
assert_eq "$(count GDBUS)" 1 "uncertain failure stops at the first probe"
if grep -q 'unexpectedly' "$Fx/stderr.log"; then
	pass "uncertain failure names the reason (loud diagnostics)"
else
	fail "uncertain failure lost its diagnostic: $(cat "$Fx/stderr.log")"
fi
rm -f "$Fx/gdbus_uncertain"

# --- scenario 14: settings-ghostty + side composition reaches its own app-id  -
say "scenario 14: settings ghostty + left side composes org.streambeats.panel.left"
mkxdg
: >"$Fx/GHOSTTY.log"; : >"$Fx/GDBUS.log"; : >"$Fx/gdbus_calls.count"
write_settings '{"theme":"oscuro","panelTerminal":"ghostty"}'
if run_launcher left >/dev/null; then
	if printf '%s' "$(latest GDBUS)" | grep -q 'org.streambeats.panel.left'; then
		pass "settings-ghostty left press targets its own app-id"
	else
		fail "left-side app-id wrong: $(latest GDBUS)"
	fi
else
	fail "settings-ghostty left press failed: $(cat "$Fx/stderr.log")"
fi
rm -f "$Fx/xdg/streambeats/settings.json"

# --- scenario 15: graceful host retirement on a mid-handshake failure --------
# (review escalación R3-001): a surfaceless spawned host is retired through
# its own quit GAction (gdbus Activate quit) — a client process must never
# die mid-display; the TERM kill is the last resort when the bus is gone.
# gdbus_refuter mode: call #1 ServiceUnknown (no claim → spawn), #2 returns
# an action-level failure (claimed but unusable → retirement path), #3+
# succeed so the quit GAction call records.
say "scenario 15: host retirement uses the quit GAction, never a blind kill"
: >"$Fx/GHOSTTY.log"; : >"$Fx/GDBUS.log"; : >"$Fx/gdbus_calls.count"; rm -f "$Fx/gdbus_uncertain"
touch "$Fx/gdbus_refuter"
rm -f "$Fx/ghostty_stderr.log"
if env -u XDG_CONFIG_HOME PATH="$STUBS:/usr/bin:/bin" XDG_CONFIG_HOME="$Fx/xdg" \
	"$LAUNCHER" right 2>"$Fx/ghostty_stderr.log" >/dev/null; then
	pass "mid-handshake failure falls back to the dedicated window (exit 0)"
else
	fail "mid-handshake fallback failed: $(cat "$Fx/ghostty_stderr.log")"
fi
QUIT_CALL=$(grep -c 'Activate quit' "$Fx/GDBUS.log")
assert_eq "$QUIT_CALL" 1 "retirement walks the quit GAction (gdbus Activate quit)"
if grep -q 'unavailable' "$Fx/ghostty_stderr.log"; then
	pass "retirement lands on the documented fallback"
else
	fail "retirement lost the fallback notes"
fi
rm -f "$Fx/gdbus_refuter"

# --- fixture presence: the REAL repo ships every kitty side preset ------------
say "fixture: assets/kitty ships all four side presets"
if [ -f "$REPO_ROOT/assert_file.sh" ]; then :; fi
for side in top bottom left right; do
	if [ -f "$REPO_ROOT/assets/kitty/streambeats-$side.conf" ]; then
		pass "kitty preset streambeats-$side.conf exists"
	else
		fail "kitty preset streambeats-$side.conf missing"
	fi
done

say "----------------------------------------"
say "scenarios: $((PASS_COUNT + FAIL_COUNT)) assertions — $PASS_COUNT pass, $FAIL_COUNT fail"
[ "$FAIL_COUNT" -eq 0 ] || exit 1
exit 0
