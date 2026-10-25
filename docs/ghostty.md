# Running streambeats in a Ghostty side panel

The player is meant to live in a Ghostty **quick terminal** (Quake-style panel)
that slides in from any screen edge, so you can control music while working in
another window. Spec: `odd/tasks/streambeats.md` — S3 (floating panel on any
side), S7 (Quick Terminal Ghostty), S8 (herdr), task T7.

Reference: <https://ghostty.org/docs/config/reference>
(`quick-terminal-position`, `quick-terminal-size`, `toggle_quick_terminal`).

**Status on this machine (Ghostty 1.3.1, KDE Plasma Wayland):** the launcher
opens the player in a **normal dedicated window** pinned by preset to a screen
side. A true edge-anchored quick terminal needs a newer Ghostty build. Both
paths are covered below — the presets are already configured for the upgrade.

## Requirements

- Ghostty **1.3.1 or newer** (verified against 1.3.1 on Ubuntu 26.04).
- Wayland with `wlr-layer-shell-v1`. Ghostty's quick terminal is **Wayland-only**
  on Linux; it is not available on X11, and GNOME cannot use it because GNOME
  does not implement `wlr-layer-shell-v1`. KDE Plasma only adds the slide-in
  animation when the "Sliding Popups" KWin plugin is enabled; on other
  compositors the panel appears without the animation.
- The `streambeats` binary (module `streambeats`, built from `cmd/streambeats`).

### Ghostty 1.3.1 limitations

Three 1.3.1 limitations shape this setup; all were verified at runtime on the
KDE Plasma Wayland desktop:

