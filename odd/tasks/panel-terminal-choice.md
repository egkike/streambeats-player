# Panel Terminal Choice (issue #90)

Cycle goal: the settings screen (`c`) becomes the single editor for which
terminal hosts the player panel — Ghostty or kitty — with only installed
terminals selectable, a restart notice on every committed change, and the
launcher resolving the choice with an explicit precedence. Kitty rides its
native 0.42+ `kitten quick-access-terminal` (wlr-layer-shell, same requirement
as Ghostty on Wayland).

Specs quote the owner's conversation strings and issue #90 verbatim.
Branch: `feature/panel-terminal-choice`.

## Specs

- S1: "Terminal en el menú c para seleccionar entre ghostty o kitty" — the
  settings screen gains a Terminal section following the theme-selection
  pattern (navigate, `enter` commits, `(current)` marker).
- S2: "con un mensaje que debe reiniciar la app para que haga efecto (siempre
  que seleccione una opción de cambio)" — committing a terminal change shows
  a message that the change applies after restarting (the choice affects the
  next panel spawn, never the live TUI host).
- S3: "que le permita seleccionar solo los terminals que tenga instalados en
  su máquina, el que no tiene no le habilita seleccionar" — detection via
  `exec.LookPath` injected for tests, failing closed (issue-#64 doctrine);
  nothing installed foreign is selectable.
- S4 (issue #90): "new `panelTerminal` field in
  `~/.config/streambeats/settings.json` written by the TUI's settings store
  (existing atomic Save)".
- S5 (issue #90): "Launcher precedence (`scripts/streambeats-quick`, POSIX
  sh): `--terminal=<kitty|ghostty>` flag > `settings.json` `panelTerminal`
  (anchored, narrow read of the machine-written field; malformed/missing file
  falls through) > builtin order (ghostty → kitty → existing friendly
  error)".
- S6 (issue #90): "per-side quick-access presets (`assets/kitty/`) mirroring
  the Ghostty side table (edge top/bottom/left/right + size mapping),
  launcher spawns via `kitten quick-access-terminal`, docs (`docs/kitty.md`)
  covering the KDE global-shortcut bind and the ASCII keycaps note
  (issue #64: kitty renders the ASCII transport, never the calibrated emoji
  caps)".
- S7 (issue #90 negative controls): kitty absent → row not selectable and
  launcher never resolves kitty; neither installed → no foreign option and
  today's friendly error; malformed `settings.json` → builtin order (no
  crash); bad `--terminal` value rejected with usage; settings save failure
  reported in the status line, selection not silently lost.
- S8 (issue #90 invariants): player behavior untouched (idle-active queue
  advance, fresh resolve per loadfile, quit stops the world); the Ghostty
  today-path stays byte-identical when `panelTerminal` is absent or `ghostty`;
  outside ghostty the TUI renders the ASCII transport.

## Tasks

- T1 (S3,S7): RED tests — terminal detection helper with injected lookup:
  ok → row enabled, missing → not selectable (fail closed).
- T2 (S4): RED test — `Settings` gains `panelTerminal`; Save/Load round trip
  and malformed-file fallback behavior.
- T3 (S1,S3): GREEN — Terminal section in the settings screen: availability
  list from the injected detector, `(current)` marker, commit on `enter`.
- T4 (S2): RED→GREEN — restart notice in the status line exactly on a
  committed change (not on opening the screen, not on no-op selection).
- T5 (S5): launcher — `--terminal` flag validation, anchored `panelTerminal`
  read from settings.json, builtin order, kitty spawn path via
  `kitten quick-access-terminal`.
- T6 (S6): kitty per-side presets under `assets/kitty/` + `docs/kitty.md`
  (KDE global shortcut, size mapping, ASCII keycaps note) + cross-links in
  `docs/ghostty.md`/`docs/install.md`.
- T7 (S7,S8): guards — existing ghostty pins pass untouched; pipeline
  (gofmt, build, vet, test) grouped in work-unit commits with the ODD log.

## Log

- L1 (owner, 2026-10-30): research question "otras posibles Terminales (Kitty,
  WezTerm)... Existe esta posibilidad?" → verdict kitty yes / wezterm no.
- L2 (owner, 2026-10-30): installer env-var + `--set-terminal` proposals both
  cut; the settings menu becomes the single editor.
- L3 (owner, 2026-10-30): "Dale avancemos con esto de Terminal en el menú c
  para seleccionar entre ghostty o kitty y con un mensaje que debe reiniciar
  la app para que haga efecto (siempre que seleccione una opción de cambio)"
  — authorized as issue #90 with `status:approved` (standing grant,
  established 2026-10-25, issue #25 working model) and `type:feature`.
- L4 (2026-10-30): cycle implemented RED→GREEN. Unit 1 (e6c7902 + d3534fa
  odd doc): settings store `panelTerminal` (omitempty) + offer-set helper
  (`panelTerminalRows`, canonical order, injected lookup, whitelist)
  with 10 RED-then-GREEN tests. Unit 2 (a5bf19d): combined settings cursor
  (themes → installed terminals), no-op on re-commit, notice via
  `saveMsg.notice` = "X set as panel terminal — applies after restarting
  the app", full-settings persistence (absorbs the latent theme-save
  sibling clobber of libraryRoot — guarded by a dedicated test), empty
  offer renders "no panel terminal installed" (S7). Unit 3 (6a8e1f3):
  launcher resolution flag > settings.json `panelTerminal` (anchored
  grep, fail-through) > builtin ghostty→kitty; wishes verified against
  PATH; kitty branch execs `kitten quick-access-terminal --config
  <preset> <player>`; stub harness 22/22 + desktop 41/41 + install 78/78.
  Session side task: stale gh branches cleaned — `fix/load-window-marker`
  (fully merged) deleted local+stale tracking; `fix/nav-skip-dotfiles`
  had unmerged L5 docs content, preserved on `docs/library-navigator-l5`
  before deletion.
- L5 (2026-10-30): slice boundary at ~859 authored lines (4 commits) —
  this cycle exceeds the ~400-line advisory as a whole; delivery strategy:
  ONE feature PR carrying `size:exception` label (advisory visibility for
  the owner; the unit commits inside keep the review slices), per AGENTS
  lane rules. Remaining: T6 uninstaller assets check, docs commit, native
  review receipt.
- L6 (2026-10-30): RDD native review receipt — lineage review-98c9795b52254860
  (high tier, owner authorized 4 refuted-binding lens runs + 1 refuter run,
  host-relay). Result: ESCALATED closed — refuter corroborated R2/R4
  "ghostty-output-regression" (summary line gained terminal= token and the
  doubled pre-existing 1.3.1 note printfs dropped to one; S8 byte-identity
  violated). Owner decision (2026-10-30): fix + full re-review. Parity fix
  restored the plain summary + doubled notes on the ghostty branch (the
  terminal= summary now prints only on the kitty branch) and scenario 8
  pins the stderr contract as a committed test. New review runs on the
  changed candidate; correction budget of the closed lineage untouched.
- L7 (2026-10-30): re-review receipt — lineage review-58690d33945a47c3 (high
  tier, 4 lens model runs host-relayed, owner-authorized for this
  corrected candidate). OUTCOME: APPROVED; acknowledgement burned
  (gentle-ai.review-acknowledged/v1, consumed_revision
  sha256:0f5b46c6fa5465c40e70aa691bfad995a0a76f1ad9173713d6e85d108f1c7dbf).
  Three non-blocking informational findings recorded as separate follow-up
  work (never re-review material): R2-settings-read-not-anchored (launcher
  settings.json read anchoring), R3-001 (reliability, streambeats-quick:102),
  R4-001 (resilience, streambeats-quick:108). Delivery follows ordinary
  repository policy: PR under the issue-first gate with `Closes #90`.
- L8 (2026-10-30): follow-up cycle #92 (informational review findings R2/R3/R4-001
  from the APPROVED lineage review-58690d33945a47c3) implemented RED→GREEN
  on the same branch: settings_terminal() replaced by a single-pass
  top-level anchored awk scanner (depth + JSON-string aware; value
  whitelisted to ghostty|kitty; escapes NOT decoded — strict compare;
  silent fail-through preserved). RED: nested pretty + nested minified
  fixtures; pins: pretty 2-space machine shape, minified flat,
  nested minified. Harness 38/38, desktop 41/41, install 79/79,
  shellcheck clean, go suite green. Docs: docs/kitty.md precedence table
  + anchor contract note updated. Why not a pure regex: line-anchoring
  breaks minified files and a boundary regex still false-matches
  nested hand-written JSON; depth-by-construction is the only POSIX
  anchoring that holds for both shapes (jq is not a launcher dependency).
