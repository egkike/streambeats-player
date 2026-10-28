# Panel Toggle Show/Hide (issue #69-day follow-up, new issue)

Ctrl+Alt+M opens the panel, but an already-running MINIMIZED panel is not
brought back: the guard's try_focus was a no-op because kdotool was absent.
With kdotool 0.3.0 installed and the raise+restore LIVE-VALIDATED on the real
desktop (2026-10-28), this cycle adds the real toggle to the installed
launcher plus the install-time advisory so new installs know about it.

Specs quote the design agreed in conversation 2026-10-28. Branch:
`feat/panel-toggle`.

## Specs

- S1: "visible → minimize (audio vivo), oculta/minimizada → activate
  (restaura), no existe → spawn" — the toggle decision logic.
- S2: the decision uses ONLY `kdotool getactivewindow` (kdotool 0.3.0 has no
  getwindowminimized query; kwinscript print() does not reach stdout):
  - active window IS the panel window → visible & focused → `windowminimize`
  - otherwise → `windowactivate` (restores if minimized, raises if
    visible-but-unfocused) — two-press toggle doctrine, documented.
- S3: "una instalación nueva queda así — todo funciona de fábrica; si el
  usuario tiene ghostty y quiere el toggle/raise, el installer le dice
  exactamente qué falta y cómo instalarlo" — the post-install advisory:
  Linux + ghostty found + kdotool missing → friendly warn with the two
  install paths (distro package; upstream prebuilt binary).
- S4: old behavior preserved: plain `streambeats-panel [side]` unchanged;
  guard still blocks duplicate spawn; no kdotool anywhere is never fatal.
- S5: macOS keeps skipping freedesktop integration entirely (no advisory).

## Tasks

- T1: branch + this doc + Engram mirror.
- T2 (S1,S2): RED — kdotool stub in STUB_BIN + toggle scenarios in
  desktop_integration_test.sh (activate-when-unfocused, minimize-when-active,
  no-window note, plain path parity).
- T3 (S1,S2,S4): GREEN — `streambeats-panel toggle [side]` mode.
- T4 (S3,S5): installer advisory + install_test scenario.
- T5 (S3): docs — README, docs/install.md, assets/kwin/window-rules.md.
- T6: full local pipeline + RDD native review + PR.

## Log

- L1 (owner, 2026-10-28): reported the minimized-panel raise gap (point 1);
  diagnosis confirmed try_focus no-op; kdotool installed user-side; raise
  test passed on the real desktop. Owner: "Funciona perfecto" → design for
  toggle + advisory + docs presented → "Me parece bien el diseño. Dale al
  ciclo."
- L2 (2026-10-28, new session): resumed from obs #1599 and found the
  previous session's scenario-6 GREEN was VACUOUS — harness defects, not
  product defects: (a) 6a/6b/6c never set SYSTEMCTL_STUB_ACTIVE=right, so
  the stub systemctl reported the unit inactive and the panel took the
  silent spawn path (kdotool never invoked); (b) the negative assertions
  used the `: else fail` idiom inside the then-branch — `: else fail "..."`
  parses as one command (`:` with arguments), so there was no else clause
  and failed greps fired nothing. Fixed the harness: notify-send stub in
  STUB_BIN + NOTIFY_LOG (message assertions now hermetic, never leak to the
  real desktop), SYSTEMCTL_STUB_ACTIVE=right in 6a/6b/6c, non-vacuous
  assertion idiom, and positive assertions for activate/minimize/minimize-
  notify. RED observed against the base panel (7 fails: usage exit 2, empty
  logs), GREEN 28/28 with the implemented toggle. The scenario-3 regression
  reported in obs #1599 does NOT reproduce — artifact of the degraded
  session; fallback assertions pass unchanged.
- L3 (2026-10-28): cycle implemented on feat/panel-toggle — T2/T3 commit
  e8571df (toggle mode + hermetic scenario 6, 28/28), T4 commit 4707721
  (installer advisory + scenario 11, 78/78), T5 docs commit (README
  requirements + panel toggle, docs/install.md dependency table,
  window-rules.md keybind toggle, kde-panel-integration.md toggle section
  rewritten from "deliberately NOT here" to shipped). Issue #70 opened
  with status:approved + type:feature per the standing grant. Full local
  pipeline green: gofmt/go build/go vet/go test, shellcheck, macos smoke
  19/19.
