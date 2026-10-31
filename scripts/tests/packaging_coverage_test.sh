#!/bin/bash
# packaging_coverage_test.sh — packaging coverage guard (issue #108, S3).
#
# Contract: every asset family scripts/install.sh consumes MUST be declared
# in `.goreleaser.yaml`'s files: list, or a fresh release archive ships
# without it and the installer silently lands nothing (v1.9.0 shipped
# without the kitty presets exactly this way). Fixture-based: reads the
# repo's own packaging config, no network, no archive, no history.
#
# Run:  scripts/tests/packaging_coverage_test.sh

set -u

REPO_ROOT="$(CDPATH='' cd -- "$(dirname -- "$0")/../.." && pwd -P)"
GORELEASER="$REPO_ROOT/.goreleaser.yaml"

PASS_COUNT=0
FAIL_COUNT=0

say() { printf 'packaging_coverage_test: %s\n' "$*"; }
pass() { printf 'packaging_coverage_test: PASS: %s\n' "$*"; PASS_COUNT=$((PASS_COUNT + 1)); }
fail() { printf 'packaging_coverage_test: FAIL: %s\n' "$*" >&2; FAIL_COUNT=$((FAIL_COUNT + 1)); }

if [ ! -f "$GORELEASER" ]; then
	fail "missing .goreleaser.yaml at repo root"
	exit 1
fi

# S3: both panel-template families the installer copies from the archive
# must be declared in the packaging files list.
if grep -q 'assets/ghostty/streambeats-\*\.conf' "$GORELEASER"; then
	pass "ghostty preset family declared in .goreleaser.yaml files"
else
	fail "ghostty preset family (assets/ghostty/streambeats-*.conf) NOT declared in .goreleaser.yaml files"
fi

if grep -q 'assets/kitty/streambeats-\*\.conf' "$GORELEASER"; then
	pass "kitty preset family declared in .goreleaser.yaml files"
else
	fail "kitty preset family (assets/kitty/streambeats-*.conf) NOT declared in .goreleaser.yaml files — the release archive would ship without it (v1.9.0 regression, issue #108)"
fi

# The installer's own expectations must still match the declared file
# names (single family per table row, per side list).
# S1 (issue #122): the shared scanner helper must ride the archive too —
# the wrapper sources it from ITS OWN scripts dir; a missing helper makes
# the kitty relay silently fall into the ghostty path (v1.9.1/v1.9.2
# shipped without it exactly that way).
if grep -q 'scripts/panel-terminal' "$GORELEASER"; then
	pass "panel-terminal helper declared in .goreleaser.yaml files"
else
	fail "panel-terminal helper (scripts/panel-terminal) NOT declared in .goreleaser.yaml files — installed trees lose the kitty relay toggle (issue #122)"
fi

if [ -x "$REPO_ROOT/scripts/tests/install_test.sh" ]; then
	pass "installer expectations test remains available (install_test.sh)"
else
	fail "install_test.sh missing — the installer expectations can drift unverified"
fi

say "scenarios: $((PASS_COUNT + FAIL_COUNT)) assertions — $PASS_COUNT pass, $FAIL_COUNT fail"
[ "$FAIL_COUNT" -eq 0 ] || exit 1
exit 0
