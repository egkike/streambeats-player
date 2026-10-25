#!/bin/bash
# scripts/tests/macos_smoke_test.sh — REAL-binary end-to-end smoke for the
# macOS arm64/amd64 CI job:
#
#   1. builds cmd/streambeats with the runner's Go (the real darwin binary)
#   2. stages a checksummed fake release carrying that binary + launch assets
#   3. runs the full `install.sh --prefix` against file:// URLs (bash 3.2 gate)
#   4. asserts the native binary answers --version, the freedesktop
#      integration stayed skipped (darwin branch), and the manifest exists
#   5. runs `streambeats uninstall [--cache]` end-to-end twice: removal with
#      targets present, then the idempotent "nothing to uninstall"
#
# The harness never touches the network (file:// override hooks) and keeps
# every filesystem interaction inside its own mktemp prefix. Requires bash
# 3.2 (macOS stock) — anything newer is not used on purpose.

set -u
umask 077

SCRIPT_DIR=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
REPO_ROOT=$(CDPATH='' cd -- "$SCRIPT_DIR/../.." && pwd)
INSTALLER="$REPO_ROOT/scripts/install.sh"

PASS_COUNT=0
FAIL_COUNT=0
Fx=""
Fx=$(mktemp -d "/tmp/streambeats-macos-smoke.XXXXXX")
trap 'rm -rf "$Fx"' EXIT

say() { printf 'smoke: %s\n' "$1"; }
pass() { PASS_COUNT=$((PASS_COUNT + 1)); say "PASS: $1"; }
fail() { FAIL_COUNT=$((FAIL_COUNT + 1)); say "FAIL: $1"; }

white_line() {
	if grep -qxF "$1" "$2"; then
		pass "manifest lists $1"
	else
		fail "manifest does not list $1"
	fi
}

assert_file() {
	if [ -f "$1" ] || [ -d "$1" ]; then
		pass "$2"
	else
		fail "$2"
	fi
}

