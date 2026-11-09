#!/bin/bash
# policy_surface_test.sh — contribution surface invariants (issues #188, #190).
#
# Contract: the contribution surface lives in files no Go test can reach, and two
# of its invariants degrade SILENTLY. Both are pinned here:
#
#   S1 (review R2-001) the `area` dropdown is duplicated across the two issue
#      forms. PREMISE this test cannot verify: GitHub issue forms cannot share an
#      option list across files. What is pinned is the consequence — the two
#      lists must never drift.
#   S2 (review R4-001) close-unsolicited-prs.yml must keep its fail-closed shape:
#      an allowlist over author_association, no checkout of the head, and no
#      `${{ }}` expression interpolated into `run:` — the injection vector on
#      that trigger.
#
# What this test does NOT prove: that the deny branch works at runtime. That
# needs a real outside pull request and no local test can manufacture one, so the
# assertions below are about FORM. Say it out loud rather than let a green check
# imply more.
#
# `set -e` is deliberately ABSENT (review R1-001). This harness is a sequence of
# independent assertions: a failing `grep -q`, a failing `diff`, or a non-zero
# counter is its control flow. `set -e` would abort on the first one instead of
# reporting all of them. `set -u` stays: a typo in a variable name must not
# silently compare two empty strings and pass.
#
# Fixtures are the repo's own files: no network, no history.
#
# Run:  scripts/tests/policy_surface_test.sh

set -u

REPO_ROOT="$(CDPATH='' cd -- "$(dirname -- "$0")/../.." && pwd -P)"
BUG_FORM="$REPO_ROOT/.github/ISSUE_TEMPLATE/bug_report.yml"
FEATURE_FORM="$REPO_ROOT/.github/ISSUE_TEMPLATE/feature_request.yml"
REDIRECT="$REPO_ROOT/.github/workflows/close-unsolicited-prs.yml"

PASS_COUNT=0
FAIL_COUNT=0

say() { printf 'policy_surface_test: %s\n' "$*"; }
pass() { printf 'policy_surface_test: PASS: %s\n' "$*"; PASS_COUNT=$((PASS_COUNT + 1)); }
fail() { printf 'policy_surface_test: FAIL: %s\n' "$*" >&2; FAIL_COUNT=$((FAIL_COUNT + 1)); }

for f in "$BUG_FORM" "$FEATURE_FORM" "$REDIRECT"; do
	if [ ! -f "$f" ]; then
		fail "missing ${f#"$REPO_ROOT"/}"
	fi
done
if [ "$FAIL_COUNT" -ne 0 ]; then
	say "0 assertions ran — prerequisites missing"
	exit 1
fi

# Normalises a newline- or comma-separated list into one sorted, space-joined line.
option_set() { tr ',' '\n' | tr -d '[]' | tr -d ' ' | sed '/^$/d' | sort | tr '\n' ' '; }

# --- S1: the area options are identical in both forms ------------------------
# Reads from `id: area` to `validations:`. No YAML parser on purpose: the shape
# is stable and adding a parser dependency to a shell test would be worse.
area_options() {
	awk '
		/^[[:space:]]*id: area$/ { inarea = 1; next }
		inarea && /^[[:space:]]*validations:/ { exit }
		inarea && /^[[:space:]]*- / { sub(/^[[:space:]]*- /, ""); print }
	' "$1"
}

# An extraction anchor that moved returns an empty list from BOTH files, and an
# empty pair compares equal. Assert the anchor itself first (review R3-003).
for f in "$BUG_FORM" "$FEATURE_FORM"; do
	if grep -qE '^[[:space:]]*id: area$' "$f"; then
		pass "extraction anchor 'id: area' present in ${f##*/}"
	else
		fail "extraction anchor 'id: area' missing in ${f##*/}: the parity check would compare nothing to nothing"
	fi
done

bug_opts="$(area_options "$BUG_FORM")"
feature_opts="$(area_options "$FEATURE_FORM")"

if [ -z "$bug_opts" ] || [ -z "$feature_opts" ]; then
	fail "could not extract the area options from both forms — the extraction anchor moved"
else
	bug_count="$(printf '%s\n' "$bug_opts" | grep -c . || true)"
	feature_count="$(printf '%s\n' "$feature_opts" | grep -c . || true)"

	# A partial extraction that happens to be equal in both files would pass the
	# parity check while proving nothing (review R3-003).
	if [ "$bug_count" -lt 2 ]; then
		fail "extracted only $bug_count area option(s) from bug_report.yml: the block shape changed and parity would be vacuously true"
	elif [ "$bug_count" -ne "$feature_count" ]; then
		fail "area option counts differ: bug_report=$bug_count feature_request=$feature_count — add the option to BOTH forms (review R2-001)"
	elif ! diff <(printf '%s\n' "$bug_opts") <(printf '%s\n' "$feature_opts") >/dev/null; then
		fail "area options differ between the two forms (review R2-001): $(diff <(printf '%s\n' "$bug_opts") <(printf '%s\n' "$feature_opts") | tr '\n' ' ')"
	else
		pass "area options identical across both issue forms ($bug_count options)"
	fi
