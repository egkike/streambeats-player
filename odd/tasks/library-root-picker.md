# Library Root Picker (dedicated mode from settings)

The library root becomes editable in the TUI through a dedicated folder-picker
mode that reuses the lazy-navigator building blocks. Specs landed as a mock
BEFORE code (owner instruction: "lo trazamos como artboard/spec antes de
codear"). Issue: egkike/streambeats-player#164 (status:approved + type:feature).

## Specs

- **S1** (owner arrival, verbatim quote of yesterday's queue): "Library root
  editable desde settings: la línea read-only al final pasa a factibilidad de
  navegación (enter → elegir carpeta, o el UX más conveniente)." Resolved form:
  a dedicated `ModeRootPicker`, NO inline rows on the settings screen (budget:
  15 list rows today 24-row preset − chromeLines 9, currently 12 + panelOffer
  lines; `padRows` only pads, no clip).
- **S2** (owner, verbatim): "Para el A — Picker quiero que la navegacion se haga
  con j/k y Flechas Up/Dwn (igual que en el resto de la app), con enter sobre
  una carpeta se entra a las sub-carpetas, con u sube a la carpeta padre, con
  esc vuelve al menú settings y con o fija la carpeta." Frozen extras: base =
  $HOME (owner: "Desde $HOME"), set key separate (owner: "Tecla aparte para
  'set'") = `o`.
- **S3** (owner, verbatim): "No sería mas simple que se pinte de un color
  diferente la carpeta root y cuando le das o sobre la carpeta seleccionada/
  posicionada se pinte esa carpeta y la anterior se despinte, no creo que sea
  necesario entrar a la carpeta con enter para luego fijarla." Frozen semantics:
  `o` fixes the CURSOR folder; when the listing is empty, `o` fixes the CURRENT
  directory (header path). Root marking is COLOR-ONLY from a theme-registered
  hue (SGR adds zero painted width, nav badge pin #146); zero new glyphs (the
  ● glyph is East-Asian-Ambiguous — the exact trap that burned keycaps, issues
  #42/#60/#64). The paint DERIVES from `m.libraryRoot` (single source of
  truth): unmark is free by construction, no sticky paint state.
- **S4** (scenario rules — owner's proposal, refined with owner approval):
  owner verbatim: "1- La carpeta No tiene ningún sub-carpeta (Se puede fijar
  con o pero el enter no le permite entrar y da mensaje de vacío o no hay
  sub-carpetas), 2- La carpeta Si tiene sub-carpeta (Se puede entrar con enter
  y fijar con o), 3- La carpeta fijada con o No contiene archivos de músicas
  (Se permite fijar con o la carpeta pero se notifica que no contiene archivos
  de músicas, para que el usuario sepa que deberá cargar archivos de músicas en
  esa carpeta)." Frozen decisions by owner selection:
  - Scenario 1 → "Descender + enseñar": enter descends; empty listing renders
    the teaching status `no sub-directories here — o fixes this folder as the
    root`. (One rule instead of an enter-block branch.)
  - Scenario 3 → "Solo avisa si está vacía de verdad": plain `(saved)` when the
    target has audio files OR only subdirectories (album layouts are VALID
    roots — the lazy navigator descends into them); appends
    `— empty folder (music comes via sync)` only when no audio AND no subdirs.
  - Read failure (`o` target unreadable) → NOTHING persists, honest status
    (failure paths mutate NOTHING).
- **S5** (`o` persistence flow): one pre-save `catalog.ListDir` validates
  (exists/readable) and classifies (files/subdirs) in the same call — zero extra
  IO. Success persists FULL-SET via the existing atomic `saveSettingsCmd`
  (theme/libraryRoot/panelTerminal, store R1-001 discipline). Status honest and
  expiring, same slot as every other status.
- **S6** (settings affordance): the read-only row copy becomes
  `library root: <path> (enter to change)` — SAME single row, same budget;
  enter on it opens the picker; esc inside the picker returns to Settings with
  settings.json untouched and no status side-effect on data.
- **S7** (guard shifts): the picker's ascend clamps at `$HOME`; the LIBRARY
  navigator's clamp stays at the (possibly new) `libraryRoot`. After a fix, the
  library view refetches from the new root on next open (lazy, no recursion,
  dot entries stay filtered — #88).

## Artboard (mock approved by owner)

- **A** — picker at $HOME: same panel skeleton (title/sep/header/list/bar/
  keycaps/status), only header (`library root — <cwd>`) and listing change:
  dirs only with `/` suffix, cursor ▸, 14 rows + scroll. Footer hint
  (`j/k ↕ · enter entra · u sube · o fija · esc vuelve`) ≈ 45 runes ≤ 52.
- **B** — after `o` on a folder: honest status `library root: <path> (saved)`
  (+ empty-folder append per S4).
- **C** — read failure: `can not read <path>: permission denied`, nothing
  persisted.
- **D** — back in Settings (D): row copy per S6; same row count.

## Tasks

- **T1** (S1, S2): ModeRootPicker state + keys (j/k/arrows, enter, u, o, esc) +
  $HOME base + clamp — RED→GREEN with a fake fs (ListDir fake; NO real
  filesystem cache, network or mpv).
- **T2** (S3, S7): view — dirs-only listing with `/`, cursor, root color
  marking (derived paint), teaching status on empty listing.
- **T3** (S4, S5): 'o' flow — pre-save ListDir validate+classify, full-set
  atomic persist, status variants (saved / saved+empty / failure-no-persist).
- **T4** (S6): settings row copy `(enter to change)` + enter affordance into the
  picker (configKey routing), budget pins intact.
- **T5**: discriminative checks: gofmt/build/vet/test/tidy clean; suite; ODD
  log; native RDD review on the committed candidate; PR `Closes #164` with
  exactly one `type:*`.

## Log

- **L1** (session original): yesterday's queue point 2 verbatim (quoted in S1)
  plus the planning-trace instruction (mock/spec before code, respected by this
  doc's Artboard + Tasks order).
- **L2** (evidence): settings screen evidence re-derived: libraryRoot is
  rendered read-only (`view.go:338-341`), persisted-but-never-mutated
  (`model.go:815/840`), no setter exists on Options.LibraryRoot (captured at
  construction, `model.go:225`). Navigator blocks reused: ListDir dirs-first
  lazy (`musicdir.go:68`), descend/ascend (`model.go:672/683`), mode-switch
  pattern (`toggleMode`, `configReturn`).
- **L3** (owner iterations, in order): (1) mock A-D approved with corrections —
  navigation j/k + arrows; enter descends; u ascends; esc returns to Settings;
  o fixes the folder (S2). (2) set-on-CURSOR semantics chosen with derived color
  marking (S3, replacing the enter-then-o draft). (3) scenario rules chosen:
  descender+enseñar for 1, vacía-de-verdad for 3 (S4). No S# reopened by the
  refinements: S2/S3/S4 absorb them.
