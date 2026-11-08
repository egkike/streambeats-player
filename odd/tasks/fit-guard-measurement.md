# Fit-guard measurement (issue #175) — test-only refactor

Lights a test-only rebuild of the keycap-row fit guard: painted-cell truth from the
calibrated table, content-anchored block, loud failures. Zero production surface.

## Specs

- **S1**: the fit guard of the keycap row pins the PAINTED width measured from the
  calibrated single source (`keycapIconWidths`, issue #60 round-6 doctrine), never a
  rune count. (Re-derived R2-001/R3-001 anchors, receipt review-6ce1c996d3fb5dec.)
- **S2**: the row block is found by CONTENT, never by a positional `rows[20:23]`
  slice; a drifted anchor is a loud failure, never a silent skip or no-op guard.
- **S3**: any residual variation selector (FE0F, uncalibrated token) inside the
  keycap frame is a hard failure — production would paint it 2 via the
  `keycapIconWidth` fallback; counting it 1 is exactly the undercount trap.
- **S4**: production tables, tokens, keys and live cross-host behavior untouched;
  the full suite stays green and the local pipeline passes.

## Tasks

- **T1** (S1-S3): throwaway RED demonstration (rune-max silently passes a 53-cell
  line built on the shipped 🔊 family), then shipped helpers `keycapBlock` +
  `paintedWidth`; `TestKeycapsRowFitsAtWidth52` rewritten content-anchored +
  painted-cell; two pin tests (`TestPaintedWidthObeysTheCalibratedFamily`,
  `TestKeycapBlockIsContentAnchored`). Route: inline; test-only; race n/a.
- **T2** (S4): docs + local pipeline + native RDD review of the committed
  candidate + PR `Closes #175` with exactly one `type:*` (refactor); owner merges.

## Log

- **L1** (owner, verbatim): "Creo que ya hicimos esos advisories informativos,
  verifica" → verification result: NOT done (no commit touched view_test.go after
  the cycle; no follow-up issue). Then: "Dale al fix ahora" (cycle authorized).
- **L2**: issue #175 opened with the re-derived findings; the burned receipt text
  is consumed (transaction store keeps the pointer only), so anchors live in
  odd/tasks/visible-shuffle-cap.md L3 + issue #175.
- **L3** (T1 evidence, 427e-style): throwaway RED observed correctly — the legacy
  rune-max measure reported 52 for a synthetic 53-painted-cell line built on the
  shipped 🔊 family (silent-pass class, not committed). GREEN: helpers
  `keycapBlock` (content anchor: ⏮️ OR `|<` middle line, ╭/╰
  neighbors) + `paintedWidth` (calibrated token cells; refuses residual FE0F);
  `TestKeycapsRowFitsAtWidth52` rewritten table-driven over both modes (both
  assertions now always execute); pin tests
  `TestPaintedWidthObeysTheCalibratedFamily` +
  `TestPaintedWidthRefusesUncalibratedSelectors` + `TestKeycapBlockIsContentAnchored`
  (quiet no-box view must not anchor). First run surfaced two honest draft
  mismatches that failed loud: the ASCII-mode token needed its own anchor (`|<`)
  and the ASCII case counts 8 (rune-exact), both fixed. Pipeline:
  gofmt empty, build/vet clean, FULL suite green -count=1, tidy -diff empty.
