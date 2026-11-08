# Visible Shuffle Cap (cycle C)

A visible 🔀 cap on the transport keycap row closes the #163 discovery gap:
keys-only shipped a correct-but-invisible control (the only discovery paths
were the README and the status message AFTER pressing z). Issue:
egkike/streambeats-player#172 (status:approved + type:feature; standing
grant "Dale al ciclo y veamos los mockups").

## Specs

- **S1** (verbatim owner): "Pero el boton de shuffle NO aparece entre los
  botones del control... no esta dibujado en pantalla" + "¿Como va saber un
  usuario que tiene una opcion de shuffle y con que tecla se activa?" — the
  6th cap 🔀 with hint `z` joins the row in BOTH render modes; `z` stays the
  keys.go single source of truth (untouched from #163).
- **S2** (mock A approved — owner "(a)"): emoji mode adds 🔀 z LAST (after
  volume), interior 3 + calibrated-icon + 1. Doctrine prediction for
  U+1F500 (EAW-wide, no VS16): paints 2 cells on the ghostty panel preset,
  same family behavior as 🔊 — the probe table entry carries 2, verified
  live by the owner after GREEN (the CPR ritual of #60: a mis-measured
  probe flips the table, never the frame math).
- **S3** (mock B approved): ASCII rebalance = ONE existing-cap change —
  asciiTokens["🔊"] "vol" → "v" (interior 9→7) — plus the new entry
  asciiTokens["🔀"] = "sh"; |< || >| [] and all key hints untouched ("pp"
  banned: misleading vs the real space key).
- **S4** (mock C approved): quiet theme inline line gains "sh z" from the
  same token table — no separate quiet copy.
- **S5** (guards): new TestKeycapsRowFitsAtWidth52 pins BOTH modes ≤ 52
  painted columns (stripANSI); the existing pins update coherently:
  TestKeycapWidthsMatchTheCalibratedTable (🔀 probe entry + plain-ASCII
  hint rule), TestKeycapsRowShowsTheApprovedPairs (6 caps), and the
  adaptive ASCII pin. No EAW-ambiguous glyph/token ever enters (the
  U+2423/±/● ban, issues #42/#60/#64).
- **S6** (invariants): pure view-layer + tables; no mpv surface, no queue
  engine change, no nav-ladder change (#146/#151), failure paths unchanged;
  quiet-theme bypass requirement holds.

## Artboard (approved)

- **A — emoji/ghostty 52**:
```
╭─────╮╭───────╮╭─────╮╭─────╮╭────────╮╭──────╮
│ ⏮️ p ││ ⏯️ spc││ ⏭️ n ││ ⏹️ s ││ 🔊 +/-  ││ 🔀 z  │
╰─────╯╰───────╯╰─────╯╰─────╯╰────────╯╰──────╯
```
- **B — ASCII 52**: `|< p` `|| spc` `>| n` `[] s` `v +/-` `sh z` (same frame
  geometry, token `sh` + the `v` rebalance).
- **C — quiet inline**: `|< p   || spc   >| n   [] s   v +/-   sh z`.

## Tasks

- **T1** (S1-S5): crowcap tables+tests — RED via the existing pin family
  (uncalibrated icon/pairs/ASCII widths), GREEN with table entries + `sh`
  + `v` rebalance + the ≤52 fit guard; pipeline (gofmt/build/vet/suite;
  race not applicable — no event-channel change). commit.
- **T2** (S6): docs + ODD log; native review of the committed candidate; PR
  `Closes #172` + exactly one `type:*`.
- Owner live check after merge (CPR confirm on the real panel; a frame
  1-cell drift = the #60 table-flip ritual, never code).

## Log

- **L1** (owner, verbatim): "Pero el boton de shuffle NO aparece entre los
  botones del control... no esta dibujado en pantalla" +
  "¿Como va saber un usuario que tiene una opcion de shuffle y con que
  tecla se activa?" plus "Deberíamos..." — no wait, the last quote belongs
  to the docs cycle. Rationale: agent confessed the keys-only compromise
  traded discovery for review workload — owner adopted the visible cap.
- **L2**: mockups computed from the shipped tables (keycapIconWidths,
  keycapInterior, asciiTokens); owner approved (a) — emoji last-position,
  `sh` ASCII token, single `vol`→`v` rebalance, quiet inline entry.
