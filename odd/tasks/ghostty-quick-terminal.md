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
- S4: "La ventana de ghostty es más corta o mas chica que la de kitty, no
  son del mismo tamaño" (owner, live desktop verification round) — the
  panel geometry must match the kitty host's full-height strip, not the
  compositor-default secondary axis.

## Tasks

- T1 `scripts/streambeats-quick` ghostty v2 branch: per-run confdir config gains
  per-side `class = org.streambeats.panel.<side>` + `command = <wrapper>`;
  wrapper resets `XDG_CONFIG_HOME` for the player child (issue #80 contract);
  spawn-if-not-claimed → toggles via gdbus `org.gtk.Actions.Activate
  toggle-quick-terminal`; falls back to the dedicated-window flow when the
  claim/toggle fails or ghostty lacks the action. Commit: bbad579 (L5).
- T2 `scripts/streambeats-panel` ghostty relay mirroring `kitty_host` (issue
  #108 S2): every press relays the launcher, detached, before the unit guard.
  Commit: logged in L6.
- T3 presets: valid dotted per-side `class`; KWin window-rule matching
  re-checked (`assets/kwin/window-rules.md`); stale 1.3.1-only comments
  removed. Commit: c0460de (L7).
- T4 docs: `docs/ghostty.md` rewritten for 1.4.0 (limitation §2 resolved; CLI
  `--class` defect documented as upstream status), KDE integration docs,
  `scripts/tests` coverage. Commit: c74fb6e (L7).
- T6 presets: explicit `quick-terminal-size` pairs (`22%,100%` vertical
  strips, `30%,100%` horizontal strips) so the secondary axis matches the
  kitty host's full-height/full-width panel instead of the compositor-
  reported default (owner-reported, S4). Commit: 7d4ae4d (L8).
- T5 (outside the repo): upstream issue to ghostty-org/ghostty —
  `+toggle-quick-terminal` parses `Options.class` but `run()` ignores it
  (hardcoded `.detect` target), so `--class` cannot address a custom instance.
  Commit: n/a (separate repository; report staged at
  /tmp/streambeats-upstream-bug-report.md pending owner filing — upstream
  rejects issue creation for our GitHub identity).

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
- L6: T2 implemented test-first (RED: relay absent, kdotool consulted via
  the guard path, second press never relayed; GREEN full). The ghostty relay
  resolves the host from the shared helper plus the builtin ghostty-first
  order and gates on the same zero-side-effect +help fingerprint as the
  launcher; pre-1.4 hosts fail the probe and keep the legacy kdotool toggle
  (pin updated deliberately — with an ACTIVE unit the legacy toggle never
  spawns a unit, so the spawn pin was restated as kdotool-consulted).
- L7: T3 + T4 committed (c0460de presets; docs rewrite this commit):
  presets validate clean on 1.4.0 both as files and through the generated
  quick-mode merge (runtime verified); stale 1.3.1 headers rewritten;
  `class = streambeats-panel` documented as the FALLBACK identity; the doc
  hint table drift was corrected (54×24 right preset, 30 top height) while
  rewriting `docs/ghostty.md` for the 1.4.0 mechanism (per-side app-id durabilty
  claim + gdbus toggle + upstream `--class` defect + KDE relay), and the KWin
  rules doc now states the layer-shell boundary explicitly.
- L8 (live functional round with the owner): the ghostty panel spawned,
  claimed org.streambeats.panel.right and showed the player TUI (owner
  verified visually). Owner reported the panel SMALLER than the kitty host
  panel (S4 verbatim) — root cause upstream geometry on Plasma Wayland
  (#11552-class): the single-value size left the secondary axis compositor-
  derived. Fix T6: explicit size pairs on all four presets; the final round
  re-verifies parity next to the kitty panel. The lifecycle stayed clean:
  the owner quit via q (player closed, app exited, claim released) and a
  subsequent press respawned a fresh claim (51748 → 51887 handover observed).
- L9 (native review cycle, lineage review-eea8da689a859364): high-risk tier
  (process-boundary shell evidence), 3 lens runs + refuter admitted; closure
  ESCALATED with two corroborated findings,
  "maintainer action is informational": R3-001 (the wait-loop kill could, in
  a tiny concurrent-press race, retire an instance drawing a panel) and
  R4-001 (the relay's unconditional notification). Owner dispositions:
  R3-001 = graceful retirement fix NOW (retire_host walks the quit GAction,
  TERM is the last resort only when the bus is gone; wait-loop branches retire
  in EITHER case, so no surfaceless instance leaks); R4-001 = documented as
  kitty-parity doctrine (native review R4-001 note in
  docs/kde-panel-integration.md); geometry "más corto" = accepted residual of
  the compositor safe area (docs/ghostty.md states the accepted residual +
  user-taste px overrides). One more mid-handshake director: a wait-loop
  uncertain transport failure now retires and falls back instead of dying —
  a surfaceless instance is never leaked into mid-handshake failures
  (scenario 15 pins the quit-call; scenario 13 keeps the pre-spawn die-loud
  contract).
- L10 (second review pass, lineage review-c8ac065e61473adc): the same three
  lenses ran on the corrected tree; the refuter confirmed the R3-001 residual
  (the quit-path kill could fire after ANY failed quit call) →
  correction_required → bounded correction plan (17 diff lines) → commit
  0e84656 (retire_host TERMs only when the quit failure proves nobody holds
  the bus claim: ServiceUnknown = surfaceless wedged spawn; a claim holder
  survives a lost quit reply and the next press retries — scenario 16 pins
  the surviving host pid, 71/71) → targeted validator APPROVED →
  acknowledgement burned: receipt lineage review-c8ac065e61473adc,
  consumed revision
  sha256:21c28e833eb6ffb5d8aaf4b4bb9e57ba719c5134066237bed5e732cbdaf36896,
  target sha256:9508c4d4ac903a36e126f9a17cf060fca4fa248101e37fed2d44c5d6ff4a0d82.
  Advisory findings (non-blocking, later follow-ups): R2-001
  scripts/streambeats-quick:282, R2-002 docs/ghostty.md:86-89, R4-001
  scripts/streambeats-panel:161-165 (R4-001 doctrine note is already in the
  docs, L9).
- L11 (delivery prep): the full cycle totals ~1290 authored lines across 9
  commits (each work-unit ≤ 362 lines, inside the per-commit advisory; the
  PR exceeds the 400-line advisory because one feature spans launcher +
  wrapper + presets + docs + both test harnesses) — `size:exception` is
  declared in the PR with this rationale. The upstream `--class` issue
  could not be filed (upstream restricts issue creation for our identity);
  the full report travels in the PR body for the owner to file.
