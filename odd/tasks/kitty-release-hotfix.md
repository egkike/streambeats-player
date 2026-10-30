# Kitty Release Hotfix (issue #108)

v1.9.0 ships broken for the kitty path: the release tarball lacks
`assets/kitty/` (the `.goreleaser.yaml` files list never gained the
presets), so the installer lands nothing and the launcher dies for any
kitty-wanting host; separately, `streambeats-panel`'s toggle is built
on the ghostty model (systemd unit guard + kdotool by title) and cannot
relay kitty's native re-run contract. This hotfix lands both and pins
the packaging coverage as a regression guard.

Specs quote issue #108 verbatim. Branch: `fix/kitty-release-hotfix`.
Target release: `v1.9.1`.

## Specs

- S1: "`goreleaser.yaml`: add `assets/kitty/streambeats-*.conf` to `files:`" (the
  four per-side kitty presets must ride the archive like the ghostty family).
- S2: "scripts/streambeats-panel: resolve the hosting terminal (same
  precedence contract as the launcher: read settings.json panelTerminal,
  exact same anchored scanner contract; `--terminal` flag not accepted at
  this layer) — when it resolves kitty, bypass the systemd unit guard and
  spawn/relay the launcher directly (setsid/detach path reused), so
  re-running the panel toggles kitty natively; the ghostty path stays
  byte-identical."
- S3: "scripts/tests gains a `.goreleaser.yaml` coverage assertion — every
  asset family the installer expects (ghostty + kitty presets) must be
  declared in the packaging config".
- S4: "docs: install.md advisory (kdotool needed only for the ghostty
  panel path; kitty toggles by re-run), README parity note."
- S5 (invariants, from #108): "Ghostty path stays byte-identical";
  "launcher precedence contract unchanged"; "release flow unchanged in
  mechanics; only packaging contents grow"; "No secrets anywhere;
  installer trust model unchanged".
- S6 (negative controls, from #108): tar guard pins declaration (the
  first archival run after the fix is the live proof); without kitty
  nothing changes; without kdotool kitty toggles; RED first for every
  rule.
- S7 (owner validation quotes): "instalé... entré a c y activé kitty...
  al volver a ejecutar con ctrl+alt+m ya no abre la app" — the live
  symptom this cycle closes.

## Tasks

- T1 (S3): RED — packaging-coverage test: `.goreleaser.yaml` must declare
  both `assets/ghostty/streambeats-*.conf` and `assets/kitty/streambeats-*.conf`
  families the installer consumes.
- T2 (S2): RED — panel-wrapper scenarios in desktop_integration_test.sh:
  with a kitty-resolving settings sandbox, the wrapper must run the
  launcher directly (bypassing the unit/kdotool guard) even without
  systemd/kdotool in PATH; with a ghostty/unset resolution the existing
  behavior is byte-identical (unit guard + kdotool semantics unchanged).
- T3 (S1): GREEN — `.goreleaser.yaml` files entry; T1 goes green.
- T4 (S2): GREEN — panel kitty resolution reuse + guard bypass; T2 goes
  green; ghostty scenarios unchanged.
- T5 (S4): docs sync (install.md kdotool advisory scoping + README note).
- T6 (S5): guards — existing panel/launcher tests pass byte-identically;
  pipeline; work-unit commits with the ODD log.

## Log

- L1 (owner, 2026-10-30): "Ya instalé... abrí la app y luego entré a c y
  activé kitty... al volver a ejecutar con ctrl+alt+m ya no abre la app"
  — live validation; host diagnosis: tarball missing assets/kitty
  (verified by downloading the published tarball) + panel toggle
  ghostty-model. Authorized: "Vamos con el hotfix v1.9.1" → issue #108
  (`status:approved`, standing grant model #25, `type:bug`).