- L9 (2026-10-30): delta-cycle re-review receipt — lineage
  review-30823f4c4f4313ab (high tier, 4 lens model runs host-relayed,
  owner-authorized for the #92/#93 delta against base eb07ac9). OUTCOME:
  APPROVED on the last admitted event, zero corrections requested;
  acknowledgement burned (gentle-ai.review-acknowledged/v1, consumed
  revision sha256:10402694521335aa2755cf9b0f78f4b84abd1a7e973700e8a8e).
  The whole PR #91 tree (~1306 lines, 11 commits) is now covered by
  APPROVED receipts: the #90 feature (review-58690d33945a47c3) and this
  #92/#93 delta. Delivery follows ordinary repository policy; owner
  merge decision remains the sole gate.
  Edit and merge point for the ledger: after the owner merges, L12
  records the merge evidence (PR #91, squash, CI 4/4 green) per
  governance step 5.
- L12 (2026-10-30): merge ledger (governance step 5) — PR #91 MERGED by the
  owner as merge commit b240e5650babd87b7a127fdb8b83aa87c76cc135
  ("Merge pull request #91 from egkike/feat/panel-terminal-choice",
  mergedAt 2026-10-30T18:24:43Z). Merge-commit CI green BEFORE any new
  main push: run 41106714736 completed success on b240e56 (Detect Go
  changes / Install macOS / Verify Go code / issue-gate). Issues closed
  by the PR keywords: #90, #92, #93. Feature branch deleted (local +
  remote) after the ledger. Release tagging: the cycle records no
  release-bound flag; a tag remains the owner's explicit decision and
  would require its own green merge-commit CI row (received here).
  Follow-ups riding the merge are done; no open engineering work from
  this cycle remains. Docs-only ledger rides its own PR per the ruleset
  (docs lane, model #25).

- L13 (owner, 2026-10-30): "Cuando actualices los docs de odd también verifica
  el README.md para ver si necesita actualización con lo que agregamos" —
  standing doc-update directive: every later odd/docs wave cross-verifies
  README.md coverage.
- L14 (2026-10-30): README.md read (187 lines) vs cycle reality — four stale
  spots found and updated: (1) quick-start comment "panel (Ghostty)" →
  panel (Ghostty or kitty, choice in the settings screen); (2) the
  Requirements bullet "Ghostty 1.2+ required only for the floating panel"
  → Ghostty 1.2+ OR kitty 0.42+ with the settings-menu choice and the
  --terminal override; (3) the "Floating quick terminal" section: presets
  paragraph now covers both asset families + per-host toggle semantics +
  docs/kitty.md link; (4) Architecture section gains docs/kitty.md and the
  settings choice; the Notes line "independent Ghostty window" became
  "independent panel window (ghostty or kitty)". macOS section untouched:
  kitty-a-macOS parity is NOT claimed as validated (CI validates the
  installer path, not the kitty panel there).
- L15 (2026-10-30): wrap-up ledger (governance step 5) — PR #97 MERGED as
  merge commit 897e11c98f... (README kitty recovery; issue #96 CLOSED by
  the PR keyword), PR #99 MERGED as 696a275 (go.mod promote + tidy gate;
  issue #98 CLOSED). RDD decline receipt recorded: the go.mod delta
  candidate ran through review START and was consent-DECLINED
  candidate-scoped (lineage created=false, zero model runs, receipt
  gentle-ai consent declined_this_candidate) — review-only evidence;
  delivery proceeded under ordinary repository policy with CI 4/4 green
  on both PRs. Post-merge host verification on main: README carries 10
  kitty mentions, `go mod tidy -diff` EMPTY, `go mod verify` all modules
  verified, working branches deleted. The cycle is fully closed; the
  release tag remains the owner's explicit decision (nothing recorded as
  release-bound).
- L16 (2026-10-30): formal disposition of the informational findings from
  the APPROVED lineage review-58690d33945a47c3 (recorded pre-merge; logged
  here by issue #102). R2-settings-read-not-anchored (readability):
  RESOLVED by cycle #92 — the loose grep was replaced by the top-level
  anchored scanner (issues pinned by harness scenarios 9a-9d); the flagged
  read no longer exists on main. R3-001 (reliability) and R4-001
  (resilience), flagged at streambeats-quick:102/108: SUPERSEDED-BY-REWRITE
  — both lines live inside the block rewritten by the same scanner
  (single-pass, value whitelist, silent fail-through), behavior pinned by
  the harness; the full finding texts are not recoverable post-ack, so
  supersession is the honest record (no invented detail). All three stay
  review-only: no code change, no re-open, per the provider disposition.
- L17 (2026-10-30): release evidence (governance step 5) — annotated tag
  v1.9.0 pushed over main tip ec9091e (merge-commit CI green on the three
  landing merges: ec9091e/c8d4a2f/5db101d, all success). The release
  workflow completed success: GitHub Release v1.9.0 published
  2026-10-30T19:17:15Z as Latest, target main, 6 assets (darwin amd64/arm64,
  linux amd64/arm64, windows amd64 tar.gz/zip + checksums.txt). Tag message
  records the panel-terminal cycle receipt cross-refs. Cycle fully closed:
  issues/PRs #90–#103 land CLOSED/MERGED with the release covering every
  landing to date.
- L18 (2026-10-31): owner live report — the restart notice for a committed
  terminal choice vanished almost instantly. Root cause VERIFIED in code:
  the saveMsg handler assigned m.status directly WITHOUT arming
  statusClearTick (model.go:469-477); the next tick/progress reading wiped
  it (window never armed: statusClearTick=0 tick=0 pinned by the RED test).
  Fix: the saveMsg success/error branches route through setStatus (issue
  #40 doctrine; states: notice/error/theme saved). Tests:
  TestSaveNoticeArmsExpiryWindow + TestSaveNoticeSurvivesFeedbackWindow
  (both wipers: tick + progress each tick). Harness: scenario 7a/7b gained
  GOCACHE/GOMODCACHE on the real defaults + tolerant trap cleanup (the
  sandbox HOME was gobbling the go module cache and rm-failing on its
  read-only dirs). Issue #116 (`status:approved`, type:bug).
- L19 (2026-10-31): RDD native review receipt — lineage review-20d1b8f1d4d345db
  (high tier, 2 lens runs host-relayed, owner-authorized: readability order
  0 + reliability order 1; the provider did NOT request a refuter this
  time). OUTCOME: APPROVED with zero corrections; acknowledgement burned
  (gentle-ai.review-acknowledged/v1, consumed_revision
  sha256:afc374c3e48079c5e98da533d402535d05315c02e94a78ecd95a6a44bae6487d).
  Delivery: PR #117 under ordinary repository policy (merge-commit CI
  green 4/4 pre-merge).
- L20 (2026-10-31): release evidence (governance step 5) — annotated tag
  v1.9.2 pushed over main tip 039359b (merge-commit CI success pre-tag:
  release-bound evidence). Release workflow success: GitHub Release v1.9.2
  published 2026-10-31T16:19:57Z as Latest with 6 assets (same composition
  as v1.9.1 — no packaging changes). Archive sanity re-check: downloaded
  the linux_amd64 tarball — 4/4 kitty presets still riding (the packaging
  coverage guard keeps working). The save-feedback fix (PR #117 / issue
  #116) rides this release.
