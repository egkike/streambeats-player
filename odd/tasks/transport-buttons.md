# Transport Buttons (UX Look Phase B)

Transport buttons in the bottom block (mouse-clickable), bottom-block redesign with
the help-truncation fix, and absorption of the 5 Phase A informational findings.
Issue: egkike/streambeats-player#37 (pre-approved per standing grant, type:feature).

## Specs

- **S1**: "Transport buttons in the bottom block: play/pause (⏯), stop (⏹), prev (⏮), next (⏭) and volume (🔊) drawn ON the panel, clickable via Bubbletea MouseCellMsg."
- **S2**: "Keybinds are NEVER replaced: every existing shortcut keeps working (keys.go single source of truth); mouse is an ADDED way to do the same actions." — buttons route through the SAME commands as keybinds (no duplicate action logic).
- **S3**: owner verbatim: "Ahora veo que los controles están en 2 lineas y ambas lineas son más largas que el ancho de la ventana. ¿Crees que esto lo podemos meter en Fase A para arreglarlo cuando toquemos los controles y botones?" — the 2 help lines must fit the 52-col panel (no truncation) after the bottom-block redesign.
- **S4**: "absorb the 5 informational findings from the Phase A review receipt (review-656a2bb02230f588, odd/tasks/theme-config.md L9)" — R1-001 settings.go:74-75, R3-001 model.go:478, R3-002 model.go:465-482, R4-001 model.go:339-341, R4-002 settings.go:73-78: re-derive prose against merged tree, judge real vs no-op with reason, fix with test when actionable.
- **S5**: invariants untouched: idle-active ONLY advances the queue; fresh resolve before every loadfile; Quit stops the world; failure paths mutate nothing; no crash for absent mpv; any terminal width; heavy work in commands, not Update.
- **S6**: "Clicks land only on the label rows inside the panel (bounds-checked, no negatives); no crash when mouse support is unavailable."

## Tasks

- **T1** (S1, S2): transport button row in the bottom block + mouse hit-testing (bounds-checked) routing to the SAME commands as keybinds. **DONE** (row + spans + tea.MouseMsg route + parity tests; enabled via tea.WithMouseCellMotion). - **T2** (S3): bottom-block redesign so both help lines fit 52 cols without truncation (wrap/rearrange, keep `keys.go` source of truth). 
- **T3** (S4): re-derive the 5 findings' prose on merged main, disposition table (fix or no-op with reason) recorded here; fixes carry their own RED→GREEN. **DONE** (3 fixes + 1 reasoned no-op + latent-fix; L4).
- **T4** (S5, S6): docs + ODD log; differential check for touched shared helpers. **DONE** (README mouse sentence; harnesses 71/71 + 21/21; suite + race).

## Log

- **L1**: owner approved Phase B split (obs-1525/1541/1550 standing agenda; help-truncation verbatim quote recorded at obs-1541). Issue #37 opened pre-approved (status:approved + type:feature in one call).
- **L2**: merge of Phase A PR #36 verified (merge commit 4615a57, CI success) before this branch.
- **L3**: T1+T2 RED→GREEN as one bottom-block unit: tea API verified from the vendored source v1.3.10 (clicks = MouseMsg{Action:MouseActionPress, Button:MouseButtonLeft}; enable via tea.WithMouseCellMotion ProgramOption — the issue's "MouseCellMsg" does not exist as a type; tea MouseMsg IS the cell-motion event). buttonsLine rendered between bar and status slot (chromeLines 8→9; bottom block bar(2)+buttons(1)+status(1)+help(2)=6, status stays at len-3 — all three R31 tests survived intact). buttonLayout spans are the single table consumed by BOTH the renderer and the hit-test (no drift). Dispatch routes selection/bounds-checked clicks to the SAME handlers (queuePrev/togglePause/stopPlayback/queueNext/changeVolume) — S2 parity paired-test: every click compared key-vs-click against twin fakes, commands executed like the tea runtime would. Guarda config: clicks inert in ModeConfig (same keyboard contract). Glyphs from the spec verbatim (⏮ ⏯⏸/▶ ⏹ ⏭ 🔊); volume click = volume up (only action the issue names for 🔊). Help RED→GREEN: both lines ≤ 52 runes bounded at the source (k/↑ j/↓ enter open | esc back l lib c theme; space ⏯ s ⏹ < > ⏮⏭ +/- 🔊 q quit), view truncate() never fires on help; TestHelpTextListsEveryAction + TestViewRendersWithoutPanicking updated to the new terse copy (list-every-action invariant intact).
- **L4**: T3 dispositions (receipts review-656a2bb02230f588; prose re-derived on merged main since receipts store ID+lens+file:line only): R1-001 FIX (unique CreateTemp staging instead of fixed ".tmp", chmod 0600 preserved; RED: a directory parked at settings.json.tmp froze the whole save) · R4-002 FIX (deferred Remove: failed commits leave no staged residue; RED: rename-onto-directory left the tmp behind) · R4-001 FIX (mpv-reported volume clamped on receipt; RED: 150 → 150, -7 → -7 on the status math) · R3-002 FIX (configIndexFor fallback returns 0 directly, no recursion if Default ever left the list; latent by construction — Default==System is in the list, so no RED possible; reasoning documented) · R3-001 NO-OP (the flagged lines are the configKey doc comment; the claim was audited: every key in ModeConfig routes through configKey and mouse is inert there — both covered by tests). All GREEN + race clean; harnesses 71/71 and 21/21 unaffected.
- **T4** DONE: README grows the mouse sentence (keybinds first, mouse as the added way); ODD log L3–L4.
- **L5**: CI caught what local vet does not: the lint step (errcheck) rejected `tmp.Close()` unchecked on the settings write-error path — fixed to the codebase's `_ = Close()` idiom (launch.go precedent). Tests all green locally; Verify Go code re-runs on the push.
