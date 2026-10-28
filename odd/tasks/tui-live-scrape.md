# TUI live-scrape startup (issue #75, follow-up of v1.7.2)

On a fresh cache the TUI prints "no cached catalog yet; scraping it
now..." and blocks for the whole live scrape with no feedback; the TUI
is not up, so even `q` is dead. Owner hit it on the v1.7.2 first run
(2026-10-28). This cycle boots the TUI immediately with a live-scrape
status and populates the list in place. Branch: `feat/tui-live-scrape`.

## Specs

- S1: "arrancar la TUI de inmediato con un estado 'scraping catalog…'
  en la barra de status, y poblar la lista cuando el scrape termina" —
  no pre-TUI terminal message, no blocking startup.
- S2: cached path unchanged: with a cache the TUI boots populated
  exactly as today (no scrape, no status flash).
- S3: scrape failure → friendly error status, app stays alive, `r`
  retries the scrape; cache written only after a successful scrape.
- S4: untouched invariants: idle-active queue-advance guard, mpv IPC
  typing, stream tokens never in the cache.

## Tasks

- T1: branch + this doc + Engram mirror.
- T2 (S1,S2,S3): RED — teatest scenarios with a fake store/refresh.
- T3 (S1,S3): GREEN — model boots with empty catalog + async scrape cmd.
- T4: full pipeline + native review + PR Closes #75.

## Log

- L1 (owner, 2026-10-28): after installing v1.7.2 the first run showed
  the scraping message and appeared frozen until quit+re-enter; root
  cause: blocking first-run scrape with zero feedback (TUI not up yet).
  Owner: "Dale a los 2 ciclos" (this one + library navigator #76).
