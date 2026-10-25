# Feature: declared macOS support (darwin installer branch + macOS CI)

Issue: egkike/streambeats-player#10 (status:approved). Branch: `feat/macos-support`. Ordered AFTER uninstall-cli (shares `scripts/install.sh`).

## Specs

- S1: `scripts/install.sh` detects `darwin` natively (`uname -s`); the darwin
  branch skips `apply_desktop_integration()` (`.desktop`, hicolor icon) and the
  Linux-only advisories (systemd units, Ghostty desktop rules), each with a
  visible note. The bash 3.2 floor keeps holding: the darwin branch adds no
  post-3.2 construct.
- S2: PATH hint for macOS defaults to zsh: guidance wrapped for `~/.zprofile`
  instead of the `.bashrc`-style hint.
- S3: Completion echo is per-OS: advertise `streambeats uninstall` with the
  darwin path set (once the uninstall CLI ships).
- S4: `streambeats-panel` on darwin: no systemd. Ship a documented
  `setsid`/background fallback keeping the detached-panel semantics
  (environment carried, non-blocking spawn); a launchd variant happens only if
  the fallback proves insufficient. No guess-patching; the decision is
  documented in `docs/`.
- S5: New macOS CI job in `.github/workflows/ci.yml` (`macos-14`, arm64,
  path-filtered like the Linux job): installer run with `--prefix` tmp asserting
  the bundle lands, integration is skipped, and the manifest is written;
  `streambeats uninstall` end-to-end; darwin binary smoke test (`--version`).
  This job is the live bash-3.2 gate for the whole script.
- S6: Docs (`README.md` Install/Requirements, `docs/install.md`) declare macOS
  install support; the native Ghostty hotkey is documented as the official
  macOS floating experience (replaces the KDE .desktop/KWin story on darwin);
  the pending live validation is declared honestly in the docs and PR.
- S7: Out of scope: iOS (architecturally separate product — no terminal/process
  exec, no mpv port) and KWin/keybind presets (Linux-only, untouched).
- S8: Live validation on a physical Mac (floating panel + audio + feel) is
  deferred to the owner and non-blocking for the PR; CI covers installer,
  uninstall, bash 3.2 and binary behavior.

## Tasks

| ID | S# | Route | Commit |
| -- | -- | ----- | ------ |
| T1 | S1,S2,S3,S4 | install.sh darwin branch + streambeats-panel darwin fallback + harness scenarios | feat(install): native darwin support (no freedesktop, zsh hint) |
| T2 | S5 | ci.yml macOS job (installer + uninstall + smoke) | ci: add macOS arm64 install/uninstall gate |
| T3 | S6,S7 | README + docs/install.md (+ `docs/` note for the floating path) | docs: declare macOS install support |

## Log

- L1 (2026-10-25): User asked whether macOS could be tested on GitHub: "se podrían hacer pruebas para macOS en Github? si es posible lo hacemos ahí, si no cerramos sin pruebas y la hago cuando pueda" — decision: CI validates installer/uninstall/bash-3.2/binary on a macos-14 runner; owner live validation (floating panel + audio) deferred, non-blocking.
- L2 (2026-10-25): User: "Dale a ambos segun tu propuesta" — both issues approved; implementation ordered uninstall-cli first (shared file), then macOS.
- L3 (2026-10-25): Feature COMPLETE on `feat/macos-support` (4 work units: 94e2378 darwin branch + harness scenario 8 RED→GREEN, 3f5616c ci.yml macOS job + macos_smoke_test.sh 19/19, f29a64e docs, cbf02e3 portable checksum-tool fix). Native RDD:
  - Lineage `review-11b27d832707cc05` (HIGH, 396→403 lines): 3 lenses + refuter; the refuter froze one deterministic candidate-introduced CRITICAL — R3-macos-checksum-tool/R4-001 (`sha256sum` absent on stock macOS; the smoke's checksums.txt stage would end empty and the macOS CI job would fail). Fixed in `cbf02e3` by mirroring the installer's own tool discovery (`shasum -a 256` fallback; branch emitted under env in the inline probe). Plan captured (14/198); the lineage's targeted-validator seam then rejected every binding (third instance of the documented upstream family) → left as durable evidence per the receipt procedure.
  - Fresh lineage `review-43c7aee413152ac7` (HIGH, 403 lines, owner-approved fresh START over the corrected candidate): 3 lenses straight to **APPROVED**, zero corrections, one informational WARNING (R4-001 scripts/install.sh:272); exact `acknowledge-approved` burned (`gentle-ai.review-acknowledged/v1`).
- Pending owner live validation: floating panel + audio on a physical Mac (declared in README/docs/PR); CI covers installer, uninstall, bash 3.2 and the darwin binary smoke.
- L4 (2026-10-25): PR #14 merged by owner (merge commit `f233ee0`, merge-commit CI green; `Install macOS` job green on the real macos-14 arm64 runner — `macos_smoke_test.sh` 19/19 natively: real darwin binary, bash 3.2 gate live, uninstall e2e + idempotent probe). Feature COMPLETE; the only item declared pending is the owner's live validation of the floating panel + audio on a physical Mac. Sync done locally (main pulled to merge tip).
