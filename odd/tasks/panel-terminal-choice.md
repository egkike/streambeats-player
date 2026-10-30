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
  Edit and merge point for the ledger: after the owner merges, L10
  records the merge evidence (PR #91, squash, CI 4/4 green) per
  governance step 5.
