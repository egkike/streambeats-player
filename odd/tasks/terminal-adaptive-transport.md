# Terminal-Adaptive Transport (issue #64)

Solo Teclado keycaps are calibrated for ghostty 1.3.1 (legacy grapheme-width
method). On any other terminal the emoji glyphs paint at standard widths and
the hand-built frames misalign. This cycle renders pure-ASCII transport tokens
outside ghostty and keeps the calibrated keycaps byte-identical inside it.

Specs quote issue #64 verbatim. Branch: `feat/terminal-adaptive-transport`.

## Specs

- S1: "Ghostty (detected via `TERM_PROGRAM=ghostty`): keep exactly the current
  calibrated keycaps — no visual change, existing pins hold."
- S2: "Any other terminal (detection fails closed to ASCII): pure-ASCII tokens
  only, width-stable by rune count on every terminal — the same doctrine as
  the pre-VS16 rounds."
- S3: "`keycapIconWidths` remains the single width source on ghostty; the ASCII
  path derives widths by rune count (no emoji, no VS16, no uncalibrated
  glyphs)."
- S4: "Bindings and semantics are untouched: this is a rendering-layer change
  only."
- S5: "Ghostty rendering must stay byte-identical to today (existing calibrated
  tests keep passing)."
- S6: "The ASCII token set must contain no emoji or VS16 sequences; every width
  must equal its rune count."
- S7: "With `TERM_PROGRAM` unset or foreign, the app must render ASCII (stable
  widths) — never the ghostty-calibrated table."
- S8: "The detection runs once at launch (env read in `Launch`), never inside
  the render path."

## Tasks

- T1 (S5,S6): RED tests — ASCII token invariants (no emoji/VS16, width =
  rune count) and frame closure in ASCII mode.
- T2 (S7): RED test — detection helper: ghostty → calibrated mode, unset or
  foreign value → ASCII mode.
- T3 (S1,S3): GREEN — mode-aware width/interior/keycaps path; the calibrated
  table stays ghostty's single source.
- T4 (S8): GREEN — `isGhosttyTerminal` env detection read once in `Launch`,
  stored on `Options`/`Model`; render path never reads the env.
- T5 (S4,S5): guard — existing calibrated tests pass untouched; render-layer
  only (no binding/semantics changes).
- T6: pipeline (gofmt, build, vet, test) + work-unit commits.

## Log

- L1 (owner, 2026-10-27): "Se puede hacer que cuando sea un terminal no-ghostty
  se use tokens ASCII puros y cuando sea Ghostty mantener lo que tenemos?" —
  authorized as issue #64 with `status:approved` (standing grant, owner
  proposed and authorized this cycle in conversation 2026-10-27).
- L1b (2026-10-27): previous session crashed before any write; cycle restarted
  from a clean `main` (4e7825f) with no residual branch or commits.

- L2 (2026-10-27): cycle implemented RED→GREEN. Tokens: |<, ||, >|, [], vol
  (rune-count widths 2/2/2/2/3; interiors 6/8/6/6/9, frames 8/10/8/8/11).
  `isGhosttyTerminal` fails closed (exact "ghostty"); env read once in
  `Launch` into `Options.AsciiTransport` → `Model.asciiTransport`; the
  render path never reads the env (S8). `keycapIconWidths` untouched —
  ghostty's single width source (S3). Existing calibrated pins pass
  unmodified (S5 byte-identical). Pipeline: gofmt clean, build ok, vet ok,
  full suite ok. Commits: f6df93b (feat, code+tests), 3b27d3f (docs odd).
- L3 (2026-10-27): RDD native review receipt — lineage
  review-b77fe430119bd2ef (medium tier, 2 model runs owner-authorized:
  readability order 0 + reliability order 1), outcome APPROVED with zero
  corrections; authority burned (gentle-ai.review-acknowledged/v1,
  consumed_revision sha256:1553ef39…). Delivery follows ordinary
  repository policy.
