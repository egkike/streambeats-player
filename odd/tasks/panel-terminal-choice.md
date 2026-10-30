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
