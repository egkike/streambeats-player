# Panel Helper Packaging (issue #122)

Second delivery defect of the #108 cycle: `scripts/panel-terminal` — the
shared scanner the wrapper sources for the kitty relay — never made it
into `.goreleaser.yaml`'s files list, so v1.9.1/v1.9.2 tarballs shipped
without it. Installed trees then SILENTLY fell into the ghostty unit
path for kitty hosts: ctrl+alt+m opens the panel, the second press does
nothing (owner live report, "no esconde ni minimiza; solo kitty"). The
launcher stayed unbroken because it carries its own inline scanner.

Specs quote issue #122 verbatim. Branch: `fix/panel-helper-packaging`.
Target release: `v1.9.3`.

## Specs

- S1: "`.goreleaser.yaml`: add `scripts/panel-terminal` to files."
- S2: "`install.sh` stage the helper from the archive AND from the raw
  fallback, exactly like the other two scripts."
- S3: "`packaging_coverage_test.sh`: require the helper family" + installer
  tests asserting it lands on both paths.
- S4: "Wrapper hardening: when the helper is MISSING, notify LOUDLY
  (`panel-terminal helper missing — reinstall for the kitty relay;
  assuming ghostty`) and proceed with the ghostty path explicitly — no
  more silent mis-routing."
- S5: release v1.9.3 whose tarball must contain the helper (live proof:
  download + list).

## Tasks

- T1 (S3,part): RED — packaging guard requires scripts/panel-terminal.
- T2 (S3,rest): RED — install_test asserts the helper on both bundle and
  raw paths.
- T3 (S4): RED — desktop scenario 9: helperless wrapper announces loudly
  and still serves the ghostty path (unit spawn).
- T4 (S1): GREEN — goreleaser files entry.
- T5 (S2): GREEN — install.sh bundle mv + raw fetch of the helper.
- T6 (S4): GREEN — wrapper advisory (moved AFTER notify() definition; the
  advisory originally fired before the function existed — caught by the
  scenario staying red, fixed by relocation).
- T7: pipeline + suites + work-unit commits + ODD ledger.
- T8: review + PR + release v1.9.3 + owner reinstall/live validation.

## Log

- L1 (owner, 2026-10-31): "En kitty al dar ctrl+alt+m abre la app pero al
  volver a darle ctrl+alt+m no esconde o minimiza la app... solo kitty,
  con ghostty bien" — authorized: "Dale con el fix cycle" → issue #122
  (`status:approved`, `type:bug`; standing grant model #25).
- L2 (2026-10-31): RED→GREEN. Unit 1 (T1+T4): goreleaser helper entry +
  packaging-guard assert (RED: 'panel-terminal helper NOT declared').
  Unit 2 (T2+T5): install.sh bundle+raw helper staging + installer tests
  (fixtures: fake-dist helper + raw/v1.1.0 helper; 81/81). Unit 3
  (T3+T6): wrapper advisory + desktop scenario 9 (helperless → loud
  advisory + unit spawn; 50/50). Test-harness lesson: scenario-9 stubs
  needed UNQUOTED heredocs (<<'EOF' froze $K9 inside the stub and the
  logs vanished into /) and the wrapper advisory required notify() to be
  defined before use. Final suites: packaging 4/4 · desktop 50/50 ·
  launcher 38/38 · install 81/81 · shellcheck clean · go full green.
