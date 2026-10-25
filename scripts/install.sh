#!/bin/bash
# streambeats-player installer — the full floating-panel experience in one line.
#
#   curl -fsSL https://raw.githubusercontent.com/egkike/streambeats-player/main/scripts/install.sh | bash
#
# Or pinned/non-pipe usage:
#
#   install.sh [--version vX.Y.Z] [--prefix DIR]
#
# What it installs (user-level, no sudo — default prefix ~/.local):
#
#   ~/.local/share/streambeats/bin/streambeats          TUI player binary
#   ~/.local/share/streambeats/scripts/                 quick-terminal launcher
#   ~/.local/share/streambeats/assets/ghostty/*.conf    side presets
#   ~/.local/bin/streambeats                            wrapper
#   ~/.local/bin/streambeats-quick                      wrapper
#
# Bash 3.2 floor — macOS stock /bin/bash must run this file.
# Forbidden constructs: associative arrays, mapfile/readarray, declare -n,
# ${var,,}, ${var^^}, pipefail, set -e. Only set -u.
#
# Release artifacts are verified SHA-256 against the published checksums.txt
# before anything is installed. The extracted layout mirrors the repository
# tree (bin/ + scripts/ + assets/) so scripts/streambeats-quick keeps
# resolving its tree via scripts/.. — see the launcher's repo-root comment.
#
# Override hooks for testing (file:// URLs are valid):
#   STREAMBEATS_API_URL      GitHub API base          (default api.github.com/.../releases)
#   STREAMBEATS_RAW_URL      raw content base         (default raw.githubusercontent.com/...)
#   STREAMBEATS_INSTALL_PREFIX    install prefix      (default ~/.local)

umask 077
set -u

REPO_NAME="streambeats-player"
API_URL="${STREAMBEATS_API_URL:-https://api.github.com/repos/egkike/${REPO_NAME}}"
RAW_URL="${STREAMBEATS_RAW_URL:-https://raw.githubusercontent.com/egkike/${REPO_NAME}}"
VERSION_ARG=""
VERSION_PINNED=""
PREFIX="${STREAMBEATS_INSTALL_PREFIX:-${HOME:-$PWD}/.local}"
CURL_BIN="${STREAMBEATS_CURL:-curl}"

CURRENT_TMP=""
EXTRACT_TMP=""
ARCHIVE_FILE=""
# BASH_SOURCE is unset when read from stdin (`curl | bash -s`); the :-$0
# fallback keeps `set -u` from aborting before main() in pipe mode.

cleanup_tmp() {
	[ -n "$CURRENT_TMP" ] && rm -f "$CURRENT_TMP"
	[ -n "$EXTRACT_TMP" ] && rm -rf "$EXTRACT_TMP"
}
# EXIT for normal exit; INT/TERM/HUP so a Ctrl-C during download leaves no
# temp dir with a half-fetched archive behind.
trap cleanup_tmp EXIT INT TERM HUP

new_tmp() {
	# mktemp on the EXIT trap: CURRENT_TMP is reused per fetch. TMP_DIR_STREAMBEATS
	# is set by main before any fetch path; fall back to /tmp defensively.
	CURRENT_TMP=$(mktemp "${TMP_DIR_STREAMBEATS:-${TMPDIR:-/tmp}}/streambeats-newtmp.XXXXXX") ||
		CURRENT_TMP=$(mktemp) ||
		die "cannot create a temp file"
	printf '%s' "$CURRENT_TMP"
}

fetch() {
	# fetch <url> <output>: curl with silent-timeout posture; errors die.
	"$CURL_BIN" -fsSL --connect-timeout 10 "$1" -o "$2" || {
		die "download failed: $1 (network error, wrong tag, or rate limit)"
	}
}

die() {
	printf 'streambeats-install: error: %s\n' "$*" >&2
	exit 1
}

say() {
	printf 'streambeats-install: %s\n' "$*"
}

warn() {
	printf 'streambeats-install: warning: %s\n' "$*" >&2
}

usage() {
	cat <<'EOF'

Usage: install.sh [--version vX.Y.Z] [--prefix DIR]

  curl -fsSL https://raw.githubusercontent.com/egkike/streambeats-player/main/scripts/install.sh | bash

Installs the full streambeats-player experience user-level (no sudo),
default prefix ~/.local:

  $PREFIX/share/streambeats/
  $PREFIX/bin/streambeats            TUI wrapper (any terminal)
  $PREFIX/bin/streambeats-quick      floating quick-terminal panel wrapper

Options:
  --version vX.Y.Z   install a specific release tag instead of latest
  --prefix DIR       install root (default ~/.local, env: STREAMBEATS_INSTALL_PREFIX)
  -h, --help         show this help
EOF
}

