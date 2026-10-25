# Feature: uninstall CLI (`streambeats uninstall`)

Issue: egkike/streambeats-player#9 (status:approved). Branch: `feat/uninstall-cli`.

## Specs

- S1: New CLI subcommand `streambeats uninstall` in the existing dispatch pattern
  (`refresh`, `sync`, `help`), accepting an optional `--cache` flag.
- S2: Uninstall removes exactly what the installer lands:
  `$PREFIX/share/streambeats/` (binary, scripts, assets),
  `$PREFIX/bin/streambeats`, `$PREFIX/bin/streambeats-quick`,
  `$PREFIX/bin/streambeats-panel`, generated
  `$PREFIX/share/applications/streambeats-panel.desktop`,
  `$PREFIX/share/icons/hicolor/scalable/apps/streambeats-panel.svg`.
- S3: Before deletion, stop any live `streambeats-panel-*.service` transient
  systemd units so a running panel does not hold or regenerate files.
- S4: The installer writes an install manifest at
  `$PREFIX/share/streambeats/manifest.txt`; uninstall uses the manifest when
  present (precise) and falls back to the known layout otherwise (older installs
  without a manifest).
- S5: `--cache` opt-in clears the catalog cache; by default the cache and
  `~/Music/StreamBeats` stay untouched (declared user data).
- S6: Idempotent: when nothing is installed, report "nothing to uninstall" and
  exit 0.
- S7: From this release on, the installer's completion echo advertises
  `streambeats uninstall` instead of the manual `rm` line; help text, `README.md`
  and `docs/install.md` Uninstall sections lead with the CLI, keeping the manual
  inventory as fallback for pre-CLI versions.
- S8: Tests are RED-first Go tests against an isolated temp prefix (no real
  systemd; unit stopper is injectable). Negative controls: paths not in the
  manifest are NOT deleted; a rejected/absent prefix changes nothing.

## Tasks

| ID | S# | Route | Commit |
| -- | -- | ----- | ------ |
| T1 | S8,S5,S1 | Go: parseCommand + runUninstall, injectable paths/unit-stopper, RED→GREEN | ✅ feat(uninstall): CLI uninstaller with manifest-aware removal |
| T2 | S4 | install.sh: write manifest.txt at install time + registry manifest scenario in install_test.sh | ✅ feat(install): record uninstall manifest and advertise the uninstall CLI |
| T3 | S6,S7 | help text + README + docs/install.md uninstall sections lead with CLI | ✅ docs(uninstall): document `streambeats uninstall` as the primary path |

## Log

- L1 (2026-10-25): User: "Solo te consulto, crees que sería mejor empezar con lo que tiene que ver con streambeats uninstall como CLI real y macOS support primero? Y dejar todo listo lo que tenga que ver con Instalación y Desintalación (Linux y macOS)? Qué te parece?" — follow-up: "Dale a ambos segun tu propuesta". Scope locked with user in chat (issue #9 body): manifest-based precise uninstall, units stopped first, `--cache` opt-in, user data untouched by default, idempotent exit 0.
- L2 (2026-10-25): Work-unit slices: T1 slice alone was 548 authored lines (CLI + its full RED→GREEN test suite are one unit; splitting tests from behavior at this size is artificial) — `size:exception` declared on the PR per the advisory heuristic. T2 (592 total across 2 commits). Harness grew install_test.sh 34→43 assertions; desktop_integration_test.sh stays 21/21 green.