1. **`--config=<path>` is not a real option.** Ghostty has no `--config` flag
   for pointing at a custom config file (feature request: ghostty discussions
   #9434/#10058). Passing it works only through the *CLI pseudo-config*, and
   the GTK runtime then reports `config: unknown field` in a "Configuration
   Errors" dialog — even with an empty config file. The preset inside the
   file is NOT applied.
2. **No CLI action to spawn a quick terminal.** `+toggle-quick-terminal`
   exists only on Ghostty master (commit `4b7bf0b`, PR #12661), merged after
   the 1.3.1 release. On 1.3.1 a Ghostty launched from the CLI **cannot** be
   started in quick-terminal mode, and in this build `ctrl+alt+s` toggles a
   quick terminal inside the instance rather than raising the dedicated
   window.
3. **Runtime dialogs are silent to logging.** A config-error dialog prints
   nothing to stdout/stderr and does not change the exit code; the launcher's
   own stderr output stays clean. Validate with `ghostty
   +validate-config --config-file=<file>` before blaming your config.

Consequence: **today** the launcher gives you a dedicated normal window built
through Ghostty's canonical config path (see step 3). The
`quick-terminal-position` / `-autohide` values and the `toggle_quick_terminal`
keybind stay in every preset: 1.3.1 validates them, and the moment you run a
Ghostty build carrying `+toggle-quick-terminal` the same presets drive a real
edge-anchored quick terminal.

## 1. Build the player

```sh
go build -o bin/streambeats ./cmd/streambeats
```

`bin/` is git-ignored. `scripts/streambeats-quick` prefers this repo build and
falls back to a `streambeats` found on `PATH`.

## 2. Pick a side preset

Each preset sets the anchor, a sensible strip size, a toggle keybind, and the
dedicated-window hints used on 1.3.1:

| Preset | `quick-terminal-position` | `quick-terminal-size` | Dedicated window hint | Shape |
| --- | --- | --- | --- | --- |
| `assets/ghostty/streambeats-top.conf` | `top` | `30%` | `window-height = 40` | full-width strip, 30% tall |
| `assets/ghostty/streambeats-bottom.conf` | `bottom` | `30%` | `window-height = 40` | full-width strip, 30% tall |
| `assets/ghostty/streambeats-left.conf` | `left` | `22%` | `window-width = 52` | full-height strip, 22% wide |
| `assets/ghostty/streambeats-right.conf` | `right` | `22%` | `window-width = 52` | full-height strip, 22% wide |

Because `quick-terminal-size` takes a single value, it applies to the edge's
primary axis (height for top/bottom, width for left/right) and the other axis
stays maximized. Use a pair (`22%,600px`) if you also want to cap the
secondary axis.

Every preset also keeps the panel open when it loses focus
(`quick-terminal-autohide = false`, the Linux default) so playback controls do
not disappear the moment you click back into your editor.

The dedicated-window values (`window-width` / `window-height` in cells,
`title`, `window-decoration`) only affect the normal window used on 1.3.1; they
are ignored by a real quick terminal. `window-decoration = false` drops the
title bar so the window looks like a panel — the trade-off is documented in
step 4. All four fields are accepted by `ghostty +validate-config` on 1.3.1.

## 3. Launch it

```sh
scripts/streambeats-quick          # right edge (default)
scripts/streambeats-quick bottom   # bottom strip
scripts/streambeats-quick left     # left strip
scripts/streambeats-quick top      # top strip
```

The launcher resolves the preset relative to its own location, prints a
one-line summary plus a note about the 1.3.1 limitation, then runs the
player through Ghostty's **canonical config path**:

```sh
XDG_CONFIG_HOME=<temp dir> ghostty --gtk-single-instance=false -e <streambeats binary>
```

The temp dir contains `ghostty/config`, built at launch by concatenating your
personal Ghostty config (if present) with the side preset, so the preset keys
win and you keep your own theme/fonts. This avoids the invalid
`--config=<path>` approach entirely: presets apply for real, and the
"Configuration Errors" dialog goes away. The temp dir is left in
`${TMPDIR:-/tmp}` (`streambeats-ghostty.XXXXXX`); delete it at will.

## 4. Keep the player pinned above your work (KDE Plasma)

On 1.3.1 the player is an ordinary window with a normal title bar, so you can
drag it, minimize it, and close it like any app. To keep it always visible
above other windows:

- Right-click the title bar → **More Actions** → **Keep Above Others** (or
  **Alt+F3** → same menu item).
- To make it stick, use KWin's **Window Rules** (*System Settings → Window
  Management → Window Rules*) with a rule matching the title
  `streambeats player` and the action *Keep above other windows*. The same
  place lets you pin it to a specific virtual desktop or activity.

Resize and placement are normal: the compact size is a hint (`window-width =
52` / `window-height = 36` cells on left/right presets, `84`/`30` on
top/bottom); drag the window edges for something else, and KDE will remember
your geometry for next launches.

## 5. Show and hide the panel

Each preset binds:

```ini
keybind = ctrl+alt+s=toggle_quick_terminal
```

`ctrl+alt+s` is unused in Ghostty's default keybind set. On a Ghostty build with
`+toggle-quick-terminal`, that keybind slides a real quick terminal in and out
using the preset's position/size, and the terminal state is preserved between
appearances: hiding the panel does not stop playback, and showing it again
returns to the same TUI instance. On 1.3.1 the keybind cannot raise the
dedicated window; close and re-run the launcher instead.

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

## 6. herdr and gentle-shell

`herdr` is a terminal multiplexer with a server; it keeps living in its own,
adjacent Ghostty window, and your gentle-shell (Pi) session keeps running inside
it. The player window is an **independent Ghostty window** (S8), so:

- starting or closing the player never touches herdr sessions, panes, or agents;
- the panel sits beside or above the work window, so you can browse tracks
  without switching herdr windows or interrupting a running agent;
- keystrokes reach the panel only while it is focused, so editor and herdr
  shortcuts keep working the rest of the time;
- because the player is a separate Ghostty instance launched by the script, a
  running herdr server is neither required nor affected.

Once the quick terminal is available, only one can exist per Ghostty process, so
use one side per instance. If you want the panel on a different side, quit the
launcher and start it again with another side preset — this holds for the
dedicated window today too.

## 7. Upgrade path to a true quick terminal

Nothing to edit when you upgrade: install any Ghostty build that ships the
`+toggle-quick-terminal` action (anything after 1.3.1, commit `4b7bf0b`), then
either

- launch the player with a preset and press `ctrl+alt+s`, or
- let the launcher open the panel and toggle it from a focused Ghostty window
  using the same preset (position/size/autohide are already there).

The dedicated-window fields (`title`, `window-width`/`window-height`,
`window-decoration`) are simply ignored in quick-terminal mode, so the same
presets serve both modes. Confirm what your binary supports with
`ghostty +help | grep toggle-quick-terminal`: on 1.3.1 the action is absent and
`ghostty +toggle-quick-terminal` fails with *unknown CLI action*.

## macOS (darwin): the floating path without systemd

macOS is a **declared install target** (see docs/install.md, macOS notes).
The whole Linux desktop story (.desktop, KWin rules, keybind) is skipped by
the installer there; what remains is exactly what makes the per-side
presets useful:

1. Install Ghostty for macOS (<https://ghostty.org/docs/install>), then run
   `streambeats` for the TUI and `streambeats-quick <side>` for a panel.
2. Floating panel: the native **Ghostty global hotkey** is the official
   macOS path — one hotkey, no window rules or kdotool needed. Configure it
   in Ghostty settings; the panel then feels like the KDE keybind on Linux
   but lives entirely inside the app.
3. `streambeats-panel` on macOS runs without systemd — the wrapper falls
   back to a `setsid`-style detached background spawn (no journalctl, no
   per-side unit lock; the single-instance guard then relies on Ghostty
   itself).

Caveat: the install/uninstall cycle is CI-validated on a real darwin
binary; the floating-panel experience itself still awaits live validation
on a physical Mac.

## Troubleshooting

- **`ghostty not found on PATH`** — install Ghostty
  (<https://ghostty.org/docs/install>) or add its binary to `PATH`.
- **`streambeats binary not found`** — run the build command from step 1.
- **`missing preset file`** — the script expects to live in `<repo>/scripts/`;
  run it from the repository checkout instead of a copy.
- **Panel opens but shows a plain shell** — run `streambeats` from that shell,
  or quit the launcher and re-run `scripts/streambeats-quick <side>`; the
  `-e` command is the entry the launcher passes to Ghostty.
- **GTK dialog about an unknown/invalid config field** — it means a
  `--config=<path>` style invocation reached Ghostty. That flag does not
  exist (ghostty discussions #9434/#10058); the runtime reports a
  pseudo-config `cli:` entry as `config: unknown field` and applies none of
  the file. The launcher builds a temp `XDG_CONFIG_HOME` (step 3) and uses
  the canonical config path instead. Click `Ignorar`, re-run the launcher,
  and the dialog should not return (verified against the full dialogue-
  window capture pipeline).
- **`ctrl+alt+s` does nothing** — expected on 1.3.1, which has no
  `+toggle-quick-terminal` action; the player is a normal window until you
  upgrade (step 7).
- **`unknown side`** — valid sides are `top`, `bottom`, `left`, `right`.
- **No slide-in animation** — Linux animations require KDE with the "Sliding
  Popups" KWin plugin; other compositors show the panel without animating.