# ---------------------------------------------------------------------------
# Platform mapping: release archives are streamed via unix domain sockets,
# so the runtime contract is POSIX only (see README Requirements).
# ---------------------------------------------------------------------------
map_os() {
	case "$(uname -s)" in
	Linux) printf 'linux' ;;
	Darwin) printf 'darwin' ;;
	*) die "unsupported OS '$(uname -s)': streambeats needs unix sockets + mpv (see README Requirements)" ;;
	esac
}

map_arch() {
	case "$(uname -m)" in
	x86_64 | amd64) printf 'amd64' ;;
	aarch64 | arm64) printf 'arm64' ;;
	*) die "unsupported architecture '$(uname -m)'" ;;
	esac
}

# resolve_version_version: pick the release tag: --version, else the latest
# published release. Output is the exact tag (leading v included).
resolve_version() {
	if [ -n "$VERSION_ARG" ]; then
		case "$VERSION_ARG" in
		v[0-9]*) printf '%s' "$VERSION_ARG" ;;
		v*) die "invalid --version '$VERSION_ARG': use vX.Y.Z" ;;
		*) die "invalid --version '$VERSION_ARG': use vX.Y.Z" ;;
		esac
		return
	fi
	local json tag
	json=$(fetch_json "$API_URL/releases/latest") || die "cannot query the latest release from $API_URL"
	tag=$(printf '%s' "$json" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1)
	[ -n "$tag" ] || die "no tag_name found in $API_URL/releases/latest (are GitHub releases published?)"
	printf '%s' "$tag"
}

fetch_json() {
	local out
	out=$(new_tmp)
	"$CURL_BIN" -fsSL --connect-timeout 10 "$1" >"$out" || return 1
	cat "$out"
}

# resolve_container: locate the release asset object line. From now on,
# the GitHub releases JSON comment above applies to both functions.
release_asset_line() { # <json> <name-suffix-regex>
	# Flatten ANY layout to one line (pretty or minified), then break BEFORE
	# every asset object so name + url keys always share exactly one line —
	# the field anchor never depends on the API's formatting. awk gsub keeps
	# the backslash-n escape portable (GNU/BSD awk).
	printf '%s\n' "$1" | tr '\n' ' ' |
		awk '{ gsub(/,[[:space:]]*\{/, "\n{"); print }' |
		grep "$2" | head -n 1
}

url_from_asset_line() { # <asset-object-line>
	printf '%s\n' "$1" |
		sed -n 's/.*"browser_download_url"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p'
}

resolve_asset() {
	local json="$1" os="$2" arch="$3" entry url
	entry=$(release_asset_line "$json" "_${os}_${arch}\.tar\.gz") || return 1
	url=$(url_from_asset_line "$entry") || return 1
	[ -n "$url" ] || return 1
	printf '%s' "$url"
}

resolve_checksums() {
	local json="$1" entry url
	entry=$(release_asset_line "$json" '"browser_download_url".*checksums\.txt') || return 1
	url=$(url_from_asset_line "$entry") || return 1
	[ -n "$url" ] || return 1
	printf '%s' "$url"
}

# archive_name: derive the tarball's file name from its download URL.
archive_name() {
	printf '%s' "${1##*/}"
}

# verify_archive: SHA-256 the downloaded tarball against checksums.txt.
# Mismatch aborts BEFORE any file is installed.
verify_archive() {
	local archive="$1" checksums="$2"
	local expected got tool
	expected=$(grep " $(basename "$archive")\$" "$checksums" | awk 'NR == 1 { print $1 }')
	[ -n "$expected" ] || die "checksums.txt has no entry for $(basename "$archive")"
	if command -v sha256sum >/dev/null 2>&1; then
		tool=sha256sum
	else
		tool='shasum -a 256'
	fi
	got=$($tool "$archive" | awk '{ print $1 }') || die "cannot hash $archive"
	[ "$got" = "$expected" ] || die "checksum mismatch for $(basename "$archive"): expected $expected, got $got"
}

