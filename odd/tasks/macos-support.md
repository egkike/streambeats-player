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
