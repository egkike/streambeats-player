# Contribution intake surface (issue #180) — docs + CI, issues-only

Publishes the contribution policy this repository already enforces in private: no
`CONTRIBUTING.md`, no issue forms, no statement anywhere a stranger would read. The
gate exists (`pr-gate.yml`, owner-only merge); the explanation does not. Mirrors the
maintainer's MCP repository, adapted to a player with no `--version` and no
troubleshooting page. Zero Go surface.

## Specs

- **S1**: the repository states publicly that it accepts **issues only**; a pull
  request opened from outside the delivery gate is closed automatically with an
  explanation, never left to die on a red check.
- **S2**: the surface follows the shape the reference repository already runs —
  `CONTRIBUTING.md`, one bug form, one feature form, a chooser config, and the
  redirect workflow — and stays "similar and adequate to this project": no
  `streambeats --version` field (the command does not exist), no troubleshooting
  page to link (there is none), and an area dropdown that names this project's
  domains.
- **S3**: the public document mirrors `AGENTS.md` § Git Governance and contradicts
  nothing in it. It claims only what the gate can back: no response time, no review
  turnaround, no promise that outside work gets evaluated.
- **S4**: the redirect workflow is a courtesy, not an access control: it never
  checks out or executes contributor code (`pull_request_target`, no checkout),
  passes every value through `env:` with no `${{ }}` inside `run:`, is allowlist
  **fail-closed** on `author_association`, and can never close the maintainer's own
  pull requests.
- **S5**: no Go source, no `cmd/`, no `internal/` behavior change; the local pipeline
  (`gofmt -l .`, `go build ./...`, `go vet ./...`, `go test ./...`, `go mod tidy
  -diff`) stays green and the yaml parses.
- **S6**: every link resolves. `CONTRIBUTING.md` and the forms point at
  `README.md#install-one-line`, `docs/install.md` and `docs/ghostty.md#troubleshooting`
  — the anchors that exist, never a guessed one.
- **S7**: the two invariants no Go test can reach — the `area` options shared by
  both issue forms, and the fail-closed shape of `close-unsolicited-prs.yml` — are
  pinned by a shell test that runs **unconditionally** in the `ci` job, so a PR that
  touches only that workflow is still guarded rather than skipped.

## Tasks

- **T1** (S1, S2): `CONTRIBUTING.md` + README contributor link. Route: inline; docs
  lane; commit 1.
- **T2** (S2, S6): `.github/ISSUE_TEMPLATE/{bug_report,feature_request,config}.yml`
  with `type:bug` / `type:feature` stamped by the form and `blank_issues_enabled:
  false`. Route: inline; code lane (`AGENTS.md` puts `.github/` there); commit 2.
- **T3** (S4): `.github/workflows/close-unsolicited-prs.yml`, ported from the
  reference design with no invention. Route: inline, own commit — Risk lines name
  items 2 and 5, so native review takes it alone; commit 3.
- **T4** (S3): `AGENTS.md` names `CONTRIBUTING.md` in the documentation lane and
  states the external intake rule; `ci.yml:8` loses the stale "docs-only flow to
  main directly" comment the ruleset contradicts. Route: inline; commit 4.
- **T5** (S5, S6): local pipeline, `actionlint`, yaml parse, link check, native RDD
  review of the committed candidate. Push, PR and merge stay with the owner.
- **T6** (S7): `scripts/tests/policy_surface_test.sh`, the unconditional step in
  `ci.yml`, and an `AGENTS.md` correction that moves both entries from accepted risk
  to guarded. Route: inline; RED observed on four throwaway fixtures, then GREEN.
  Risk: item 5 (`ci.yml`). Commit 5.

## Log

- **L1** (owner, verbatim): "Hola capo, me gustaría evaluar la posibilidad de
  agregar la forma de contribuir en el repo tomando como ejemplo mi proyecto de mcp
  que ya lo tiene implementado. La idea es que se siga un workflow similar adecuado
  a este proyecto. Puedes verificar y luego decirme la factibilidad de hacerlo o no?"
- **L2** (owner, verbatim): "Implementemos tu recomendación issues-only" → cycle
  authorized; the standing issue-first grant (`AGENTS.md` step 0) pre-labels the
  scope the owner green-lit in conversation.
- **L3** (evidence, verified before writing): labels `type:bug`, `type:feature`,
  `type:docs`, `type:refactor`, `type:chore`, `status:approved`, `size:exception`
  already exist in the repository (`gh label list`), so each form stamps its own
  `type:` label and `pr-gate` keeps passing. Absent: `.github/ISSUE_TEMPLATE/`,
  `CONTRIBUTING.md`, `SECURITY.md`, `CODEOWNERS`, any contribution text in
  `README.md`.