fi

# --- S2a: the trigger that carries the write token ---------------------------
if grep -qE '^[[:space:]]*pull_request_target:[[:space:]]*$' "$REDIRECT"; then
	pass "trigger is pull_request_target"
else
	fail "trigger is not pull_request_target: the job no longer runs in the base-branch context where it holds its write token"
fi

# Both YAML spellings of the same list are accepted, because the behaviour is
# identical and matching one literal would fail a correct workflow (review R3-001).
types_value() {
	awk '
		/^[[:space:]]*types:/ {
			rest = $0
			sub(/^[[:space:]]*types:[[:space:]]*/, "", rest)
			if (rest ~ /\[/) { gsub(/[][ ]/, "", rest); print rest; exit }
			inblock = 1
			next
		}
		inblock && /^[[:space:]]*-[[:space:]]*/ {
			v = $0
			sub(/^[[:space:]]*-[[:space:]]*/, "", v)
			sub(/[[:space:]]*$/, "", v)
			print v
			next
		}
		inblock { exit }
	' "$1" | option_set
}

got_types="$(types_value "$REDIRECT")"
want_types="$(printf 'opened\nreopened\n' | option_set)"
if [ "$got_types" = "$want_types" ]; then
	pass "trigger types are opened + reopened (either YAML spelling)"
else
	fail "trigger types parsed as [$got_types], expected [$want_types]: a PR opened by an outside author may never be redirected"
fi

# --- S2b: never execute contributor code ------------------------------------
if grep -q 'actions/checkout' "$REDIRECT"; then
	fail "an actions/checkout step is present: on pull_request_target this would check out and run contributor code with a write token"
else
	pass "no actions/checkout — contributor code is never checked out"
fi

# --- S2c: the guard is an allowlist, not a deny-list ------------------------
# The associations are parsed out of the `fromJSON('[ … ]')` literal itself, not
# grepped out of the whole file: a stray `"OWNER"` in a comment or an `env:` block
# used to satisfy a whole-file grep while the allowlist was missing it
# (reviews R3-002 and R1-002). An enumerated deny-list fails OPEN — an
# association GitHub adds later would leave that PR open by omission.
array_literal="$(grep -oE "fromJSON\('\[[^]]*\]'\)" "$REDIRECT" | head -n 1)"

if [ -z "$array_literal" ]; then
	fail "no fromJSON('[ … ]') allowlist literal found: the guard may have been rewritten into something this test cannot read"
else
	got_assoc="$(printf '%s' "$array_literal" | grep -oE '"[A-Za-z_]+"' | tr -d '"' | option_set)"
	want_assoc="$(printf 'OWNER\nMEMBER\nCOLLABORATOR\nCONTRIBUTOR\n' | option_set)"

	if [ "$got_assoc" = "$want_assoc" ]; then
		pass "allowlist literal names exactly the four trusted associations"
	else
		fail "allowlist parsed as [$got_assoc], expected [$want_assoc] — a maintainer PR could be closed, or an unknown association trusted"
	fi

	if grep -q '!contains(' "$REDIRECT"; then
		pass "the guard negates the allowlist (!contains), so an unknown association falls through to closing"
	else
		fail "the guard no longer negates the allowlist: an inverted or deny-list shape leaves untrusted PRs OPEN"
	fi
fi

# --- S2d: no expression interpolated into a run: block ----------------------
# Indentation is measured on the first NON-WHITESPACE character, so a tab-indented
# workflow is scanned correctly instead of ending the block early (review R3-004).
run_blocks="$(grep -cE '^[[:space:]]*-?[[:space:]]*run:' "$REDIRECT" 2>/dev/null || true)"
run_blocks="${run_blocks:-0}"

if [ "$run_blocks" -ge 1 ]; then
	pass "found $run_blocks run: block(s) to inspect"
else
	fail "no run: block found in the redirect workflow: the injection assertion would pass having inspected nothing"
fi

expr_lines="$(awk '
	/^[[:space:]]*-?[[:space:]]*run:/ {
		match($0, /[^[:space:]]/); run_indent = RSTART; in_run = 1
		if ($0 ~ /\$\{\{/) print NR
		next
	}
	{
		if (in_run) {
			if ($0 ~ /^[[:space:]]*$/) next
			match($0, /[^[:space:]]/)
			if (RSTART <= run_indent) in_run = 0
		}
		if (in_run && $0 ~ /\$\{\{/) print NR
	}
' "$REDIRECT")"

if [ -n "$expr_lines" ]; then
	fail "a \${{ }} expression appears inside a run: block at line(s) $(printf '%s' "$expr_lines" | tr '\n' ' ')— script injection vector on pull_request_target; pass values through env: instead"
else
	pass "no \${{ }} expression inside any run: block"
fi

say "assertions: $((PASS_COUNT + FAIL_COUNT)) — $PASS_COUNT pass, $FAIL_COUNT fail"
[ "$FAIL_COUNT" -eq 0 ] || exit 1
exit 0