# ---------------------------------------------------------------------------
# Install layout
# ---------------------------------------------------------------------------
install_bundle() {
	local prefix="$1" share="$2" ver="$3"
	mkdir -p "$share/bin" "$share/scripts" "$share/assets/ghostty" "$prefix/bin" ||
		die "cannot create install directories under $prefix (is HOME writable?)"

	# Binary moves to bin/ so scripts/streambeats-quick resolves its tree.
	# Regular archives keep it at the archive root; fall back to the rare
	# pre-staged layout where bin/ already exists.
	if [ -f "$EXTRACT_TMP/bin/streambeats" ]; then
		:
	else
		mkdir -p "$EXTRACT_TMP/bin" || die "cannot stage the archive staging dir"
		mv "$EXTRACT_TMP/streambeats" "$EXTRACT_TMP/bin/streambeats" ||
			die "archive does not contain a streambeats binary"
	fi

	mv "$EXTRACT_TMP/bin/streambeats" "$share/bin/streambeats" ||
		die "cannot install $share/bin/streambeats"
	chmod 0755 "$share/bin/streambeats"

	# Launcher + presets: from the archive when the tag ships them, else
	# pulled from raw content at the same tag (unchecksummed fallback).
	if [ -f "$EXTRACT_TMP/scripts/streambeats-quick" ]; then
		mv "$EXTRACT_TMP/scripts/streambeats-quick" "$share/scripts/streambeats-quick"
		mv "$EXTRACT_TMP/assets/ghostty/"*.conf "$share/assets/ghostty/" 2>/dev/null || true
		chmod 0755 "$share/scripts/streambeats-quick"
	else
		warn "release archive predates bundled launcher assets; fetching them from raw.githubusercontent (NOT checksum covered)"
		local tmpfile
		tmpfile=$(new_tmp)
		fetch "$RAW_URL/${VERSION_PINNED}/scripts/streambeats-quick" "$tmpfile" ||
			die "archive has no launcher assets and raw fallback failed; see README Requirements for a manual install"
		mv "$tmpfile" "$share/scripts/streambeats-quick"
		chmod 0755 "$share/scripts/streambeats-quick"
		local side
		for side in top bottom left right; do
			tmpfile=$(new_tmp)
			fetch "$RAW_URL/${VERSION_PINNED}/assets/ghostty/streambeats-$side.conf" "$tmpfile" ||
				die "raw fallback: preset streambeats-$side.conf download failed"
			mv "$tmpfile" "$share/assets/ghostty/streambeats-$side.conf"
		done
	fi

	# Desktop integration extras (optional by design): panel spawn script,
	# icon, KWin rules doc; from this tag forward they ride inside the
	# checksum-verified archive, older tags fall back to raw content.
	local have_panel=1
	consume_optional_asset "scripts/streambeats-panel" \
		"$share/scripts/streambeats-panel" 0755 || have_panel=0
	local icons_dir="$prefix/share/icons/hicolor/scalable/apps"
	mkdir -p "$icons_dir" "$share/assets/kwin" ||
		die "cannot create the desktop integration directories"
	if [ "$have_panel" = 1 ]; then
		consume_optional_asset "assets/icons/streambeats-panel.svg" \
			"$icons_dir/streambeats-panel.svg" 0644 || have_panel=0
	fi
	consume_optional_asset "assets/kwin/window-rules.md" \
		"$share/assets/kwin/window-rules.md" 0644 || true
	if [ "$have_panel" = 1 ]; then
		local desktop_src="$EXTRACT_TMP/assets/desktop/streambeats-panel.desktop.in"
		if [ ! -f "$desktop_src" ]; then
			desktop_src="$(new_tmp)"
			consume_optional_asset "assets/desktop/streambeats-panel.desktop.in" \
				"$desktop_src" 0644 || have_panel=0
		fi
		if [ -f "$desktop_src" ]; then
			generate_desktop_file "$prefix" "$desktop_src"
		else
			warn "streambeats-panel.desktop generation skipped (template unavailable)"
		fi
	else
		warn "desktop integration (streambeats-panel) skipped: assets missing"
	fi

	# Run from the repo copy's docs; cheap and keeps the installed tree whole.
	[ -f "$EXTRACT_TMP/README.md" ] && cp "$EXTRACT_TMP/README.md" "$share/README.md"
	[ -f "$EXTRACT_TMP/LICENSE" ] && cp "$EXTRACT_TMP/LICENSE" "$share/LICENSE"

	write_wrappers "$prefix" "$share"
	write_manifest "$prefix" "$share"
}

