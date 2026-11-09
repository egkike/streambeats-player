#!/bin/bash
# policy_surface_test.sh — contribution surface invariants (issue #188).
#
# Contract: the contribution surface lives in files no Go test can reach, and two
# of its invariants degrade SILENTLY. Both are pinned here:
#
#   S1 (review R2-001) the `area` dropdown is duplicated across the two issue
#      forms: GitHub issue forms cannot share an option list across files (YAML
#      anchors do not cross file boundaries), so the duplication itself is
#      structural and the DRIFT is what has to be caught.
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

bug_opts="$(area_options "$BUG_FORM")"
feature_opts="$(area_options "$FEATURE_FORM")"

if [ -z "$bug_opts" ] || [ -z "$feature_opts" ]; then
	fail "could not extract the area options from both forms — the extraction anchor moved (expected 'id: area' followed by 'validations:')"
else
	bug_count="$(printf '%s\n' "$bug_opts" | wc -l | tr -d ' ')"
	feature_count="$(printf '%s\n' "$feature_opts" | wc -l | tr -d ' ')"

	if [ "$bug_count" -ne "$feature_count" ]; then
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

if grep -qE '^[[:space:]]*types:[[:space:]]*\[opened, reopened\]' "$REDIRECT"; then
	pass "trigger types are [opened, reopened]"
else
	fail "trigger types are not [opened, reopened]: a PR opened by an outside author may never be redirected"
fi

# --- S2b: never execute contributor code ------------------------------------
if grep -q 'actions/checkout' "$REDIRECT"; then
	fail "an actions/checkout step is present: on pull_request_target this would check out and run contributor code with a write token"
else
	pass "no actions/checkout — contributor code is never checked out"
fi

# --- S2c: the guard is an allowlist, not a deny-list ------------------------
# An enumerated deny-list fails OPEN: an association GitHub adds later would
# leave that PR open by omission.
if grep -q '!contains(' "$REDIRECT" && grep -q 'fromJSON(' "$REDIRECT" && grep -q 'author_association' "$REDIRECT"; then
	missing=""
	for assoc in OWNER MEMBER COLLABORATOR CONTRIBUTOR; do
		grep -q "\"$assoc\"" "$REDIRECT" || missing="$missing $assoc"
	done
	if [ -n "$missing" ]; then
		fail "trusted associations missing from the allowlist:$missing — a maintainer PR could be closed by this job"
	else
		pass "guard is a fail-closed allowlist over author_association (OWNER MEMBER COLLABORATOR CONTRIBUTOR)"
	fi
else
	fail "guard is not the fail-closed allowlist: !contains/fromJSON/author_association missing — it may have been simplified into a deny-list"
fi

# --- S2d: no expression interpolated into a run: block ----------------------
expr_lines="$(awk '
	/^[[:space:]]*-?[[:space:]]*run:/ {
		match($0, /[^ ]/); run_indent = RSTART; in_run = 1
		if ($0 ~ /\$\{\{/) print NR
		next
	}
	{
		if (in_run) {
			if ($0 ~ /^[[:space:]]*$/) next
			match($0, /[^ ]/)
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
