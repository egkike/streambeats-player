# first-play-prefetch — first playback wait + prefetch on album open (issue #62)

## Specs

- S1: Owner report (verbatim, 2026-10-27): "Al entrar por primera vez a la app y darle barra espacio sobre la primer musica es como que tarda un poco en arrancar... revisa si eso es normal o si tenemos algun problema por ahí" — corrected same session: "Verifica bien porque la barra de espacio no activa la primer musica, solo el Enter... luego si ya funciona".
- S2: Owner approved both improvements: "Sí, ambas mejoras" — prefetch del primer track al abrir el álbum + status "resolving stream…" durante la espera.
- S3: The Architecture Invariant 2 (AGENTS.md) must keep holding: stream URLs are tokenized and expire; resolve FRESH right before every `loadfile`; the catalog cache must NEVER persist a `stream_url`. The seed prefetch keeps the existing semantics: slug-matched use only, fresh-resolve fallback on mismatch/failure.
- S4: Bandcamp courtesy: one fetch per album open, never per cursor movement (each fetch downloads the full ~311 KB album page).
- S5: Diagnosis finding (included in this cycle, disclosed to owner in the PR): the catalog HTTP client bounds each TCP dial at 5s — a dropped SYN to one Fastly edge IP stalled on kernel retransmissions ~7.5s (strace evidence, 2026-10-27) before the next address was tried.

## Tasks

- T1: RED tests for album-open seed, seed guard, no cursor-move fetch (S2, S4) — committed in feat cycle.
- T2: RED tests for direct-play prefetch use + mismatch fallback (S3) — committed in feat cycle.
- T3: RED test for "resolving stream…" status cleared by loadedMsg (S2).
- T4: Implement seed + direct-play prefetch consult + status; delete unused resolveAndPlayCmd.
- T5: Dial timeout bound in catalog.NewClient (S5).
- T6: Pipeline green (gofmt/build/vet/test + race) and live tmux verification of both flows (with and without prefetch).

## Log

- L1: Owner reported a perceived delay starting the first track in a fresh session (see S1, S2).
- L2: Investigation (2026-10-27): catalog loads from cache (no network at launch); mpv spawns at Launch; the wait is the mandatory fresh resolve (~1s measured in-app) + mpv stream open (~1.5s bare) + random TCP stalls to Fastly edge IPs on this network (~7.5s when SYNs drop, strace) — all invisible to the user because the play path set no status. Space DOES start the first track (round-3 replay, issue #42): reproduced end-to-end in tmux with the real binary; pinned as a regression test.
- L3: Owner authorized the cycle in conversation (standing grant, working model: issue #25) and selected "Sí, ambas mejoras" (both improvements).
- L4: During diagnosis, temporary probe tests were created and removed; the space-replay probe became the permanent regression pin (TestSpaceStartsFirstTrackFromAlbumList).
