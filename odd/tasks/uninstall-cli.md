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
- L3 (2026-10-25): Native RDD review, two lineages over the same candidate:
  - Lineage 1 `review-a6b0261e1b3e4679` (HIGH, 714 lines): 4-lens reviewers group + refuter admitted; refuter froze ONE candidate-caused CRITICAL — R3-001 (`streambeats uninstall --cache` never reached the parser: `parseCommand`'s generic >1-arguments guard ran before the uninstall delegate). Fixed (commit `e4039bd`): delegate moved before the guard, regression test `TestUninstallCacheFlagReachesUninstallerR31` (exit path before/after) and sibling-behavior pin `TestOtherCommandsStillRejectExtraArguments`. Correction plan captured (37/200 lines) and committed; the lineage's targeted-validator slot then rejected EVERY binding (`missing or stale for current STATUS`) — the known upstream validator-seam family (gentle-shell#1324/#924 pattern, documented in mcp-appointments-crm receipts). Left as durable evidence, per the same receipts.
  - Lineage 2 `review-ce2528002629dde4` (HIGH, 745 lines, owner-approved fresh START over the corrected candidate per the established receipt procedure): 3 reviewers admitted straight to **APPROVED** — zero corrections this time; 6 informational WARNINGs recorded as follow-ups (R1-manifest-path-scope, R3-001/002/003, R4-001/002, all `cmd/streambeats/uninstall.go`); exact `acknowledge-approved` executed, authority burned (`gentle-ai.review-acknowledged/v1`).
- L4 (2026-10-25): Feature COMPLETE. Pipeline green (gofmt/vet/build/test, install harness 43/43, desktop integration 21/21). Delivery: PR `Closes #9` referencing lineage 2 receipt; user data guarantees verified by tests (library never touched, cache only with --cache).
- L5 (2026-10-25, post-merge): PR #11 merged by owner (`29ee7d9`, merge-commit CI green). The lint-fix delta and the corrections cycle ran as lineage 3 `review-f3c24666367be0d6` (741 lines): 4 lens reviewers + refuter froze TWO candidate-made CRITICALs — R3-001 (the end-to-end `--cache` test built production deps and could clear the developer's real cache) and R4-001 (custom-prefix installs were never removed by plain `streambeats uninstall` — success reported while the install stayed). Correction plan captured (60/200), fixed in `e19ba7d` (prefix now derived from the running binary's own layout env > binary layout > $HOME/.local; the cache test pins XDG_CACHE_HOME and asserts the clearing). The lineage-3 validator seam blocked all bindings again (same upstream family) → per the receipt procedure, a FRESH lineage `review-1ea31dc654632fab` (medium, 3 files, 111 lines, base = merged main) reviewed ONLY the correction delta: **APPROVED, zero corrections, authority burned**. Delivery of the fix: follow-up PR citing both source lineages as evidence.
