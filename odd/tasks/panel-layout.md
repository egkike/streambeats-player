# Panel layout — bottom-anchored TUI (issue #31)

The panel window drops a large dead area below the player controls when the
focused list is shorter than the viewport, and the bottom block jumps when a
status message appears or clears.

## Specs

- S1 (user, verbatim): "El tamaño de la ventana creo que podría mejorar ya que
  actualmente es larga y por debajo de los controles queda un espacio vacío
  grande hasta el final. Incluyamos el mejoramiento de la ventana de la app
  ajustando tamaño y alguna otra cosa que consideres que se pueda mejorar"
- S2 (derived root cause): the list does not pad its viewport — `windowBounds`
  returns only existing rows when `n <= height`, so the content shifts up and
  the gap lands below the controls.
- S3: bottom-anchored layout: list occupies the flexible middle (padded with
  blank rows to its full viewport height); title/header top; player bar,
  status slot and help pinned to the bottom edge; the whole panel reads
  full-height with the controls always at the bottom.
- S4: reserved status slot: one line always allocated for status (blank when
  empty), so the bottom block never jumps when a message appears or clears.
- S5 (rewritten 2026-10-26, owner-validated visually): side presets
  (left/right) drop to `window-height = 24`; 36 left ~24% of vertical slack
  under the bottom-anchored layout for a catalog list. Top/bottom presets
  keep 30 (horizontal strips). chromeLines metrics unchanged (the render
  already paints m.height fully).
- S6 (negative controls): View side-effect free; pure-update tests only;
  help truncation, header text, playing/selected markers, list windowing and
  `chromeLines` semantics pinned; no keybind changes.

## Tasks

- T1 (S2-S6): view layout rewrite + view tests RED→GREEN. Commit.

## Log

- L4 (2026-10-26, owner visual validation + size decision): after PR #32 the
  owner still saw slack ("ahora sale con el mismo tamaño y los controles en
  la parte inferior... el espacio vacio queda en el medio ahora") — the layout
  worked as designed and the residual slack was the preset height itself
  (36 rows). Side presets set to 24; owner: "Ahora si... ya no sale el espacio
  vacio y la ventana es ajustada al contenido". Issue #33 carries the asset
  change (code lane, L-sidebar 36→24; top/bottom keep 30). User authorized this cycle in conversation (issue #31 opens directly
  `status:approved` per the AGENTS.md standing grant). Branch `feat/panel-layout`.

## Log

- L2: RED first: 3 tests failed exactly as diagnosed (0 padding rows between
  list and bottom block; status shifted line count 10↔11; 24-row panel
  rendered only 10 lines). GREEN rewrite exposed a bonus chronic bug: the old
  `chromeLines = 9` overcounted the help block (helpLines() returns 2), so
  every panel rendered exactly one line short — height-1 paint since the
  original scaffold; fixed to the true count 8 (title+sep+header+bar+status+help).
- L3 (delivered): layout bottom-anchored (listRows padded to viewport,
  playerBarLines split, reserved status slot, full-height paint verified:
  rendered == 24 for height 24). Pinned: help truncation, windowBounds,
  playing/selected markers via suite green (66 view/model tests). S4 status
  telemetry: statusAt = len-3 positional. Panel size unchanged (S5:
  window-height 36 holds: 27 usable list rows now genuinely used).
