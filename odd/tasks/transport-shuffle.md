# Transport Shuffle (single new transport control)

Queue shuffle as the ONLY new transport control this cycle, keys-only (no visual
row change). Issue: egkike/streambeats-player#163 (status:approved + type:feature,
standing grant model).

## Specs

- **S1** (owner, custom reply): "Vamos a meter Solo 1 boton más... veamos cuál es
  el más conveniente de agregar: shuffle, repeat u otro" → resolved by selection
  "Confirmo shuffle": shuffle is the single new control for this cycle.
- **S2** (owner): "Keys-only este ciclo" — the 5-cap keycap row keeps painting its
  current 44 columns (emoji) / 46 (ASCII) at width 52; no caps are added, no
  recording/calibration pins change.
- **S3 (shuffle semantics)**: Fisher-Yates over the REMAINING queue (Go-side,
  `internal/tui` model), never the playing position. No mpv IPC commands are added
  (`playlist-shuffle` upstream is useless here: the mpv playlist holds ONE entry
  because every track is a `loadfile replace`). The new keybind joins
  `internal/tui/keys.go` — the single source of truth — and routes through the
  same update loop as `queuePrev`/`queueNext`.
- **S4 (failure & idle behavior)**: shuffling during idle/stop mutates nothing.
  Shuffle is precomputed and committed on success only (S10 doctrine: failure
  paths mutate NOTHING). The idle-active observation (777) is untouched.
- **S5 (out of scope)**: repeat (track loop via `loop-file=inf` through
  `SetProperty`) is a FUTURE micro-issue, deliberately not this cycle.

## Tasks

- **T1** (S1, S3): shuffle unit in the tui model + `keys.go` entry + dispatch. **DONE** (b753bf3→bff7caa: `shuffleRemaining(rng)` with seeded rng, dispatch routes 'z' with a per-press seeded rng; signature returns concrete (Model, tea.Cmd) like its queueNext/queuePrev siblings).
- **T2** (S3, S4): RED→GREEN tests. **DONE** (RED observed as 4× undefined `shuffleRemaining` build failures; GREEN 6-test family: multiset+order-change+grouping never crosses albums, idle/empty-future no-op with untouched data, library deque, dispatch routes 'z'; race detector clean; suite gofmt/vet/test/tidy clean).
- **T3** (S5): docs: README keys sentence line + ODD log, native RDD review, PR
  `Closes #163` with exactly one `type:*` label.
- **Docs deferred** (S2): visible 🔀 cap on the keycap row = separate future
  issue (interior rebalance touches #60/#64 calibration pins — review workload
  kept out of this cycle on owner instruction).

## Log

- **L1** (session original, verbatim): yesterday's queue — "1. Botones de
  transporte: shuffle + el control que falte → mañana investigamos los comandos
  mpv desde DOCS/man/ipc.rst (shuffle/random, loop, quizás cola), y con el ojo
  puesto en las dos invariantes que va a tocar: la calibración de anchos de
  emoji (issues #42/#60/#64) y el presupuesto del ladder 52/47/43/26/6 si la
  fila cambia." Planning corrections (evidence): (a) DOCS/man/ipc.rst is NOT
  vendored here — upstream mpv-player/mpv DOCS/man/input.rst used as the mpv
  source of truth; (b) the 52/47/43/26/6 ladder governs the nav-hint row
  (`view.go:845-860`), NOT the transport row — the transport risk from the
  ladder is nil; the real constraint is the keycap row budget (44/46 painted
  cols at 52, a 6th cap does not fit) resolved by owner as keys-only (S2).
- **L2**: owner picked the single control by recommendation table
  (shuffle/repeat/seek comparison; shuffle recommended and confirmed — S1),
  keys-only confirmed (S2). Repeat deferred (S5), visible-cap rebalance deferred
  (S2 note in Tasks). No S# reopened.
- **L3** (T1/T2 evidence): work-unit commit **bff7caa** — 103 lines in `model.go`
  (helpers `shuffleSlice[T]`, `shuffleCatalogQueue`, `shuffleLibraryQueue`,
  `Model.shuffleRemaining`; status counts the FUTURE tracks: "shuffled N tracks
  after the current one"), 2 lines `keys.go` ('z'), 194 lines `model_test.go`
  (6-test family; deterministic seeds 11/3 pinned; multiset via order-insensitive
  `sortedSlugs` — the first RED roundtrip also proved the test suite rejects
  order-sensitive multiset comparison), 2 lines README. Architectural note worth
  pinning: shuffle does NOT cross albums — a track must stay in its own album
  for the stream-resolve context (advanceCatalogCmd receives the album); the
  visible order is album-order tail × within-album order, so two remaining
  tracks in DIFFERENT albums with single-item groups may render identical after
  a shuffle; the status still reports the honest count, identity permutation on
  tiny queues is expected, not a bug. Seed-related fix: Fisher-Yates seeds are
  pinned per test (11/3) — changing them re-verifies the order-change assertion
  (test even checks "seed must reorder").
- **L4** (CI evidence, e9e8e89): Verify Go code lint rejected the per-press seed
  with gosec **G404** (math/rand) — a false positive here: the rng orders the
  playlist, it guards no secret. Quieted with the repo idiom
  `//nolint:gosec // G404: shuffle order, not cryptographic` + the honest
  comment (precedent `musicdir.go:18`). Local it was silent: lint runs in CI
  only. Shuffle tests re-run green after the annotation.
