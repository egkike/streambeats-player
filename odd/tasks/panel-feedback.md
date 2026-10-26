# Panel Feedback Fix (live v1.5.0 defects)

Feedback visibility for transport actions (mouse + keys), visible theme palettes, status expiry.
Issue: egkike/streambeats-player#40 (pre-approved per standing grant, type:bug).

## Specs

- **S1** (owner verbatim): "Los botones del control no hacen nada cuando le doy click con el mouse." — every transport action (buttons AND the shared keybind paths) must leave a DISTINCT, honest status line: paused/resumed/stopped/previous/next/volume X%, not a silent state change.
- **S2** (owner verbatim): "cuando cambio a cualquiera de las 3 opciones no cambia nada del aspecto" — the three themes must be visibly different at a glance on the owner's dark terminal; System keeps the shipped dark look at boot (byte-parity), so Oscuro needs its own accent pair that differs from System-dark where the owner can SEE it.
- **S3** (owner verbatim): "sale un mensaje debajo del control de botones que dice !Theme saved pero no desaparece y se queda ahí" — feedback statuses must AUTO-EXPIRE after a short window (no insider requirement to wait for playback); statuses stop feeling sticky.
- **S4** (evidence): the progress poll tick currently `m.status = ""` at every real reading — it erases feedback instantly; feedback must survive a suppression window and clear itself after.
- **S5** (invariants): buttons keep routing through the SAME commands as keybinds (no second action path); no queue-advance trigger added; failure paths mutate nothing; absent-mpv friendly statuses unchanged in substance.

## Tasks

- **T1** (S1, S5): distinct feedback status for pause/resume (both directions), stop, and the volume receipt (currently silent); pause/stop paths wrapped through a shared setStatus helper. RED→GREEN.
- **T2** (S4): progress poll suppression window: a fresh feedback status survives the next real progress reading and clears when its window has passed (tick-count based, deterministic in tests). RED→GREEN.
- **T3** (S3): expiry — the same tick counter clears stale feedback after statusFeedbackTicks (~4s at 1s tick cadence when mpv is connected; no-player panels stay stable). RED→GREEN.
- **T4** (S2): palettes: System-dark keeps the shipped golden pair; Oscuro defines its own dark pair with different accents (selected/playing at minimum) so a switch is VISIBLE; Claro picks a light-pair family with distinct hues. RED→GREEN.
- **T5**: ODD log + docs (README TUI keys note needless? no visual copy change needed).

## Log

- **L1**: owner reported 3 live defects against the installed v1.5.0 (merged PR #38/#39, release v1.5.0 verified): verbatim in the Specs. Issue #40 opened pre-approved (status:approved + type:bug).
- **L2**: reproduction via pty harness + instrumented copied tree (not committed): span dispatch reached ALL buttons and mpv roundtrips confirmed; the visible failure is feedback-none-volume/status-inheritance + the tick eraser + non-expiry; probe data in mem obs-1556.
- **L3**: T1+T2+T3 RED→GREEN as one feedback unit: setStatus helper arms the window (statusFeedbackTicks=4, counted by the existing tick stream); togglePause says "paused — space to resume"/"resumed"; stop says "stopped" (and the honest no-op lines on the nil/idle paths, same as keys — S2 one-command-path parity); the volume receipt states "volume N%"; the progress poll clears only after the window (feedback survives the next reading); the tick counter clears stale feedback itself (T3 timer equivalent, no wall clock in the model). Limit note: expiry runs on the tick stream, so a mpv-absent panel keeps statuses stable — owner's panel has mpv connected (ticking).
- **T4** (S2) DONE: System-dark keeps the shipped golden pair (boot byte-parity pinned); Oscuro gets distinct accents (selected 212→99 violet, playing 42→46 bright green) so the switch is visible ON a dark terminal; Claro is its own hue family (teal title/bar, gray-green dim, magenta selected, green playing, coral status — all roles differ from Oscuro). Byte-parity consequence: on boot the panel is IDENTICAL to v1.4.x look; explicit Oscuro is now intentionally a visible variant.
- **T5** DONE: log L3–L4 notes + README/docs unchanged copy needs none (status texts are runtime feedback, help block untouched). CI relearn: golangci-lint staticcheck QF1001 rejects the De-Morgan-form negation — condition rewritten direct (third CI lint lesson: staticcheck semantics lint on top of errcheck).
