# keycap-controls — Solo Teclado panel (keycap transport + status-dual slot)

Keyboard-only panel: mouse dispatch removed end to end, the 5 transport buttons
become a visual keycap row, and the two help lines are replaced by a dual-purpose
status slot. Owner approved design Alternativa A + Alt 3 in conversation.

## Specs

- S1 ("La desición es que vamos a hacer una versión Solo Teclado y nada de mouse para los controles y lista de musicas."):
  mouse input is inert EVERYWHERE (controls AND music lists): a `tea.MouseMsg`
  changes no playback/list state and produces no command. No mouse reporting is
  enabled at program start; bubblezone machinery (NewGlobal/Close, View scan,
  row marks, zone dispatch) is removed together with the dependency.
- S2 ("Los 5 botones debajo de la barra los podemos quitar y dejar en su lugar los controles con las Teclas (letras) y su icono como una cuestión visual."):
  the transport button row is replaced by a keycap row; the keycaps are visual
  only and show REAL bindings (⏮️ p / ⏯️ ␣ / ⏹️ s / ⏭️ n / 🔊 ±) — no keycap
  advertises a key the TUI does not handle (p/n are bound aliases of < >).
- S3 ("Haz un diseño lindo y estético de la zona de controles con letras + iconos. Primero muestrame alternativas de cómo quedaría y luego yo te digo cual me gusta."):
  Approved design = Alternativa A "Keycaps": rounded lipgloss borders, one cap
  per transport action, icon + key inside:
      ╭──────╮  ╭────────╮  ╭─────╮  ╭──────╮  ╭──────╮
      │ ⏮️ p │  │ ⏯️ ␣ │  │ ⏹️ s │  │ ⏭️ n │  │ 🔊 ± │
      ╰──────╯  ╰──────╯  ╰─────╯  ╰──────╯  ╰──────╯
- S4 ("Me gusta la opción A pero mejoremos esta zona: k/↑  j/↓  enter open  esc back  l lib  c theme / space ⏯  s ⏹  < > ⏮⏭  +/- 🔊  q quit ... creo que repetimos botones y podría tener mejor diseño tambien. Veamos alternativas"):
  the two help lines are REMOVED; no key or action is advertised twice. The keys
  shown in the keycaps (playback) never appear in the navigation hint line.
- S5 ("Vamos con A y Alt 3"):
  Approved navigation design = Alt 3 "Status dual": the reserved status slot
  shows feedback when there is a message, else the navigation hints
  (`↑↓ k/j  enter open  esc back  l lib  c theme  q quit` — nav keys only).
  Total panel height unchanged (24 rows at the standard panel size).
- S6 ("Las letras e iconos no cuadran bien en los botones, falta ajustar eso"): every keycap token must be width-STABLE between ghostty's grapheme-width-method choices. This amends the S2/S3 mockup spelling: ␣ (U+2423) and ± (U+00B1) were EAW-ambiguous, and the measured/painted widths disagreed on the owner's device, moving the inner borders off the frame. Cap tokens are now only emoji+VS16 (2 cells, validated in v1.5.2's button row) and ASCII (1 cell): `⏮️ p / ⏯️ spc / ⏹️ s / ⏭️ n / 🔊 +/-`. A package guard test pins every cap rune to those classes.

## Tasks

| ID | Specs | Route | Commit |
|----|-------|-------|--------|
| T1 | S1 | mouse dispatch off + zone machinery out (model.go, launch.go, view.go, go.mod) | f1ea305 |
| T2 | S2-S4 | keycap row + status-dual slot pinned by view tests | f1ea305 |
| T3 | S1-S5 | model_test.go surgery repaired (clickModel refs, mouse tests → keyboard-only guards) | f1ea305 |
| T4 | S5 | pipeline green (gofmt/build/vet/test + race tui; golangci-lint 0 issues) | f1ea305 |

## Log

- L1: user request (verbatim): "La desición es que vamos a hacer una versión Solo Teclado y nada de mouse para los controles y lista de musicas. Los 5 botones debajo de la barra los podemos quitar y dejar en su lugar los controles con las Teclas (letras) y su icono como una cuestión visual. Haz un diseño lindo y estético de la zona de controles con letras + iconos. Primero muestrame alternativas de cómo quedaría y luego yo te digo cual me gusta. Te parece bien así?"
- L2: design approved (verbatim): "Vamos con A y Alt 3" — Alternativa A (keycaps) + Alt 3 (status dual). Evidence of the approved mockups is quoted in S3/S5.
- L3: previous session died mid-surgery ("Te colgaste y tuve que salir y volver a entrar"); the tree was left on feat/keycap-controls with keycaps/Alt-3 implemented but `model_test.go` referencing the deleted `clickModel` helper (5 refs) and bubblezone remnants (launch.go options, view.go marks, TestMain arming) — this session resumes clean-up, tests, and pipeline from that state.
- L4: lint flagged 6 unused functions after the mouse removal: the jukebox click-path (jukeArm/jukeNext/jukePrev/albumAt/jukeRestartLibrary/jukeRestartCatalog — wrap/autoplay semantics that existed ONLY for the removed buttons) was deleted whole (103 lines). The keybinds keep the classic queue semantics, deliberately unchanged. Library: issue #60 opened pre-approved (status:approved + type:feature, standing grant); commit slice f1ea305 (169+/566−, single keyboard-only work unit; the ~400-line advisory counts mostly deleted dead code, PR will carry size:exception).
- L5: RDD native review COMPLETE on the committed candidate (target sha256:6548cd0a…, base 1fd56f45, committed-only): lineage review-1d10da7c4f8edaef, tier medium, 3 host-relay reviewer runs authorized by the owner (review-risk order 0, review-readability order 1, review-reliability order 2), 3/3 submitted, state approved, correction budget untouched (no corrections opened). Receipt burned: acknowledgement consumed revision sha256:6a9efef5… (gentle-ai.review-acknowledged/v1). One non-blocking advisory finding stands as later work: R2-001 (lens readability, internal/tui/view.go:126, WARNING, informational). Delivery (push/PR/merge) remains the owner's decision per ordinary repository policy.
- L6: owner reported the caps misaligned on-device (verbatim: "Las letras e iconos no cuadran bien en los botones, falta ajustar eso"). Root cause: ␣ (U+2423) and ± (U+00B1) inside the caps are EAW-ambiguous — the width depends on ghostty's grapheme-width-method, which Go cannot know (round-3 doctrine). Fix 81b0492: tokens swapped to unambiguous ASCII (⏯️ spc, 🔊 +/-), package guard test TestKeycapContentsAreWidthSafe added FIRST (RED observed on U+2423, then GREEN); S2/S3 ammended into S6; new pins aligned. Pipeline green (suite + race + lint 0).