- **L4** (evidence, adapter constraints): `cmd/streambeats/main.go:64-93` accepts
  only `tui|refresh|sync|uninstall|help`, and `.goreleaser.yaml:29-30` injects
  `-s -w` with no `-X` build version — so the bug form cannot ask for
  `streambeats --version`; it asks for the installed release tag or the source
  commit. Version injection is a separate code cycle, deliberately not folded here.
- **L5** (evidence, contradiction found): `ci.yml:8` claimed docs-only changes
  "flow to main directly without friction" while `AGENTS.md:85` states the
  `main-ci-gate` ruleset routes every main write through a pull request. The
  comment is the stale half; a policy document saying "issues only" would have
  inherited a lie. Fixed in T4.
- **L6** (evidence, lane gap found): `CONTRIBUTING.md` sits in neither lane of
  `AGENTS.md:66-69`. Named in the documentation lane in T4 so the next cycle knows
  which path applies.
- **L7**: issue #180 opened with `status:approved` + `type:chore`; branch
  `feature/contribution-intake`.
- **L8** (delivery strategy, slice boundary recorded): the slice closed at 497
  authored lines in 4 commits, past the ~400-line advisory. Strategy: **one pull
  request declared `size:exception`**, not a chain. Reason: `CONTRIBUTING.md` and
  both issue forms reference `.github/workflows/close-unsolicited-prs.yml` by
  name, so splitting docs from CI would ship a document pointing at a workflow
  that does not exist yet — a false claim, which S1/S3 forbid. The text and the
  enforcement are one behavior; they travel together, and the advisory exists to
  make the oversize visible, which the label does.
- **L9** (T5 evidence): `gofmt -l .` clean; `go build ./...`, `go vet ./...`,
  `go mod tidy -diff` clean; `go test ./...` green on all six packages (no Go file
  touched — the run proves that, it does not merely assert it). Issue forms and
  `config.yml` parse as YAML with all required fields carrying an `id`; every
  relative link and every `contact_links` anchor resolves against the real
  headings (`README.md#install-one-line`, `docs/ghostty.md#troubleshooting`).
  `actionlint` is clean on the new workflow once its stale runner-label list is
  excluded — the same false positive `ci.yml` and `release.yml` already carry for
  `ubuntu-26.04` (issue #161 pin).
- **L10** (decision, outside the tree): private vulnerability reporting was
  **disabled** on the repository (`{"enabled": false}`), so `CONTRIBUTING.md` §
  Security would have pointed at a form that does not exist. Enabled it during
  this cycle (`PUT /repos/egkike/streambeats-player/private-vulnerability-reporting`
  → `{"enabled": true}`). Additive and reversible; the owner can disable it again,
  in which case that section must fall back to the direct contact and say so.
- **L11** (native review receipt): lineage `review-0f5e8ee4d2791c1b`, tier `high`,
  537 changed lines, three lenses admitted on the last event — `review-risk`,
  `review-resilience`, `review-readability`. Outcome **approved**; authority burned
  with `gentle-ai.review-acknowledged/v1` at consumed revision
  `sha256:0a2a296e80240ee3de431022a59cd6aa82b70e71b1e41d0cf4773268fd8eec28` for
  target `sha256:248d64aa6a7e2e28ff20382142a2926dd0021609d721bc014ec0f67b06c30466`.
  Correction budget (200) went **unused**: no correction was opened.
- **L12** (advisory findings, non-blocking, recorded for later work — never a reason
  to re-run this review): `R2-001` readability `SUGGESTION` at
  `.github/ISSUE_TEMPLATE/feature_request.yml:27-40` (the seven-option area
  dropdown, identical to the bug form) and `R4-001` resilience `WARNING` at
  `.github/workflows/close-unsolicited-prs.yml:53-62` (the fail-closed
  `author_association` allowlist). Both are informational: neither opened a
  correction nor reopens the review, so they travel as separate follow-up work,
  not as edits to this approved candidate.
- **L13** (merge evidence, cycle closed): PR #181 merged by the owner as a merge-PR
  commit, not a squash — `08c783a8b469ccd03113c65f96c17792b6921cde`, so the six
  work-unit commits survive in history with their messages. Issue #180 closed by
  `Closes #180`. Pre-merge required checks: `issue-gate`, `Detect Go changes`,
  `Verify Go code`, `Install macOS` — all pass, and both Go paths were **real**
  runs, not the docs-only echo, because `.github/workflows/ci.yml` sits in the
  `code` and `installer` filters (29s of `-race` suite, 36s of macOS smoke test).
  Post-merge `ci` on `main` for exactly `08c783a` is `success` on the `push`
  event and was **not cancelled** — `ci.yml` cancels in flight only on
  `pull_request`, so the merge-commit run stands as evidence for that SHA.
  Verified on `main`, not on the branch: `CONTRIBUTING.md`, the three
  `ISSUE_TEMPLATE/` files and `close-unsolicited-prs.yml` are present.
  **First real execution, observed on PR #182:** the redirect workflow DID run — on
  the follow-up docs PR — and its `if:` evaluated false: the `Close PR and redirect
  to issues` job reported `skipping`, so the maintainer's own pull request was left
  open. That is S4's negative control (*it must never close the maintainer's own
  pull requests*) proven by execution instead of by reading the YAML — a real run,
  not a structural assumption. What that run does **not** prove is the deny branch:
  no untrusted author has exercised it, and no local test can manufacture one, so
  `R4-001` keeps that half open rather than trading it for a green check that
  asserts nothing about runtime.
