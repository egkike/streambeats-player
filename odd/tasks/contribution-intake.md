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
