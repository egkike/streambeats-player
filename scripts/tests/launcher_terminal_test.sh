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
stub ghostty GHOSTTY
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
G=$(latest GHOSTTY)
if printf '%s' "$G" | grep -q -- '-e env'; then
	pass "flag>settings still spawns the ghostty panel"
else
	fail "ghostty spawn lost under flag>settings: $G"
fi
assert_eq "$(count KITTEN)" 0 "kitty never spawned while the flag says ghostty"

# --- scenario 4: malformed / foreign settings fall through fail-closed --------
say "scenario 4: settings fail-through (malformed + foreign value)"
mkxdg
write_settings '{not json at all'
if run_launcher right; then pass "malformed settings falls through"; else fail "malformed settings crashed: $(cat "$Fx/stderr.log")"; fi
assert_eq "$(count KITTEN)" 0 "kitty untouched under malformed settings"
G=$(latest GHOSTTY)
if [ -n "$G" ]; then pass "malformed settings keeps the builtin ghostty path"; else fail "builtin path lost with malformed settings"; fi

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
if [ -n "$G" ]; then pass "builtin ghostty spawn on the plain path"; else fail "builtin ghostty missing: log empty"; fi

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

say "----------------------------------------"
say "scenarios: $((PASS_COUNT + FAIL_COUNT)) assertions — $PASS_COUNT pass, $FAIL_COUNT fail"
[ "$FAIL_COUNT" -eq 0 ] || exit 1
exit 0
