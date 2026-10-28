# Desktop entry launches the toggle (issue #73, v1.7.2)

Binding Ctrl+Alt+M (or KRunner) to the APP — the natural user move — runs
the .desktop Exec, which still spawns plain `streambeats-panel <side>`:
toggle works in one direction only. Owner live-validated on v1.7.1
(2026-10-28). This cycle changes the generated Exec to
`streambeats-panel toggle <side>` so binding the app works out of the
factory. Branch: `feat/desktop-toggle-exec`.

## Specs

- S1: "cambiar el Exec del template del .desktop a streambeats-panel
  toggle <side>" — default entry + the four side Actions use `toggle`;
  generated file must pass desktop-file-validate.
- S2: the toggle block only runs when the per-side unit is ACTIVE; with
  no unit, `toggle <side>` spawns exactly like plain `<side>` (v1.7.1,
  review-f0ae204a30b17e8b approved). First press spawns, later presses
  show/hide.
- S3: scripts/streambeats-panel logic untouched (S4 of panel-toggle: old
  behavior preserved); plain command keeps spawn/guard semantics.
- S4: install_test pins the generated Exec lines (default + 4 side
  actions contain `toggle`); desktop_integration_test stays 30/30.
- S5: docs that describe the desktop entry match the new behavior.

## Tasks

- T1: branch + this doc + Engram mirror.
- T2 (S4): RED — install_test asserts `toggle` in generated Exec.
- T3 (S1): GREEN — template Exec → toggle (default + actions).
- T4 (S5): docs alignment (README/docs/kde-panel-integration.md).
- T5: full pipeline + native review + PR + release v1.7.2.

## Log

- L1 (owner, 2026-10-28): v1.7.1 live validation — app-bound Ctrl+Alt+M
  toggles only one direction (kglobalshortcutsrc shows
  `[services][streambeats-panel.desktop] _launch=Ctrl+Alt+M`); root cause
  is the Exec, not the toggle code (live kdotool measurement proved
  `toggle right` minimizes/restores). Owner: "Dale al ciclo".
- L2 (2026-10-28): cycle implemented — T2/T3 commit 6f97735 (Exec=toggle
  RED→GREEN, scenario 6e spawn pin, scenario 3 determinism fix, 33/33 ×5
  stable), T4 commit b2acb0d (docs). Native review review-907fc91678570fa4
  (tier HIGH, lenses risk + reliability, committed-only base 9806b50):
  2 lens reviewers admitted, no refuter required, APPROVED with authority
  burned (gentle-ai.review-acknowledged/v1). 2 authorized model runs,
  correction budget 51 unused.
- L3 (2026-10-28): owner live validation AFTER v1.7.2 install — toggle
  still one-directional via the app-bound keybind even with the new
  Exec on disk. Debug chain: instrumented installed script (set -x +
  PPID trace, restored byte-exact after diagnosis) proved the keybind
  launch still ran the OLD Exec (args "right", MODE=spawn) — the stale
  KService lives in the PLASMA SESSION's memory (parent process was
  systemd --user; Plasma 6 launches app shortcuts via transient units),
  and neither kbuildsycoca6 nor a System-Settings re-register refreshed
  it. Session logout/login re-resolved the entry: owner confirmed both
  toggle directions working ("Funciona perfecto"). No code defect in
  v1.7.2; the caching is upstream Plasma behavior worth an advisory in
  docs (users upgrading the desktop entry need a session restart).