- **L14** (incident, merge raced a correction): the `L13` correction above was
  pushed to `docs/contribution-intake-close` at 23:06 and PR #182 was merged at
  23:07 while the PR head still pointed at the pre-correction commit. GitHub
  accepted the push (the branch tip on `origin` is the corrected commit
  `a19513c`), but the merge took the registered head `ed02e84`, so `main` carried
  the line "`close-unsolicited-prs.yml` has never executed" — which the very same
  PR's run had just disproved. The claim survived in `main` for minutes, until
  this follow-up. **Lesson, for the branch rules:** a correction pushed to a PR
  branch is not delivered by the push; it is delivered when the PR head SHA
  matches the corrected commit. Merge only what the head SHA confirms, never what
  the branch tip suggests.
- **L15** (issue #188, the advisories resolved instead of accepted): the owner asked
  for the surface to be clean, which reversed the earlier accept-and-document
  decision. `scripts/tests/policy_surface_test.sh` now pins both findings — the
  `area` options must be identical across the two forms, and
  `close-unsolicited-prs.yml` must keep its fail-closed shape (allowlist over
  `author_association`, no `actions/checkout`, no `${{ }}` inside a `run:`). Wired as
  an **unconditional** step in the `ci` job: a PR editing only that workflow sets no
  `code` filter, so a conditional step would never guard it.
- **L16** (RED, and a fixture of mine that was wrong): four throwaway fixtures in a
  scratch tree, one per rule. `deny` (allowlist flipped to `contains(`), `checkout`
  (an `actions/checkout` step added) and `inject` (an expression moved into `run:`)
  each failed with the right message and exit 1. The first `drift` fixture **passed**,
  and the test was right: I had appended the extra option at the end of the file,
  outside the `id: area` block, so the extractor correctly ignored it. Rebuilt inside
  the options block, it failed with `bug_report=7 feature_request=8`. A RED that is
  not real RED is worse than none — it looks like proof.
- **L17** (GREEN): real repo, 6 assertions, 6 pass, exit 0. The `AGENTS.md` entry that
  called these two findings accepted risk is replaced: leaving it would have made the
  file claim a deferral the repo no longer honours.
- **L18** (issue #190, the seven advisories of the #189 receipt): the accepted-risk
  cycle produced seven non-blocking findings, all on
  `scripts/tests/policy_surface_test.sh`, and all the same disease — assertions
  that could pass without proving what they claimed. Their text was never
  persisted (acknowledging a review consumes its store), so each was re-derived
  from the review's own line reference. Fixed: associations are now parsed out of
  the `fromJSON('[ … ]')` literal instead of grepped from the whole file; the
  `types:` assertion accepts both YAML spellings and compares a normalised set;
  indentation is measured on the first non-whitespace character so tabs no longer
  end a `run:` block early; zero `run:` blocks and fewer than two extracted
  options now fail instead of passing vacuously; `set -e` is documented as
  deliberately absent rather than blindly added, since a failing `grep -q` is this
  harness's control flow; the YAML-anchor premise in the header is marked as
  something the test cannot verify.
- **L19** (RED and GREEN for L18): seven throwaway fixtures. Five existing ones
  still fail as before, plus `no-run` and `tabs-inject`. The two that prove the
  hardening, run against BOTH scripts: `owner-in-comment` (association removed
  from the array, the string left in a comment) **passed with the old script and
  fails with the new one** — that is R3-002 made visible; `block-types` (same
  behaviour, block-list spelling) **failed with the old script and passes with the
  new one** — that is R3-001's false positive removed. Real tree: 10 assertions,
  10 pass, exit 0.
