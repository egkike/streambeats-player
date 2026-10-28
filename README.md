# streambeats

<div align="center">

<a href="https://github.com/Gentleman-Programming/gentle-ai">
  <img width="220" src="https://raw.githubusercontent.com/Gentleman-Programming/gentle-ai/main/docs/assets/brand/built-with-gentle-ai.png" alt="Built with Gentle-AI" />
</a>

</div>

A floating, quick-terminal music player for the free [StreamBeats](https://streambeats.bandcamp.com/music)
catalog — built for a live coding workflow (Ghostty + herdr + gentle-shell).

The catalog streams directly from Bandcamp's mp3-128 endpoints (StreamBeats
releases are free), with an optional local library built from the same
streams. No third-party unlockers, accounts or subscriptions involved.

## Install (one line)

```sh
curl -fsSL https://raw.githubusercontent.com/egkike/streambeats-player/main/scripts/install.sh | bash
```

User-level install, no sudo: the full floating-panel experience lands in
`~/.local/bin` + `~/.local/share/streambeats`. The archive is downloaded and
SHA-256 verified against the release's `checksums.txt` before anything is
written. Details, pinned versions and the installed layout: docs/install.md.

```sh
streambeats               # TUI player, runs in any terminal
streambeats-quick right   # floating quick-terminal panel (Ghostty)
```

Uninstall (the bundled CLI removes exactly what the installer landed; the
panel is stopped first and your local library — the `StreamBeats` folder
inside your XDG music dir, e.g. `~/Música` on es systems or `~/Music` on
en — is never touched):

```sh
streambeats uninstall          # add --cache to also clear the catalog cache
```

Manual inventory for installs from a pre-CLI version (v1.2.x and older):

```sh
# stop any live panel first
systemctl --user stop streambeats-panel-{top,bottom,left,right} 2>/dev/null
systemctl --user reset-failed streambeats-panel-{top,bottom,left,right} 2>/dev/null

# drop the app (all paths user-level, no sudo)
rm -rf ~/.local/share/streambeats \
       ~/.local/bin/streambeats ~/.local/bin/streambeats-quick ~/.local/bin/streambeats-panel \
       ~/.local/share/applications/streambeats-panel.desktop \
       ~/.local/share/icons/hicolor/scalable/apps/streambeats-panel.svg

# optional: the catalog cache is redownloadable — drop it too
rm -rf ~/.cache/streambeats
```

Your local library (the `StreamBeats` folder inside your XDG music dir)
and your `~/.config/streambeats/settings.json` (theme, `libraryRoot`) are
user data: nothing above ever touches them.

## Requirements

- **Linux (amd64/arm64)** — the runtime contract builds on Unix domain
  sockets (mpv IPC) and POSIX tooling; a native Windows install is refused.
- **Windows via WSL2 (Ubuntu)** — the TUI player works in WSL2 (same
  linux_amd64 runtime and sockets): install inside WSL2 with the same
  one-liner and run `streambeats` in any terminal. The floating panel is
  NOT available there — it needs Ghostty, which does not support Windows;
  the panel/flotation story stays Linux + macOS.
- **macOS (amd64/arm64, darwin)** — declared as of this release: the
  installer detects darwin, skips the Linux freedesktop integration
  (`.desktop`, hicolor icon, KWin rules are Linux-only) and points the PATH
  hint at `~/.zprofile` (zsh). The floating panel on macOS is the NATIVE
  Ghostty experience (global hotkey — better than the KDE keybind story, see
  `docs/ghostty.md`); the manual per-side presets under `assets/ghostty/`
  keep working. The installer and CLI are validated by the CI macOS job
  (real-binary install + uninstall end-to-end, bash 3.2 gate); the floating
  panel itself is CI-validated (real-binary install/uninstall); physical
  Mac live validation is accepted as out of scope — the owner has no
  macOS hardware, so CI coverage is the support contract.
- **mpv ≥ 0.41** — playback backend over JSON IPC. Without mpv the TUI still
  browses the catalog and says so in a friendly status line; it never crashes.
- **Network access to bandcamp.com** — stream URLs are tokenized and expire;
  a fresh one is resolved right before every play, never cached.
- **Ghostty 1.2+** — required only for the floating quick-terminal panel
  (`streambeats-quick` + presets under `assets/ghostty/`); the `streambeats`
  TUI itself runs in any terminal. See docs/ghostty.md.
- **kdotool 0.3+** (Linux, optional) — enables the panel show/hide toggle:
  `streambeats-panel toggle <side>` minimizes the focused panel (audio keeps
  playing) and restores/raises it on the next press. Without it the panel
  still opens — the second press just cannot hide it. The installer warns
  with the exact install paths when it is missing.
- **Go 1.27+** — only for building from a source checkout (below); the user
  install never runs the compiler.

## Build

```sh
go build -o bin/streambeats ./cmd/streambeats
```

## Use

```sh
./bin/streambeats          # launch the TUI
./bin/streambeats refresh  # scrape the StreamBeats catalog into cache
./bin/streambeats sync     # download playable albums to <music dir>/StreamBeats (XDG)
./bin/streambeats help     # key bindings and paths
```

TUI keys: `j/k` move, `enter` play/open, `l` library browser (navigate
folders; `u` goes up; the root defaults to your XDG music dir, editable
via `libraryRoot` in settings.json), `space` pause,
`s` stop, `+/-` volume, `>` next song, `<` previous song, `esc` back,
`q` quit. The transport row at the bottom is visual only — the keyboard
does the work — and is terminal-adaptive (detected once at launch):
ghostty shows the emoji keycaps (⏮ ⏯ ⏹ ⏭ 🔊), any other terminal shows
width-stable ASCII tokens (`|<` `||` `>|` `[]` `vol`) so the frames stay
aligned. The player auto-advances to the next item of the list when a
track ends; the end of the queue shows `playlist ended` in the status
line. Tracks always resolve a fresh stream URL right before playing
(Bandcamp tokens expire).

## Floating quick terminal

```sh
scripts/streambeats-quick [top|bottom|left|right]   # from a source checkout
streambeats-panel  [top|bottom|left|right]          # installed (detached)
```

Presets live in `assets/ghostty/streambeats-<side>.conf` (default side:
`right`, narrow 22% strip). Toggle with `ctrl+alt+s`.

The installed companion `streambeats-panel` spawns the same panel detached
from any terminal (systemd user unit per side, single-instance guard) so a
`.desktop` entry or a KDE global keybind runs it with one action. Since
v1.7.2 the desktop entry and the recommended keybind launch the **toggle**:
first press spawns, later presses show/hide (audio keeps playing when
hidden; needs kdotool — the installer advisory names the install paths).
KRunner ships side actions via the generated desktop entry. Keybind + KWin
floating window-rules steps: docs/kde-panel-integration.md
(plain `scripts/streambeats-quick` stays tied to the calling shell; use the
README "Run detached" recipe for that path).

## Architecture

- `internal/catalog` — Bandcamp scraping (/music + data-tralbum), JSON
  cache, fresh stream resolution, incremental local sync.
- `internal/player` — mpv JSON IPC client (typed set_property per mpv 0.41,
  loadfile replace, event-aware).
- `internal/tui` — Bubbletea UI, pure-update model, friendly degraded mode
  without mpv.
- `scripts/install.sh` — curl-based user installer, SHA-256 verified
  (tests in `scripts/tests/install_test.sh`; detail in docs/install.md).
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
