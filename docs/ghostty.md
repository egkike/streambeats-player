# Running streambeats in a Ghostty quick terminal

The player is meant to live in a Ghostty **quick terminal** (Quake-style panel)
that slides in from any screen edge, so you can control music while working in
another window. Spec: `odd/tasks/streambeats.md` — S3 (floating panel on any
side), S7 (Quick Terminal Ghostty), S8 (herdr), task T7.

Reference: <https://ghostty.org/docs/config/reference>
(`quick-terminal-position`, `quick-terminal-size`, `toggle_quick_terminal`).

## Requirements

- Ghostty 1.3.1 or newer (verified against 1.3.1 on Ubuntu 26.04).
- Wayland with `wlr-layer-shell-v1`. Ghostty's quick terminal is **Wayland-only**
  on Linux; it is not available on X11, and GNOME cannot use it because GNOME
  does not implement `wlr-layer-shell-v1`. KDE Plasma only adds the slide-in
  animation when the "Sliding Popups" KWin plugin is enabled; on other
  compositors the panel appears without the animation.
- The `streambeats` binary (module `streambeats`, built from `cmd/streambeats`).

## 1. Build the player

```sh
go build -o bin/streambeats ./cmd/streambeats
```

`bin/` is git-ignored. `scripts/streambeats-quick` prefers this repo build and
falls back to a `streambeats` found on `PATH`.

## 2. Pick a side preset

Each preset sets the anchor, a sensible strip size, and a toggle keybind:

| Preset | `quick-terminal-position` | `quick-terminal-size` | Shape |
| --- | --- | --- | --- |
| `assets/ghostty/streambeats-top.conf` | `top` | `30%` | full-width strip, 30% tall |
| `assets/ghostty/streambeats-bottom.conf` | `bottom` | `30%` | full-width strip, 30% tall |
| `assets/ghostty/streambeats-left.conf` | `left` | `22%` | full-height strip, 22% wide |
| `assets/ghostty/streambeats-right.conf` | `right` | `22%` | full-height strip, 22% wide |

Because `quick-terminal-size` takes a single value, it applies to the edge's
primary axis (height for top/bottom, width for left/right) and the other axis
stays maximized. Use a pair (`22%,600px`) if you also want to cap the
secondary axis.

Every preset also keeps the panel open when it loses focus
(`quick-terminal-autohide = false`, the Linux default) so playback controls do
not disappear the moment you click back into your editor.

## 3. Launch it

```sh
scripts/streambeats-quick          # right edge (default)
scripts/streambeats-quick bottom   # bottom strip
scripts/streambeats-quick left     # left strip
scripts/streambeats-quick top      # top strip
```

The launcher resolves the preset relative to its own location, then runs:

```sh
ghostty --config <preset> --config-default-files=false -e <streambeats binary>
```

`--config-default-files=false` keeps your personal Ghostty config out of the
way, so the preset fully owns the panel geometry.

## 4. Show and hide the panel

Each preset binds:

```ini
keybind = ctrl+alt+s=toggle_quick_terminal
```

`ctrl+alt+s` is unused in Ghostty's default keybind set. The terminal state is
preserved between appearances: hiding the panel does not stop playback, and
showing it again returns to the same TUI instance.

A system-wide shortcut is available as a commented line in each preset:

```ini
# keybind = global:ctrl+alt+s=toggle_quick_terminal
```

`global:` keybinds depend on the Linux build and compositor (they usually need
extra system configuration). Uncomment only if your Ghostty accepts the config;
otherwise leave it out and toggle the panel from a focused Ghostty window.

You can also drop the `quick-terminal-*` lines and the keybind into your main
Ghostty config (`~/.config/ghostty/config`) if you always want the same side.
The presets exist so you can switch sides per launch without editing your
config.

## 5. herdr and gentle-shell

`herdr` is a terminal multiplexer with a server; it keeps living in its own,
adjacent Ghostty window, and your gentle-shell (Pi) session keeps running inside
it. The quick terminal is an **independent Ghostty window** (S8), so:

- starting or closing the player never touches herdr sessions, panes, or agents;
- the panel floats above the work window, so you can browse tracks without
  switching herdr windows or interrupting a running agent;
- keystrokes reach the panel only while it is focused, so editor and herdr
  shortcuts keep working the rest of the time;
- because the quick terminal is a separate Ghostty instance launched by the
  script, a running herdr server is neither required nor affected.

Only one quick terminal can exist per Ghostty process, so use one side per
instance. If you want the panel on a different side, quit the launcher and start
it again with another side preset.

## Troubleshooting

- **`ghostty not found on PATH`** — install Ghostty
  (<https://ghostty.org/docs/install>) or add its binary to `PATH`.
- **`streambeats binary not found`** — run the build command from step 1.
- **`missing preset file`** — the script expects to live in `<repo>/scripts/`;
  run it from the repository checkout instead of a copy.
- **Panel opens but shows a plain shell** — run `streambeats` from that shell,
  or quit the launcher and re-run `scripts/streambeats-quick <side>`; the
  `-e` command is the entry the launcher passes to Ghostty.
- **`unknown side`** — valid sides are `top`, `bottom`, `left`, `right`.
- **No slide-in animation** — Linux animations require KDE with the "Sliding
  Popups" KWin plugin; other compositors show the panel without animating.
