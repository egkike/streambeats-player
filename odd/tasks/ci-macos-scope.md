# CI macOS Scope Filter (issue #66)

The `Install macOS` smoke is the only native darwin gate but every Go-code PR
pays the macOS hosted-runner queue for it, and on 2026-10-27 the queue starved
three times (one attempt cancelled after 15m with zero steps). This cycle adds
a cheap GOOS=darwin cross-build to the Linux pipeline and narrows the native
smoke to installer-relevant changes.

Specs quote issue #66 verbatim. Branch: `feat/ci-macos-scope`.

## Specs

- S1: "add cross-build steps to the Linux `ci` job — `GOOS=darwin GOARCH=amd64
  go build ./...` and `GOOS=darwin GOARCH=arm64 go build ./...` — so a
  darwin-only compilation break surfaces in seconds on Linux, with zero
  runner-queue exposure."
- S2: "add a second `dorny/paths-filter` output (`installer`) matching
  `scripts/**`, `go.mod`, `go.sum`, `.github/workflows/ci.yml`;
  `install-macos` runs `if: needs.changes.outputs.installer == 'true'`."
- S3: "The `ci` workflow still runs on every PR/push and always reports its
  required checks (the #16 doctrine: a skipped required check fails the
  `main-ci-gate` ruleset, GH013). Path filtering stays at the job level."
- S4: "`install-macos` keeps its full smoke unchanged; only its trigger
  narrows."
- S5: "Merge-commit CI on `main` remains uncancellable release evidence."
- S6 (negative control): "A docs-only change still satisfies both required
  checks without running any Go or macOS step."
- S7 (negative control): "A Go-only change (e.g. `internal/tui/**`) runs the
  Linux pipeline + cross-builds but skips `install-macos` — and the run still
  reports green required checks."
- S8 (negative control): "A change to `scripts/install.sh` runs the full macOS
  smoke natively."

## Tasks

- T1 (S1): cross-build steps in the Linux `ci` job.
- T2 (S2): `installer` filter output + narrowed `install-macos` condition.
- T3 (S3-S5): workflow-level review — always-reported checks, smoke content
  unchanged, main-run concurrency untouched.
- T4: `actionlint` + push + live filter verification on the PR (the PR
  itself exercises S6-S8 against its own diff) + merge-commit CI green.

## Log

- L1 (owner, 2026-10-27): authorized the cycle ("Vamos con el 2") after the
  macOS runner queue starved three times in the PR #65 cycle; issue #66
  opened with `status:approved` + `type:chore` per the standing grant.
- L2 (2026-10-27): implemented. Cross-builds darwin amd64+arm64 added to the
  Linux `ci` job after Build (S1); `installer` filter output declared on the
  `changes` job (actionlint caught the missing output declaration in the
  first draft) and `install-macos` now triggers on
  `needs.changes.outputs.installer == 'true'` (S2) — smoke content untouched
  (S4). actionlint clean. Commit 58b67c5.
- L3 (2026-10-27): RDD native review receipt — lineage
  review-1db4e7449447e64a (HIGH tier, 2 model runs owner-authorized: risk
  order 0 + reliability order 1), outcome APPROVED with zero corrections;
  authority burned (gentle-ai.review-acknowledged/v1, consumed_revision
  sha256:1760cd63…). Delivery follows ordinary repository policy.
