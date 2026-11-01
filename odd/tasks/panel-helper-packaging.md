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
- L3 (2026-10-31): RDD native review receipt — lineage review-4580a917941f2f80
  (high tier, 4 lens runs host-relayed, owner-authorized; no refuter
  requested). OUTCOME: APPROVED with zero corrections; acknowledgement
  burned (gentle-ai.review-acknowledged/v1, consumed_revision
  sha256:cc9afede0b6097b2c450e400561022f575ee8c1aab0948cf3d9322acb1bf5ba1).
  Two informational warnings recorded as separate follow-up work (never
  reopen): R2-001 (readability) and R3-001 (reliability), both at
  scripts/install.sh:245-246 (the raw-fallback helper staging lines).
  Delivery: PR + owner merge + tag v1.9.3 + archive live proof (S5).
- L4 (2026-10-31): release evidence (governance step 5) — PR #123 MERGED as
  712f86b (issue #122 CLOSED by keyword), merge-commit CI success ×2, branch
  cleaned. Annotated tag v1.9.3 pushed; release workflow success: GitHub
  Release v1.9.3 published 2026-10-31T17:26:07Z as Latest with 6 assets.
  S5 LIVE PROOF: downloaded the published linux_amd64 tarball and listed
  it — scripts/panel-terminal IS inside the archive. Owner laptop
  recovery: reinstall from v1.9.3 (installer now lands the helper) →
  ctrl+alt+m kitty relay toggle re-validated live to close the cycle.
- L5 (2026-10-31): OWNER LIVE VALIDATION — installed from v1.9.3
  (post-#125 main) and reported "ya funciona correctamente :)": the
  ctrl+alt+m kitty relay toggle works end-to-end on the laptop. The
  helper-packaging cycle (issue #122 → PR #123 → release v1.9.3 → S5
  archive proof → owner validation) is CLOSED end-to-end. Doctrine
  reaffirmed for the arc: every fix cycle finishes with an entry like
  this one (L6 / L21 / L5) after the owner's live validation, and the
  packaging coverage guard (ghostty + kitty presets + shared helper)
  keeps the delivery mistakes of this arc from ever shipping again.
- L6 (2026-10-31): follow-ups informational cleanup (issue #128, owner
  directive "Limpiemos los follow-ups informativos de pulido") — the two
  findings at install.sh:245-246 from review-4580a917941f2f80 resolved:
  R3-001 reliability → the bundled staging now RESULT-CHECKS the outcome
  (helper + both preset families verified in the installed tree; any
  missing family warns loudly with the #122-class lesson named), killing
  the silent ||true skips; R2-001 readability → the raw-fallback
  fetch/mv quartet deduped into fetch_raw_file() (labels preserved per
  caller). RED first: install_test scenario 2b (partial archive: quick
  rides, helper+presets absent → deleted from the fixture; asserts BOTH
  warns + degraded-not-fatal + absent-files-stay-absent). Suites: install
  85/85, desktop 50/50, launcher 38/38, packaging 4/4, shellcheck clean,
  go green.
- L7 (2026-10-31): RDD native review receipt — lineage review-e03549a3e6fce89a
  (high tier, 3 lens runs host-relayed, owner-authorized: resilience 0,
  readability 1, reliability 2; no refuter requested). OUTCOME: APPROVED
  with zero corrections; acknowledgement burned
  (gentle-ai.review-acknowledged/v1, consumed_revision
  sha256:fb016ce9aa0cb0f1831f9ca0a23c563ab21a48ccb5a942ee39bd6d2f76a30caf).
  Two NEW informational findings parked as separate later work (never
  reopen): R2-001 (readability, install.sh:257-260) and R4-001
  (resilience, install.sh:255-257) — both at the fetch_raw_file/_advisory
  zone this cycle just created. Delivery: PR under ordinary policy.
- L8 (2026-11-01): follow-up cleanup of the L7 findings (issue #130, owner
  directive "Hagamos los 2 findings nuevos (install.sh:255-260) y dejemos
  limpio"): R4-001 resilience → the bundled staging now checks PER SIDE
  (loop over top/bottom/left/right for both preset families; the single
  "right" sentinel could not catch missing top/bottom/left) and R2-001
  readability → the warn trio folded into the per-side loop (one template,
  per-side interpolation). RED first: install_test scenario 2c builds an
  archive missing ONLY kitty-bottom (right sentinel present — proven the
  granularity gap) and asserts the specific warn; scenario 2b (whole
  family missing) re-anchored to the per-side wording.Fixture lesson: the
  bundled dist never mkdir'd assets/kitty — the ptr redirections failed
  silently and the fixture had no kitty presets at all (that is why 2b's
  family check passed since the very first version). Suites: install
  87/87, desktop 50/50, launcher 38/38, packaging 4/4, shellcheck clean,
  go green.