# write_manifest: record the canonical installed paths for the `streambeats
# uninstall` CLI (issue #9): uninstall is registry-driven and the manifest is
# its registry. The six paths are stable by design (absent entries uninstall
# as "already absent"), and user data (~/Music/StreamBeats, catalog cache) is
# NEVER listed — only --cache clears the cache opt-in.
write_manifest() { # <prefix> <share>
	local prefix="$1" share="$2" m
	m="$(new_tmp)"
	{
		printf '# streambeats-player install manifest — consumed by: streambeats uninstall\n'
		printf '%s\n' \
			"$share" \
			"$prefix/bin/streambeats" \
			"$prefix/bin/streambeats-quick" \
			"$prefix/bin/streambeats-panel" \
			"$prefix/share/applications/streambeats-panel.desktop" \
			"$prefix/share/icons/hicolor/scalable/apps/streambeats-panel.svg"
	} >"$m"
	mv "$m" "$share/manifest.txt"
	chmod 0644 "$share/manifest.txt"
}

# fetch_raw_asset: pull one support file from the repository content at the
# same release tag. Raw bytes are NOT checksum covered — warned at call site.
fetch_raw_asset() { # <rel-path> <dest>
	local rel="$1" dest="$2" tmpfile
	tmpfile=$(new_tmp)
	# curl directly, NOT fetch(): a missing raw file must degrade gracefully
	# (the caller skips with a warn) instead of killing the whole install.
	if ! "$CURL_BIN" -fsSL --connect-timeout 10 \
		"$RAW_URL/${VERSION_PINNED}/$rel" -o "$tmpfile"; then
		rm -f "$tmpfile"
		return 1
	fi
	mv "$tmpfile" "$dest"
}

# consume_optional_asset: take a support file from the extracted archive when
# the tag ships it, else pull it from raw content at the same tag. Missing on
# BOTH sides is carried by the caller: desktop integration degrades with a
# warning, but the player itself never depends on these files.
consume_optional_asset() { # <rel-path> <dest> <mode>
	local rel="$1" dest="$2" mode="$3"
	if [ -f "$EXTRACT_TMP/$rel" ]; then
		mv "$EXTRACT_TMP/$rel" "$dest" || return 1
	elif fetch_raw_asset "$rel" "$dest"; then
		warn "fetched $rel from raw content at $VERSION_PINNED (NOT checksum covered)"
	else
		warn "release archive lacks $rel and raw fallback failed — skipping it"
		return 1
	fi
	chmod "$mode" "$dest"
}

# escape_sed_repl: make a string safe as a sed REPLACEMENT (backslash, ampersand
# and pipe are special there; pipe doubles as our delimiter).
escape_sed_repl() {
	printf '%s' "$1" | sed 's/\\/\\\\/g; s/[&|]/\\&/g'
}

# generate_desktop_file: render the shipped template with the ABSOLUTE install
# prefix (@PREFIX@) so the .desktop Exec/Icon never depend on PATH.
generate_desktop_file() { # <prefix> <template-path>
	local prefix="$1" tpl="$2" esc dest
	esc=$(escape_sed_repl "$prefix")
	dest="$(new_tmp)"
	sed "s|@PREFIX@|$esc|g" "$tpl" >"$dest" ||
		die "cannot render streambeats-panel.desktop from the template"
	mkdir -p "$prefix/share/applications" ||
		die "cannot create $prefix/share/applications"
	mv "$dest" "$prefix/share/applications/streambeats-panel.desktop"
	chmod 0644 "$prefix/share/applications/streambeats-panel.desktop"
	if command -v update-desktop-database >/dev/null 2>&1; then
		update-desktop-database "$prefix/share/applications" >/dev/null 2>&1 || true
	fi
}

# escape_sh_single: make a path safe inside SINGLE quotes. Single-quoted
# strings in sh expand NOTHING (no $cmd-substitution, no backticks), so only
# the single quote itself needs escaping (' -> '\'' closes, escapes, reopens).
# Double-quoted embeds are unusable here: they keep $ and ` live, which would
# let a crafted --prefix name execute commands every time the wrapper runs.
escape_sh_single() {
	printf '%s' "$1" | sed "s/'/'\\\\''/g"
}

write_wrappers() {
	# Wrapper skeletons live in single-quoted heredocs; the share path is the
	# ONLY interpolated value and goes through escape_sh_single, so no
	# $substitution or backtick inside a hostile --prefix name survives.
	local prefix="$1" share="$2" esc
	esc=$(escape_sh_single "$share")
	local w dest
	for w in streambeats streambeats-quick streambeats-panel; do
		dest="$(new_tmp)"
		{
			printf '#!/bin/sh\n'
			case "$w" in
			streambeats)
				printf "exec '%s/bin/streambeats' \"\$@\"\n" "$esc"
				;;
			streambeats-quick)
				printf "exec '%s/scripts/streambeats-quick' \"\$@\"\n" "$esc"
				;;
			*)
				printf "exec '%s/scripts/streambeats-panel' \"\$@\"\n" "$esc"
				;;
			esac
		} >"$dest"
		mv "$dest" "$prefix/bin/$w" || die "cannot install wrapper $prefix/bin/$w"
		chmod 0755 "$prefix/bin/$w"
	done
}

