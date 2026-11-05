# Panel themes expansion (issue #140)

Five new panel themes with per-theme structure, three settings-screen
sections, and the `c settings` hint rename. Legacy Claro/Oscuro/System
stay byte-identical.

## Specs

S1. Keep the existing themes as-is: "mantenemos las opciones de
    Claro/Oscuro/System como tenemos hoy" — `theme.Resolve` for
    `claro`, `oscuro`, `system` produces byte-identical output to the
    pre-change tree, including the pinned system-on-dark parity test.
S2. Five new themes `vinyl`, `synthwave`, `quiet`, `cute`, `sexy` whose
    palettes match the artboard spec tables exactly
    (design/streambeats-panel-look/01-vinyl-dark.html,
    02-synthwave.html, 03-quiet-ink.html, 04-gentle-cute.html,
    05-gentle-sexy.html). Palette roles stay the existing six:
    Title, Dim, Selected, Playing, Status, Bar.
S3. Per-theme structure, active only under its own theme id:
    - vinyl: dotted leaders between the label and the row tail
      (counts) in album and track rows.
    - synthwave: cyan second voice — » before track counts, a dot
      separator in the title line, cyan progress timestamps.
    - quiet: selected-row background band, boxless single-line
      transport, count column layout for album rows.
    Legacy themes never hit these branches.
S4. Settings screen renders three labeled dim section headers:
    ambient themes first (Claro/Oscuro/System), then "themes:" with
    the five new ids, then the existing "panel terminal:" section.
    One persisted field (`Settings.Theme`) remains the single source
    of truth; the `(current)` marker follows the model's theme id
    from either section. User words: "el usuario podría seleccionar
    lo que más le guste".
S5. The nav hint renames to "c settings": view.go navHints becomes
    "↑↓ k/j  enter open  esc back  l lib  c settings  q quit"; the
    screen title stays "Settings"; the pinned nav-hint test is
    updated. Owner quote: "Vamos con c settings".
S6. Workflow decision from the scope question: the question was
    answered "Paleta + estructura por tema" — each artboard drags
    its structure, always gated to the active theme (output changes
    only under the new themes; parity of the three legacy themes
    intact).
S7. Phase A invariant preserved: a theme switch is a pure state
    change — palette id + per-theme render branches only; no process
    spawn, no config writes, no terminal background repaint.
S8. Settings robustness intact: unknown ids resolve to the default
    without crashing; `claro`/`oscuro` remain valid persisted ids.

## Tasks

T1 (S2, S8): theme registry — five palettes, Lookup/Resolve entries,
    table tests. Commit.
T2 (S4, S5, S1): settings screen sections + theme rows + `c settings`
    hint + pinned-test updates. Commit.
T3 (S3, S6, S7): gated per-theme structure (leaders, cyan voice, band,
    boxless transport) + view tests + legacy-theme differential byte
    check vs pre-change HEAD. Commit.
T4 (S2): commit design assets under design/streambeats-panel-look/ and
    docs touch-ups; ODD log entries. Commit.

## Log

L1. User request (2026-01 cycle): improve the app's look (ghostty and
    kitty); use /gentle:design to explore alternatives before the
    owner picks one. Design exploration produced
    design/streambeats-panel-look/ (five artboards + manifest).
L2. User: keep Claro/Oscuro/System as they are today, add the five
    new themes in a separate settings section, so the user picks what
    they like; settings screen becomes three labeled sections.
L3. User: nav hint "c theme" should change title; decided "c settings"
    over `c config`/keeping `c theme` (screen already manages panel
    terminal since issue #90).
L4. Scope question asked (palette-only / palette+structure / hybrid);
    owner answered "Paleta + estructura por tema". Reconciled into
    S3 and T3.
L5. Issue #140 opened with status:approved (standing grant, issue #25
    model). Branch feature/panel-themes off main.
L6. T1 done: commit 1c2c80f (feat(theme): register five panel themes
    with artboard palettes). Registry tests RED->GREEN; pipeline
    (gofmt/build/vet/test/tidy) green before committing.
L7. T2 done: commit bcfbe78 (feat(tui): settings sections, artboard
    theme rows, c settings hint). Three RED tests observed before
    implementation; two historical terminal-navigation pins and the
    pinned nav-hint test updated to the approved new cursor map.
L8. T3 done: commit b2be093 (feat(tui): per-theme row structure).
    Four RED tests observed; legacy differential byte check ran in both
    trees (throwaway dumper, uncommitted): claro/oscuro/system View()
    output identical against bcfbe78 across catalog/config/tracks
    states. One catch fixed: the legacy default album format kept the
    artist suffix; vinyl also carries it per the artboard.
L9. Work-unit slice boundary: the three T1-T3 commits total 618
    authored lines across the ~400-line advisory; each is a separate
    reviewable behavior unit (registry / screen / structure).
L10. T4 done: commit 2ce18f5 (docs(odd): design assets, README theme
     docs, log entries). Four work-unit commits hold the whole feature
     cycle: 1c2c80f, bcfbe78, b2be093, 2ce18f5. Next: RDD native review
     on the committed candidate, then push + PR referencing the burned
     receipt.
