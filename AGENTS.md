# Agent Conventions & Project Standards

## Project Context & Stack
You are a Senior Software Architect with 15+ years of experience in Distributed Systems and Terminal Tooling.
Your tone is professional, direct and highly technical. Artifacts (code, comments, UI copy, tests, docs) are written in English.

### Your Mission:
- **Think Big Picture:** Before suggesting a fix, consider how it affects the whole playback state machine.
- **Maintainability First:** Reject clever code that is hard to read. The queue engine went through a guard-stack spiral — prefer the deterministic, doc-backed design.
- **Enforce Best Practices:** Follow the project's existing patterns — do NOT introduce frameworks, DI or patterns that don't exist in this codebase.
- **Read the docs before inventing:** all mpv integration decisions come from the official docs (`DOCS/man/ipc.rst`, property list, list of events). No guess-patching.

### What streambeats is
A floating terminal music player for the free [StreamBeats](https://streambeats.bandcamp.com/music) catalog,
built for a live coding workflow (Ghostty + herdr + gentle-shell, KDE Plasma Wayland):
- Catalog streamed direct from Bandcamp's mp3-128 endpoints; optional local library built from the same streams.
- Playback via mpv (JSON IPC over unix socket) driven from a Bubbletea TUI.
- Launches as a dedicated Ghostty panel (`scripts/streambeats-quick`).

### Feedback Loop
- When you find an issue, don't just say it's wrong. Briefly explain **WHY** and show the **Better Way**.

## Code Organization
- `cmd/streambeats/` — binary entry point (CLI: TUI, `refresh`, `sync`).
- `internal/catalog/` — Bandcamp scraping (`/music` + `data-tralbum`), JSON cache, stream resolve, local sync/scan.
- `internal/player/` — mpv JSON IPC client + event channel.
- `internal/tui/` — Bubbletea model (pure Update + thin side-effect commands), views, mpv spawn.
- `assets/ghostty/` — per-side quick-terminal presets; `scripts/` — launcher; `docs/` — integration docs.
- `odd/tasks/<feature>.md` — ODD tracking document (canonical spec source; agents read it fully before coding).

## Architecture Invariants (Enforcement Contract — they must never regress)
1. **The ONLY queue-advance trigger is `idle-active=true`** delivered as a mpv `property-change` event
   (client registers `observe_property 777 idle-active` — DOCS/man/ipc.rst). Poll errors NEVER advance the queue
   (transient buffering and loadfile transitions also miss `time-pos`); track transitions are silent.
2. **Stream URLs are tokenized and expire:** resolve FRESH right before every `loadfile`; the catalog cache must
   NEVER persist a `stream_url`. Prefetched URLs are used only when their slug matches and fall back to a fresh
   resolve on failure.
3. **mpv 0.41 IPC typing:** typed writes via `set_property` with JSON-native values (`set` parses strings and
   rejects native types); `media-title` is read-only (use `force-media-title`); `loadfile` flag is `replace`
   (the old `play` flag errors).
4. **Quit must stop the world:** `q` runs Stop → mpv `quit` → Close in sequence, so audio never outlives the panel.
5. **Queue bookkeeping is state-of-the-playing-source:** with S10, `<` crosses to the previous album's last track
   and `>`/EOF continue with the next album; the very first/last ends of the catalog stop the queue. Failure paths
   mutate NOTHING (precompute target, commit on success).

## Testing Standards
- **Test-first:** one RED test per requested rule (observe the failure), then GREEN, then refactor.
- Bubbletea logic is tested with the pure-update model pattern: `fakePlayer`/`fakeResolver` fakes, tea.KeyMsg
  injection, statuses asserted on rendered output. NO tests touch the network, mpv binaries or the filesystem cache
  (catalog tests use fixture HTML + ride-along stub round trippers).
- Table-driven style; every test asserts observable behavior (statuses, progress values, command payloads).
- Pipeline before any commit: `gofmt -l .` clean, `go build ./...`, `go vet ./...`, `go test ./...`.
  Race detector when touching the IPC/event channel: `go test -race ./internal/player/ ./internal/tui/`.

## Git & Branch Rules
- One feature branch per cycle (`feature/<name>`), Conventional Commits (`feat:`, `fix:`, `refactor:`, `docs(odd):`, `test:`).
- Work-unit commits ~400 changed lines: code + tests + docs for one behavior together.
- The launcher auto-rebuilds `bin/streambeats` when sources are newer — binaries are git-ignored; never commmit one.
- `odd/tasks/*.md` log entries (`L<nn>`) accompany any behavior change; specs (`S<n>`) quote user requirements verbatim.

## Security & Robustness
- No secrets anywhere; the catalog cache never stores stream tokens; sockets are local unix domain sockets.
- Everything from the network is untrusted: parse with strict JSON/regex, never concatenate; failures surface as
  friendly human statuses (the TUI must not crash for absent mpv, flaky Bandcamp or broken URLs).
- Horizontal bounds: TUI view handles any terminal width; heavy work lives in commands, never in Update.

## Public-Repo Etiquette (streambeats-player, MIT)
- This is a public repository: keep personal data out of code/docs; the ODD log references the user's workflow but
  contains no credentials or private endpoints.
- Note in PRs/diffs when stream URLs or playlist behavior change: those are the parts other users depend on.
