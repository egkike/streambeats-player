# kde-panel-integration — .desktop + keybind, the 100% floating panel

KDE Plasma desktop integration for the installed streambeats panel: one
action (menu/KRunner/keybind) spawns the detached floating side panel with a
single-instance guard; no terminal involved.

## Specs

- S1 (user request, this cycle): "Dale mejor hagamos el .desktop + keybind global KDE para dejarlo 100% 'ventanita flotante que se oculta/mueve'." — plus the chat-agreed scope: .desktop + icon; streambeats-panel spawn script (systemd-run detached, per-side unit, guard + notify); title = streambeats-panel in the 4 Ghostty presets; shipped KWin window-rules documentation + keybind instructions; installer installs/generates all of it.
- S2 (issue gh#7): the panel must be reachable with ONE KDE action, detached from any terminal.
- S3 (issue gh#7 negative controls): no systemd → background fallback, never die; desktop-file-validate absence in CI SKIPS (not "passes"); second launch of the same side does not duplicate panels (per-side unit guard); generated .desktop Exec uses the absolute prefix path.
- S4 (explicit non-goal): true toggle hide/show via KWin scripting/kdotool is OUT — separate later issue after live KDE validation.

## Tasks

- T1 (S1): assets: `assets/desktop/streambeats-panel.desktop.in` (4 side actions + declarative Actions= key per desktop-file-validate), `assets/icons/streambeats-panel.svg`, `assets/kwin/window-rules.md` (supported System Settings path; kwriteconfig automation deliberately NOT shipped); presets gain `class = streambeats-panel` (title unchanged); goreleaser ships the new files. Commit 42b4b43 ✅
- T2 (S1,S3): `scripts/streambeats-panel` spawn script: symlinks-aware tree resolution, per-side unit `streambeats-panel-<side>`, guard via is-active → kdotool focus/notify; setsid fallback without systemd. Commit (with T1) ✅
- T3 (S1,S3): installer extension: optional-asset consumer (archive → raw fallback → graceful skip-with-warn; the core install NEVER fails for a missing panel asset); generated .desktop with @PREFIX@→absolute (sed-escaped replacement), hicolor icon, kwin doc, third wrapper, uninstall line extended. Commit 76470d3 ✅
- T4 (S3): `scripts/tests/desktop_integration_test.sh` — stubbed systemctl/systemd-run in fake tree: guard path (no spawn), per-side unit + --user --collect + launcher args, no-systemd fallback, invalid side, installer-generated .desktop payload + desktop-file-validate (SKIP if absent). RED exposed two REAL bugs, fixed in b9c6f8a: (a) release JSON parsing was layout-dependent (minified API body picked the CHECKSUMS url as the tarball!) — now flattens + splits + field-anchored; (b) template lacked the declarative Actions= key. Commits b9c6f8a ✅ (21/21 + install_test 34/34).
- T5 (S1): docs/kde-panel-integration.md (one-action keybind, .desktop actions, window rules anchored to class = streambeats-panel, toggle explicitly deferred to a separate ISSUE with live KDE validation), README section extended, docs/install.md layout table +7 rows. This commit ⇢

## Log

- L1 (user, this cycle): "Dale mejor hagamos el .desktop + keybind global KDE para dejarlo 100% 'ventanita flotante que se oculta/mueve'. Qué debemos hacer para esto? cual es el plan?" — plan presented (scope + explicit toggle OUT); user: "Dale aprobado el issue #6, lo puedes armar y poner status:approved" → issue registered as gh#7 with status:approved.

- L2 (delivery evidence): harness 21/21 desktop_integration_test + 34/34 install_test red->green over 3 work-unit commits (42b4b43, 76470d3, b9c6f8a). Real-bug story preserved in T4: GH API minified-JSON trap + mapper Actions=. XCTest... note the mock: kwinrulesrc automation and true toggle deliberately OUT (S4, live-KDE validation needed).
- L3 (delivered): PR #8 merged (6290c6b), branch deleted remote+local; merge-commit CI green BEFORE further main pushes (post-merge sequencing). v1.2.0 tag gated on this green (release-bound decision pending owner). Live KDE validation (keybind + window rules + desktop entry actions) is the owner post-merge step; advisory follow-ups (R2-001..R4-002) batched for the quality sweep.
- L4 (delivery complete, owner authorized tag): v1.2.0 tagged+released (all 5 archives + checksums), real one-liner reinstall on the owner machine landed the desktop integration: streambeats-panel.desktop generated with absolute Exec (desktop-file-validate VALID), hicolor icon, 3 wrappers. LIVE validation on the real KDE session: streambeats-panel right spawned as transient unit streambeats-panel-right.service (active), single-instance guard answered "already running" on second press. Panel opens detached; keybind + window rules remain a two-minute owner UI step (docs/kde-panel-integration.md). Feature CLOSED.