smoke_test() {
	local share="$1"
	if "$share/bin/streambeats" help >/dev/null 2>&1; then
		say "installed binary answers 'help' cleanly"
	else
		warn "installed binary did not answer 'help' — inspect $share/bin/streambeats manually"
	fi
}

environment_advisories() {
	local prefix="$1"
	case ":$PATH:" in
	*":$prefix/bin:"*) ;;
	*) warn "$prefix/bin is not in PATH — add it ('export PATH=\"$prefix/bin:\$PATH\"') or call with its full path" ;;
	esac
	if command -v mpv >/dev/null 2>&1; then
		say "mpv found at $(command -v mpv)"
	else
		warn "mpv was NOT found in PATH — the TUI opens but cannot play; install mpv 0.41+ (see README Requirements)"
	fi
	if command -v ghostty >/dev/null 2>&1; then
		say "ghostty found at $(command -v ghostty)"
	else
		warn "ghostty was NOT found in PATH — the floating quick-terminal panel needs it (the TUI itself does not)"
	fi
}

main() {
	TMP_DIR_STREAMBEATS="$(mktemp -d "${TMPDIR:-/tmp}/streambeats-install-tar.XXXXXX")" ||
		die "cannot create temp working dir"
	EXTRACT_TMP="$TMP_DIR_STREAMBEATS/extract"
# the tarball keeps the asset's real name — verify_archive matches
	# its SHASS entry by file name.
	ARCHIVE_FILE="$TMP_DIR_STREAMBEATS/release.tar.gz"

	while [ "$#" -gt 0 ]; do
		case "$1" in
		-h | --help)
			usage
			return 0
			;;
		--version)
			[ -n "${2:-}" ] || die "--version needs an argument like v1.1.0"
			VERSION_ARG="$2"
			shift
			;;
		--prefix)
			[ -n "${2:-}" ] || die "--prefix needs an argument"
			PREFIX="$2"
			shift
			;;
		*) die "unknown argument '$1' — see --help" ;;
		esac
		shift
	done

	say "resolving release version"
	VERSION_PINNED=$(resolve_version)
	local ver="${VERSION_PINNED#v}"
	local os arch json
	os=$(map_os)
	arch=$(map_arch)
	say "target: ${os}_${arch}, release ${VERSION_PINNED}"

	say "querying release assets"
	json=$(fetch_json "$API_URL/releases/tags/$VERSION_PINNED") ||
		die "cannot fetch $API_URL/releases/tags/$VERSION_PINNED"
	local asset_url
	asset_url=$(resolve_asset "$json" "$os" "$arch") ||
		die "release ${VERSION_PINNED} has no ${os}_${arch} tar.gz asset"
	# keep the asset's real file name — verify_archive matches its
	# checksums.txt entry by name, not by our temp dir's choice.
	ARCHIVE_FILE="$TMP_DIR_STREAMBEATS/$(archive_name "$asset_url")"

	fetch "$asset_url" "$ARCHIVE_FILE"
	say "verifying SHA-256"
	local checksum_url
	checksum_url=$(resolve_checksums "$json") || die "clean release artifacts missing a checksums.txt verifier — refusing to install"
	local checksums_file
	checksums_file=$(new_tmp)
	fetch "$checksum_url" "$checksums_file"
	verify_archive "$ARCHIVE_FILE" "$checksums_file"
	say "extracting"
	mkdir -p "$EXTRACT_TMP" || die "cannot stage the extraction dir"
	tar -xzf "$ARCHIVE_FILE" -C "$EXTRACT_TMP" || die "extract failed for $ARCHIVE_FILE"

	share="$PREFIX/share/streambeats"
	say "installing bundle to $share"
	install_bundle "$PREFIX" "$share" "$ver"

	say "checking wrappers"
	smoke_test "$share"
	environment_advisories "$PREFIX"

	say "done — try:  $PREFIX/bin/streambeats-quick right"
	say "uninstall:   $PREFIX/bin/streambeats uninstall (add --cache to also clear the catalog cache)"
}

if [ "${BASH_SOURCE[0]:-$0}" = "$0" ]; then
	main "$@"
fi