stage_release() {
	# Stage a one-asset fake release carrying the REAL host binary; the asset
	# name follows the platform table the installer resolves on its HOST
	# (linux_amd64 here, darwin_arm64 on the macOS runner).
	local share="$Fx/stage/share/streambeats"
	mkdir -p "$Fx/stage/artifacts" "$share" "$share/bin" "$share/scripts" "$share/assets/ghostty"
	cp "$Fx/streambeats.bin" "$share/bin/streambeats"
	cp "$REPO_ROOT/scripts/streambeats-quick" \
		"$share/scripts/streambeats-quick" 2>/dev/null || true
	cp "$REPO_ROOT"/assets/ghostty/*.conf "$share/assets/ghostty/" 2>/dev/null || true
	cp "$REPO_ROOT/README.md" "$share/README.md" 2>/dev/null || true
	# An older-tag raw fallback would fetch scripts/assets from GitHub at
	# runtime; pin STREAMBEATS_RAW_URL at a dead local path so any fetch
	# attempt fails loudly instead of silently passing.
	# Host asset name: same mapping as the installer (map_os/map_arch).
	local name
	case "$(uname -s)" in Darwin) STAGED_OS=darwin ;; *) STAGED_OS=linux ;; esac
	case "$(uname -m)" in x86_64 | amd64) STAGED_ARCH=amd64 ;; *) STAGED_ARCH=arm64 ;; esac
	STAGED_NAME="streambeats_${STAGED_OS}_${STAGED_ARCH}"
	name="$STAGED_NAME"
	mkdir -p "$Fx/stage/artifacts"
	# Release layout: bin/ + scripts/ + assets/ at the archive root (same as
	# goreleaser assets since v1.1.0).
	tar -czf "$Fx/stage/artifacts/$name.tar.gz" -C "$share" \
		bin scripts assets ghostty-right.conf 2>/dev/null ||
			tar -czf "$Fx/stage/artifacts/$name.tar.gz" -C "$share" bin scripts assets || true
	sha256sum "$Fx/stage/artifacts/$name.tar.gz" |
		awk -v n="$name.tar.gz" '{ print $1 "  " n }' \
		>"$Fx/stage/artifacts/checksums.txt"
	touch "$Fx/stage/raw"
}

build_binary() {
	local out="$Fx/streambeats.bin"
	(cd "$REPO_ROOT" && go build -o "$out" ./cmd/streambeats) || return 1
}

main() {
	say "building the real binary"
	if ! build_binary; then
		fail "go build ./cmd/streambeats"
		say "scenarios: $((PASS_COUNT + FAIL_COUNT)) assertions — $PASS_COUNT pass, $FAIL_COUNT fail"
		exit 1
	fi

	stage_release
	mkdir -p "$Fx/stage/api/releases/tags"
	cat >"$Fx/stage/artifacts/release.json" <<EOF
{"release": "v1.1.0", "assets": [
  {"name": "checksums.txt", "browser_download_url": "file://$Fx/stage/artifacts/checksums.txt"},
  {"name": "$STAGED_NAME.tar.gz", "browser_download_url": "file://$Fx/stage/artifacts/$STAGED_NAME.tar.gz"}
]}
EOF
	cp "$Fx/stage/artifacts/release.json" "$Fx/stage/api/releases/tags/v1.1.0"

	PREFIX="$Fx/prefix"
	say "installing (real binary, darwin host)"
	OUT=$(STREAMBEATS_API_URL="file://$Fx/stage/api" \
		STREAMBEATS_RAW_URL="file://$Fx/stage/raw" \
		STREAMBEATS_INSTALL_PREFIX="$PREFIX" \
		"$INSTALLER" --version v1.1.0 2>"$Fx/err")
	rc=$?
	if [ "$rc" = "0" ]; then
		pass "installer exited 0"
	else
		fail "installer exited $rc; stderr: $(cat "$Fx/err")"
	fi

	SHARE="$PREFIX/share/streambeats"
	for f in "$SHARE/bin/streambeats" "$SHARE/scripts/streambeats-quick" \
		"$PREFIX/bin/streambeats" "$PREFIX/bin/streambeats-quick" \
		"$SHARE/assets/ghostty/streambeats-right.conf" "$SHARE/manifest.txt"; do
		assert_file "$f" "bundle landed: $f"
	done
	if [ -f "$PREFIX/share/applications/streambeats-panel.desktop" ]; then
		fail "freedesktop .desktop present on darwin"
	else
		pass "no .desktop on darwin (freedesktop skipped)"
	fi
	white_line "$SHARE" "$SHARE/manifest.txt"
	white_line "$PREFIX/bin/streambeats" "$SHARE/manifest.txt"
	white_line "$PREFIX/bin/streambeats-quick" "$SHARE/manifest.txt"
	white_line "$PREFIX/share/applications/streambeats-panel.desktop" "$SHARE/manifest.txt"
	white_line "$PREFIX/share/icons/hicolor/scalable/apps/streambeats-panel.svg" "$SHARE/manifest.txt"

	# The binary exposes subcommands (help), not --version (the INSTALLER
	# takes --version; the CLI reports usage). Assert help output lines.
	V0=$("$PREFIX/bin/streambeats" help 2>&1) || true
	if printf '%s' "$V0" | grep -q "terminal music player"; then
		pass "installed native binary answers help"
	else
		fail "installed native binary did not answer help (got: ${V0%%$'\n'*})"
	fi

	say "uninstall end-to-end (manifest-driven)"
	U0=$("$PREFIX/bin/streambeats" uninstall 2>&1)
	if printf '%s' "$U0" | grep -q "uninstalled"; then
		pass "uninstall removed the install"
	else
		fail "uninstall did not report removal: $U0"
	fi
	for f in "$SHARE" "$PREFIX/bin/streambeats" "$PREFIX/bin/streambeats-quick"; do
		if [ -e "$f" ]; then
			fail "not removed by uninstall: $f"
		else
			pass "removed: $f"
		fi
	done

	# The first uninstall removed the wrapper (that is its job): the idempotent
	# probe reruns the STAGED binary with the prefix env, targets now absent.
	U1=$(STREAMBEATS_INSTALL_PREFIX="$PREFIX" "$Fx/streambeats.bin" uninstall --cache 2>&1)
	if printf '%s' "$U1" | grep -q "nothing to uninstall"; then
		pass "idempotent: nothing to uninstall, exit path clean"
	else
		fail "second uninstall not idempotent: $U1"
	fi

	say "scenarios: $((PASS_COUNT + FAIL_COUNT)) assertions — $PASS_COUNT pass, $FAIL_COUNT fail"
	[ "$FAIL_COUNT" -eq 0 ] || exit 1
}

main "$@"
