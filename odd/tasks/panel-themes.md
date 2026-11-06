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
S16. Issue #149 field cycle: the tier-0 dot paints in `palette.Status`
    (distinct hue from the badge keys, high contrast on the dark
    surface) instead of `Bar`; painted width unchanged (56 runes full
    tier, stripANSI keeps the dot). Owner field report: "en terminal
    ghostty no sale el · entre ↑↓ k/j · enter" — the dot bytes were
    pinned present; this is the visibility fix.
S17. Issue #149 field cycle: a 43-rune tier lands between 47 and 26:
    `enter open esc back l lib c settings q quit`, single-space
    separators, badges included. Ladder becomes 56 / 50 / 47 / 43 /
    26 / 6; widths 43-46 paint the new tier, ≤42 fall to essentials.
    Owner field report: "en terminal kitty solo salen estos controles:
    enter open esc back q quit. Parece haber menos espacio en kitty
    pero mejor verifica" — the kitty panel paints ≤42 usable columns
    (denser cell grid at the same quick-terminal geometry).
S18. Regression guards (issue #149): the #143 fit contract passes at
    the new boundaries; badge band/no-band pins unchanged; legacy
    trio keeps no-band + only the dot.
S19. Issue #151 (compact full tier): `↑↓ k/j · enter open esc back l
    lib c settings q quit` = 52 runes, single spaces elsewhere; the
    56/50 tiers dissolve; ladder 52/47/43/26/6; owner quote: "El 60
    test-preset aplica" / field evidence cols=60 at 560px.
S20. Issue #151: the #143 fit contract passes at 52/51/47/46/43/42;
    painted-count pins 56 → 52; navHints() = compact text.
S21. Issue #153: shipped kitty left/right presets use `columns 560px`
    (top/bottom keep `lines 324px`); docs/kitty.md mapping reflects it.
S22. Issue #153: the mapping table keeps the owner-tunables doctrine.
S23. Issue #153: passive config + docs (no applicable RED; structural
    grep verification; field evidence cols=60 + the rendered dotted
    row with Status dot + badges).
S24. Issue #156 docs cycle (README Use): the auto-rebuild doctrine is
    `streambeats-quick`'s — direct binary runs (`kitten …`, `go run …`)
    use the LAST BUILD; the rebuild command is documented. Owner Nov 5
    field session hit the silent stale binary ("c theme" era).
S25. Issue #156: docs-only change — no behavior or tests touched.
S26. Issue #158 (handoff, artboard long-title-truncate/02 — owner
    choice): overflow album rows drop the artist chunk first
    (least-unique information, matches the artboard overflow row),
    then middle-truncate the TITLE (head ~3/5 + `…` + tail ~2/5 of
    the name) so the count tail always paints. Non-overflow rows stay
    byte-identical.
S27. Issue #158: track rows reserve the duration tail and
    middle-truncate the title on overflow, same helper.
S28. Issue #158: the vinyl dotted-leader branch keeps its separators;
    a too-long head middle-truncates before ` … ` + count (the tail
    never falls to the generic truncate).
S29. Issue #158: `truncateMiddle(s, budget)` lives with the pure
    helpers (head ~3/5, `…`, tail ~2/5; table-tested); the generic
    `truncate` stays as the final row guard.
S30. Issue #158: settings-screen and library rows keep today's plain
    truncate (out of scope); ladder/badge/byte contracts untouched;
    quiet's fixed `%-24s` column truncates within the column.

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
L18. Post-merge (PR #147, merge d2ab8d4, CI green) advisory
     verification, owner-requested, empirical via throwaway probes:
     (a) R3-001@view.go:424-425 (themes review): long-title rows clip
     the count tail with "…" IDENTICALLY across synthwave/quiet/legacy
     — shipped truncate policy, uniform, no defect (pinned bytes were
     not at risk); a keep-the-tail-truncate-the-middle policy remains
     an optional owner-driven future design decision. (b)
     R2-nav-floor-overflow + R3-001@view.go:744-746 (nav fix review):
     the nav floor paints 6 runes at panels narrower than 6 columns
     only — a degenerate domain documented deliberately (S10), reached
     by no realistic panel; NOT absorbed by #146's rewrite, and no fix
     is warranted. Both advisories verified-informational, no fix
     cycle; observe receipts stand as approved.
L19. Owner field test on release v1.12.0 (PR #147 cycle): ghostty does
     not SHOW the dot (bytes pinned present — visibility problem: the
     Bar-tone dot is ~4:1 on the dark surface between bright badges)
     and kitty compresses to the 26-rune essentials tier (panel paints
     ≤42 columns — denser cell grid at the same quick-terminal
     geometry). Fix cycle opened as issue #149 (status:approved,
     standing grant): new specs S16 (dot hue -> palette.Status),
     S17 (43-rune tier, ladder 56/50/47/43/26/6), S18 (regression
     guards). Release for this cycle: v1.12.1 (fix).
L20. T6 done (issue #149 field fix): RED observed on the 43-tier pins
     and the Status-hue dot; GREEN = navPairsLibrary tier between 47
     and 26 (ladder 56/50/47/43/26/6), dot rides statusStyle (S16).
     Full pipeline green incl race; the cute full row still paints
     exactly 56 runes with the dot (zero-width SGR, S14/S18). Release
     v1.12.1 carries this fix for the owner's retest.
L21. Review receipt: native committed-only review lineage
     review-ce51aa268860090e (medium, lenses readability + reliability,
     owner-authorized 2 runs) — approved, authority burned (store rev
     sha256:f9983464...595c). One non-blocking advisory
     (R2-kitty-tier-mismatch, readability, odd/tasks/panel-themes.md:86,
     WARNING/informational): recorded as separate later work — likely
     the doc's kitty-boundary wording vs the true panel geometry, to
     verify with the owner's retest numbers.
L22. Owner retest of v1.12.1 (verbatim: "En ghostty sigue sin salir el
     punto... En kitty solo sale esto: enter open esc back l lib c
     settings q quit. Creo que acá ya no hay espacio") corrected the
     #149 diagnosis: the ghostty side panel paints ~51-55 columns —
     BELOW the 56-rune full tier — so the dot never rendered; the hue
     change never executed. The kitty panel renders exactly the
     43-rune library tier (working as shipped; resolves the
     R2-kitty-tier-mismatch advisory observation with real geometry).
     Owner choice: "Implemento el tier de 52". Issue #151 opened
     (status:approved): S19 compact full tier (52 runes, dot between
     move pair and rest, singles elsewhere), S20 boundary guards,
     ladder 52/47/43/26/6. Release v1.12.2 carries it.
L23. T7 done (issue #151): RED observed on the compact-tier pins, both
     painted-count pins (56 -> 52) and the navHints() pin; GREEN =
     full-tier separator run "  " -> " " in navRowFor/navHintsFor and
     the standalone 50-rune arrows tier dissolved (S19). Ladder is now
     52/47/43/26/6; the cute full row paints exactly 52 runes with the
     Status dot (zero-width badge SGR). Full pipeline green incl race.
     Release v1.12.2 carries it for the owner's next retest.
L24. Review receipt (issue #151): lineage review-6bbe1a3626e78c26
     (medium, readability + reliability, owner-authorized 2 runs) —
     approved, authority burned (store rev sha256:f1c20713...ede6);
     no advisories this time.
L26. Owner choice flush with the kitty field test at 560px (cols=60
     measured with tput inside the panel; full row with Status dot +
     badges rendered): wide the shipped kitty left/right presets to
     560px so the tier-52 nav row (S12/S19) fits both terminals.
     Issue #153 (status:approved, standing grant — owner "Dale con el
     A"). New specs S21-S23; no applicable RED (passive config value
     + docs; no unit harness for pixel widths).
L27. Merged-and-shipped record for the field-fix arc: PR #150 merged
     (76e7064, CI 42123504411; receipt review-ce51aa268860090e burned,
     store rev sha256:f9983464...); PR #152 merged (e3319c5, CI
     42127468311; receipt review-bcc30909b8d6a7e9 burned on the exact
     final tree, store rev sha256:a684f9c9...; the tab-indented Closes
     in its first body left #151 unlinked — fixed by editing the body,
     verified via the issue timeline cross-reference). PR #154 merged
     (14bbd0a, CI 42132593126; receipt review-5e9d1cab2a0c04db burned,
     store rev sha256:f5a256c3...; issue #153 auto-closed).
     Release v1.12.2 tagged on 14bbd0a and published green (run
     42132829087). Field validation recorded: kitty width test preset
     560px measured cols=60 with the full dotted row + badges; the
     shipped 424px strips ladder to the 43-rune tier by design.
L28. Owner retest remains the last arc step: install v1.12.2, then the
     kitty daily panel (streambeats-quick --terminal=kitty right, now
     560px) should render the complete dotted row; the cleanup of the
     owner's local kitty-right-test.conf is optional (their call).
L29. Docs cycle (issue #156 → PR #157): the auto-rebuild note landed in
     the README Use section ("direct binary runs use the LAST BUILD");
     owner merged f996cf67 (CI 42135642005 green); issue #156 auto-
     closed (Closes at line start verified BEFORE the first merge attempt).
L30. Handoff received (design/long-title-truncate/02-middle-ellipsis.html,
     owner choice over Tail keep): middle-ellipsis truncation implemented
     as issue #158 (status:approved, standing grant); specs S26-S30 +
     spec addendum S19-S25 reconciled into this doc. Implementation
     follows (RED first).
L31. T8 done (issue #158): RED observed on the truncateMiddle table
     (42/30/2 budgets) and the gated overflow rows (all four themes:
     count tail died — today's bug); GREEN = truncateMiddle pure
     helper (head 3/5, …, tail 2/5, ellipsis-only edge at keep=1) +
     budget-first wiring: default drops the artist then middle-
     truncates the title; synthwave/quiet reserve their tails; vinyl
     heads middle-truncate before " … " + count. Two test fixes
     during GREEN: edge-case literal pinned as ellipsis-only, and the
     quiet count column assertion (bare number by design). Pipeline
     green incl race and tidy; short-title rows byte-identical.
     Release v1.12.3 after merge (owner's call).
L32. Review receipt (issue #158): lineage review-5ea81069e2ab350c
     (medium, readability + reliability, owner-authorized 2 runs) —
     approved, authority burned (store rev sha256:f6519370...170a).
     Two non-blocking advisories recorded as later work (R2-001
     SUGGESTION view.go:466, R3-001 WARNING view.go:661).
