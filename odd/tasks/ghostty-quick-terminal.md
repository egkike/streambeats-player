# Ghostty quick terminal (1.4.0 kitty parity)

Cycle issue #134 (`status:approved` standing grant). Host parity: the ghostty
panel rides the terminal's native quick terminal with toggle-by-rerun, exactly
like the kitty host since v1.9.0. Ghostty binary under test on the desktop:
1.4.0, KDE Plasma Wayland.

## Specs

- S1: "ya tenemos actualizada la nueva versión de ghostty" — the project runs on
  the freshly installed Ghostty 1.4.0 (verified locally: `ghostty --version`
  1.4.0 stable, GTK runtime).
- S2: "que funcione como kitty" — the ghostty panel gets the kitty host's
  launch model: the same command opens the panel on the first press and toggles
  show/hide on every later press, riding the terminal's native quick terminal
  (edge-anchored layer shell), no kdotool / unit guard in that path; the 1.3.1
  dedicated-window flow remains as the fallback.
- S3: owner decisions recorded this session: (a) cycle T1–T4 authorized in
  conversation (standing grant); (b) the desktop mechanism proof (spawn +
  D-Bus claim + gdbus `toggle-quick-terminal`) was visually confirmed on the
  owner's desktop; (c) a desktop cleanup round proved the rule: panel instances
  are NEVER killed while showing — panels close by toggle (hide) or natural app
  exit (`quit-after-last-window-closed = true`, the 1.4.0 default).

## Tasks

- T1 `scripts/streambeats-quick` ghostty v2 branch: per-run confdir config gains
  per-side `class = org.streambeats.panel.<side>` + `command = <wrapper>`;
  wrapper resets `XDG_CONFIG_HOME` for the player child (issue #80 contract);
  spawn-if-not-claimed → toggles via gdbus `org.gtk.Actions.Activate
  toggle-quick-terminal`; falls back to the dedicated-window flow when the
  claim/toggle fails or ghostty lacks the action. Commit: logged in L5.
- T2 `scripts/streambeats-panel` ghostty relay mirroring `kitty_host` (issue
  #108 S2): every press relays the launcher detached before the unit guard.
  Commit: TBD.
- T3 presets: valid dotted per-side `class`; KWin window-rule matching
  re-checked (`assets/kwin/window-rules.md`); stale 1.3.1-only comments
  removed. Commit: TBD.
- T4 docs: `docs/ghostty.md` rewritten for 1.4.0 (limitation §2 resolved; CLI
  `--class` defect documented as upstream status), KDE integration docs,
  `scripts/tests` coverage. Commit: TBD.
- T5 (outside the repo): upstream issue to ghostty-org/ghostty —
  `+toggle-quick-terminal` parses `Options.class` but `run()` ignores it
  (hardcoded `.detect` target), so `--class` cannot address a custom instance.
  Commit: n/a (separate repository).

## Log

- L1: "Hola capo, ya tenemos actualizada la nueva versión de ghostty así que
  puedes verificar y ver que debemos hacer para actualizar el proyecto y que
  funcione como kitty" (verbatim request).
- L2: runtime verification on Ghostty 1.4.0 established: (a) presets validate
  clean on 1.4.0; (b) `+toggle-quick-terminal --class=X` documented but not
  wired (all toggles reach the default bus owner — toggled the owner's own
  default singleton by accident); (c) dedicated instance with explicit
  `--gtk-single-instance=true` + dotted `class` claims its own bus name and the
  gdbus toggle shows/hides the panel (owner visually confirmed); (d) the
  `detect` default does not claim any bus name.
- L3: owner reported ~3 orphan layer-shell windows after test instances were
  SIGTERM-killed mid-display; owner closed them manually (KWin reconfigure GC
  did not suffice). Rule recorded in S3(c): never kill a showing panel; close by
  toggle or natural exit.
- L4: owner authorized the full cycle (T1–T5) after the diagnostic summary;
  issue #134 opened with `type:feature status:approved` and read back verified
  (facts above).
- L5: T1 implemented test-first: RED = 25 failing assertions in
  launcher_terminal_test scenario pins (quick-mode contract), GREEN =
  65/65 across the rebuilt launcher_terminal harness; pending discoveries
  encoded as pins: (a) `set -eu` aborts on a failed command substitution
  capture — toggles capture rc via `$(...) || rc=$?` so failures are handled
  and loud (scenario 13: uncertain D-Bus failure dies without a second
  panel); (b) surfaceless instances are retired with TERM only when no
  toggle ever succeeded (showing-panel rule, S3(c)); (c) settings-driven
  ghostty + side compose per-side app ids (org.streambeats.panel.<side>).
  Full pipeline green (gofmt/build/vet/test, mod tidy clean, all five
  scripts/tests suites incl. macos smoke with bash per its shebang).
