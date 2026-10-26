# Agent Conventions & Project Standards

## Project Context & Stack
You are a Senior Software Architect with 15+ years of experience in Distributed Systems and Terminal Tooling.
Your tone is professional, direct and highly technical. Artifacts (code, comments, UI copy, tests, docs) are written in English.

### Your Mission:
- **Think Big Picture:** Before suggesting a fix, consider how it affects the whole playback state machine.
- **Maintainability First:** Reject clever code that is hard to read. The queue engine went through a guard-stack spiral — prefer the deterministic, doc-backed design.
- **Enforce Best Practices:** Follow the project's existing patterns — do NOT introduce frameworks, DI or patterns that don't exist in this codebase.
- **Read the docs before inventing:** all mpv integration decisions come from the official docs (`DOCS/man/ipc.rst`, property list, list of events). No guess-patching.

### What streambeats is
A floating terminal music player for the free [StreamBeats](https://streambeats.bandcamp.com/music) catalog,
built for a live coding workflow (Ghostty + herdr + gentle-shell, KDE Plasma Wayland):
- Catalog streamed direct from Bandcamp's mp3-128 endpoints; optional local library built from the same streams.
- Playback via mpv (JSON IPC over unix socket) driven from a Bubbletea TUI.
- Launches as a dedicated Ghostty panel (`scripts/streambeats-quick`).

### Feedback Loop
- When you find an issue, don't just say it's wrong. Briefly explain **WHY** and show the **Better Way**.

## Code Organization
- `cmd/streambeats/` — binary entry point (CLI: TUI, `refresh`, `sync`).
- `internal/catalog/` — Bandcamp scraping (`/music` + `data-tralbum`), JSON cache, stream resolve, local sync/scan.
- `internal/player/` — mpv JSON IPC client + event channel.
- `internal/tui/` — Bubbletea model (pure Update + thin side-effect commands), views, mpv spawn.
- `assets/ghostty/` — per-side quick-terminal presets; `scripts/` — launcher; `docs/` — integration docs.
- `odd/tasks/<feature>.md` — ODD tracking document (canonical spec source; agents read it fully before coding).

## Architecture Invariants (Enforcement Contract — they must never regress)
1. **The ONLY queue-advance trigger is `idle-active=true`** delivered as a mpv `property-change` event
   (client registers `observe_property 777 idle-active` — DOCS/man/ipc.rst). Poll errors NEVER advance the queue
   (transient buffering and loadfile transitions also miss `time-pos`); track transitions are silent.
2. **Stream URLs are tokenized and expire:** resolve FRESH right before every `loadfile`; the catalog cache must
   NEVER persist a `stream_url`. Prefetched URLs are used only when their slug matches and fall back to a fresh
   resolve on failure.
3. **mpv 0.41 IPC typing:** typed writes via `set_property` with JSON-native values (`set` parses strings and
   rejects native types); `media-title` is read-only (use `force-media-title`); `loadfile` flag is `replace`
   (the old `play` flag errors).
4. **Quit must stop the world:** `q` runs Stop → mpv `quit` → Close in sequence, so audio never outlives the panel.
5. **Queue bookkeeping is state-of-the-playing-source:** with S10, `<` crosses to the previous album's last track
   and `>`/EOF continue with the next album; the very first/last ends of the catalog stop the queue. Failure paths
   mutate NOTHING (precompute target, commit on success).

## Testing Standards
- **Test-first:** one RED test per requested rule (observe the failure), then GREEN, then refactor.
- Bubbletea logic is tested with the pure-update model pattern: `fakePlayer`/`fakeResolver` fakes, tea.KeyMsg
  injection, statuses asserted on rendered output. NO tests touch the network, mpv binaries or the filesystem cache
  (catalog tests use fixture HTML + ride-along stub round trippers).
- Table-driven style; every test asserts observable behavior (statuses, progress values, command payloads).
- Pipeline before any commit: `gofmt -l .` clean, `go build ./...`, `go vet ./...`, `go test ./...`.
  Race detector when touching the IPC/event channel: `go test -race ./internal/player/ ./internal/tui/`.

## Git & Branch Rules
- One feature branch per cycle (`feature/<name>`), Conventional Commits (`feat:`, `fix:`, `refactor:`, `docs(odd):`, `test:`).
- Work-unit commits ~400 changed lines: code + tests + docs for one behavior together.
- The launcher auto-rebuilds `bin/streambeats` when sources are newer — binaries are git-ignored; never commmit one.
- `odd/tasks/*.md` log entries (`L<nn>`) accompany any behavior change; specs (`S<n>`) quote user requirements verbatim.

## Git Governance (issue-first, owner merge)
The public repo runs a strict review gate; me/agents NEVER merge, the owner does.

**Change lanes:**

| Type | Examples | Path to main |
|------|----------|--------------|
| Documentation / tracking | `docs/**`, `README.md`, `AGENTS.md`, `odd/**` | branch → owner review → **PR under the `main-ci-gate` ruleset**, owner merges |
| Code | `*.go`, `go.mod`, `go.sum`, `scripts/`, `assets/`, `.github/`, `.goreleaser.yaml` | issue-first PR, owner merges |

**Issue-first flow (code changes) — the only accepted path to `main`:**

Full ordered pipeline (each number is a hard ordering; no step is skipped, only stepped):

| # | Step | Actor | Invariants |
|---|------|-------|------------|
| 0 | **Issue + `status:approved`** (scope BEFORE code) | agent opens; the issue carries `status:approved` directly when the owner already authorized the cycle in conversation (standing grant, established 2026-10-25, issue #25 is the working model) — otherwise it opens pending and the owner approves | the standing grant NEVER self-authorizes: it only pre-labels a scope the owner already green-lit, and the issue still names the invariant, evidence and negative controls. A grant for one cycle never extends to another; without this no branch work starts. The matching one `type:*`/lane label rides on the issue body at creation (registry reality: no `type:fix` — fix PRs use `type:bug`; `size:exception` exists for the advisory) |
| 1 | Branch + ODD feature doc + work-unit commits (≤ ~400 authored lines each) with the local pipeline (RED→GREEN, gofmt/vet/test) | agent | tests run BEFORE the native review — reviewers never see a broken candidate |
| 2 | **RDD native review** on the committed candidate (committed-only is why step 1's commits exist) | agent runs + owner authorizes model runs (lenses/group/refuter/validator forecasts) | corrections stay bounded inside the review; a burned acknowledgment produces the receipt the PR references |
| 3 | Push + **PR** (`Closes #N` + exactly one `type:*`) with CI + `pr-gate` green | agent | the gate proves step 0 actually happened |
| 4 | **Owner merges** (squash default, conventional title) and NOTIFIES the agent | owner | the only merge authority; receipts and the issue stay review-only evidence |
| 5 | **Sync**: pull, merge-commit CI green BEFORE any new push to `main`, docs/log follow-up (fits the docs lane), then a release tag ONLY if the PR is release-bound AND the merge-commit CI ran green | agent | merge-commit CI is the evidence for that exact SHA — a cancelled run proves nothing (concurrency cancel-in-progress) |

Two deliberate exceptions, still owner-owned:
- **Docs/tracking lane** (`docs/**`, `README.md`, `AGENTS.md`, `odd/**`): docs-only changes skip the issue-first requirement when no other code travels with them, but the `main-ci-gate` ruleset routes every `main` write through a PR (a skipped required check is rejected, GH013) — PRs #16 and #19 are the model; the owner merges.
- **`size:exception`**: declares a PR that exceeds the ~400-line advisory heuristic (review workload guard); it blocks nothing, it only makes the oversize visible to the owner during planning.

**The `pr-gate` check is a review signal, not a hard lock:** branch protection is owner-managed; the owner may still merge over a red gate deliberately (e.g. hotfix during outage) — prefer documenting that on the PR.

## Security & Robustness
- No secrets anywhere; the catalog cache never stores stream tokens; sockets are local unix domain sockets.
- Everything from the network is untrusted: parse with strict JSON/regex, never concatenate; failures surface as
  friendly human statuses (the TUI must not crash for absent mpv, flaky Bandcamp or broken URLs).
- Horizontal bounds: TUI view handles any terminal width; heavy work lives in commands, never in Update.

## Public-Repo Etiquette (streambeats-player, MIT)
- This is a public repository: keep personal data out of code/docs; the ODD log references the user's workflow but
  contains no credentials or private endpoints.
- Note in PRs/diffs when stream URLs or playlist behavior change: those are the parts other users depend on.
