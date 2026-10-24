# streambeats

A floating, quick-terminal music player for the free [StreamBeats](https://streambeats.bandcamp.com/music)
catalog — built for a live coding workflow (Ghostty + herdr + gentle-shell).

The catalog streams directly from Bandcamp's mp3-128 endpoints (StreamBeats
releases are free), with an optional local library built from the same
streams. No third-party unlockers, accounts or subscriptions involved.

## Requirements

- Go 1.27+
- mpv (playback backend, JSON IPC)
- Ghostty 1.2+ (quick terminal; on Linux the quick terminal surfaces on
  Wayland with wlr-layer-shell — see docs/ghostty.md)

## Build

```sh
go build -o bin/streambeats ./cmd/streambeats
```

## Use

```sh
./bin/streambeats          # launch the TUI
./bin/streambeats refresh  # scrape the StreamBeats catalog into cache
./bin/streambeats sync     # download playable albums to ~/Music/StreamBeats
./bin/streambeats help     # key bindings and paths
```

TUI keys: `j/k` move, `enter` play, `l` local-library mode, `space` pause,
`s` stop, `+/-` volume, `>` next song, `<` previous song, `esc` back,
`q` quit. The player auto-advances to the
next item of the list when a track ends; the end of the queue shows
`playlist ended` in the status line. Tracks always resolve a fresh
stream URL right before playing (Bandcamp tokens expire).

## Floating quick terminal

```sh
scripts/streambeats-quick [top|bottom|left|right]
```

Presets live in `assets/ghostty/streambeats-<side>.conf` (default side:
`right`, narrow 22% strip). Toggle with `ctrl+alt+s`.

## Architecture

- `internal/catalog` — Bandcamp scraping (/music + data-tralbum), JSON
  cache, fresh stream resolution, incremental local sync.
- `internal/player` — mpv JSON IPC client (typed set_property per mpv 0.41,
  loadfile replace, event-aware).
- `internal/tui` — Bubbletea UI, pure-update model, friendly degraded mode
  without mpv.
- `docs/ghostty.md` — quick terminal integration details.

## Run detached (survives the invoking terminal)

The commands above tie the player to your shell; closing the terminal window
would kill it. Launch it under your user manager instead:

```sh
systemd-run --user --unit=streambeats-player \
  "${PWD}/scripts/streambeats-quick" right
```

Then:

```sh
systemctl --user status streambeats-player   # check
systemctl --user stop streambeats-player     # stop player
journalctl --user -u streambeats-player      # launcher/ghostty logs
```

(Verified on this machine: the unit goes active, `WAYLAND_DISPLAY` is
inherited by the user manager under KDE Plasma, and the floating window
opens with the catalog.)

## Notes

- Runs beside herdr as an independent Ghostty window; nothing needs to be
  embedded into the agent runtime.
- MP3-128 is Bandcamp's streaming tier; for archival quality download the
  FLAC from Bandcamp manually per release.
