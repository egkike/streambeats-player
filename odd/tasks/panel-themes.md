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
S9. Issue #143: the nav hint row paints within the painted width — for
    any panel width the rendered line (sans SGR) is ≤ that width; no
    wrap, no clipped tail. Owner report: "q qui se come la t final de
    quit (queda fuera de pantalla y solo se ve qui)".
S10. Tier ladder (issue #143): longest tier that fits — 55 (the
    approved line, double separators), 50 (single-space separators),
    47 (arrows dropped), 26 (essentials), floor "q quit"; vocabulary
    order preserved, no keycap playback tokens ever repeated (issue
    #60 S4 keeps holding).
S11. navHints() (no-arg) keeps returning the approved full line; the
    pinned byte-exact test keeps holding (S5's approved wording).
S12. Issue pending (handoff cycle, artboard 05 — owner-refined): the
    nav row repaints as: ONE dot separator between the move pair and
    the rest, keys rendered as badges (bold, Selected fg, theme band
    bg), actions dim. Owner words: "Yo pondría el · solo entre:
    ↑↓ k/j · enter ... el resto lo dejaría sin ·" — full tier 56
    runes, ladder recalibrates to 56/50/47/26/6 (tier 1 keeps the 50-
    single-space vocabulary; the old 55 tier disappears under the new
    full).
S13. Badge bands per theme, deterministic: quiet #1c2433 (its own
    band), cute #28121E, sexy #28121E (gentle-pi selection tokens,
    from the artboard spec card), vinyl #241b12 and synthwave #2a2350
    (their artboard border tokens as the design-language equivalent);
    legacy claro/oscuro/system render NO band — their row keeps the
    plain dim paint, changing only by the inserted dot (+1 rune).
S14. The badge SGR is zero painted width: painted byte counts and the
    width-fit contract of S9-S10 hold for every theme.

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
T5 (S12-S14): nav row repaint — single dot ladder recalibration +
    badged keys (nav-row-look/05 artboard handoff). Commit.

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
L11. Step 2 done: native committed-only review, lineage
     review-23685e2ccc38498c (tier medium, lenses readability +
     reliability, owner-authorized 2 runs). Result: approved;
     authority burned (gentle-ai.review-acknowledged/v1, store revision
     sha256:c68c8fd3...). One non-blocking advisory (R3-001, WARNING,
     informational, internal/tui/view.go:424-425) is recorded as
     separate later work — never a correction on this candidate.
L12. Step 3 done: pushed feature/panel-themes; PR #141 (Closes #140,
     type:feature) opened for owner review and squash merge. Steps 4-5
     (owner merge + sync) remain the owner's.
L13. Steps 4-5 done: owner merged PR #141 (merge commit 0bf2eff on
     main, 2026-11-05); merge-commit CI ran green (run 42101213584)
     before any further push — the release-shipping recipe evidence for
     this exact SHA. Release v1.11.0 pushed onto the sync tip 2609d22
     with owner authorization: release.yml green (run 42102305206),
     release published with 5 archives + checksums.txt; local install
     verified via scripts/install.sh --version v1.11.0.
L14. Owner field report after v1.11.0: the nav hint row clips its tail
     on side-anchored panels ("q qui"). Root cause: the row never had a
     painted-width budget; the S5 rename grew it 52 → 55 runes. RED
     tests on tier boundaries observed (after discarding a first RED
     run that measured the pre-existing boot status message instead of
     the nav row); GREEN ladder implemented (commit follows). Issue
     #143, status:approved via the standing grant (in-conversation
     bugfix cycle).
L15. Cycle #143 delivered: PR #144 merged by the owner (merge commit
     ae6532f, merge-commit CI green run 42105718211). Owner-authorized
     tag v1.11.1 annotated on that SHA: release.yml green (run
     42108506256), release published with the 5 archives + checksums.txt.
     The local install is the owner's to run (scripts/install.sh
     --version v1.11.1 applies, checksum-verified, idempotent).
L16. Handoff (design/nav-row-look/05-single-dot-badges.html, chosen;
     canvas alternatives 01-04 explored first): owner refined the nav
     row to one dot + badged keys. Feature doc extended with S12-S14
     and task T5; implementation cycle opened as issue first (issue
     #146, status:approved).
L17. T5 done (commit follows): ladder rebuilt as pairs with the
     single-dot full tier (56) — navRowFor paints badges (SGR zero
     width), navHintsFor/navRowText pin the exact tier text, badge map
     per S13 (quiet reuses its band; legacy trio stays bandless).
     RED observed on tier pins + badge band + navHints nav pinned
     line; suite green incl race and 56/55/50/… fit boundaries.
