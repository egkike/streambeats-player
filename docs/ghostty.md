# Running streambeats in a Ghostty side panel

The player lives in a Ghostty **quick terminal** (Quake-style panel) that
slides in from any screen edge, so you can control music while working in
another window. The launcher also supports **kitty** (0.42+ native quick
access terminal) as an alternative panel host — see `docs/kitty.md` and the
settings-menu choice (issue #90). Spec: `odd/tasks/streambeats.md` — S3
(floating panel on any side), S7 (Quick Terminal Ghostty), S8 (herdr), task
T7; quick-terminal parity: `odd/tasks/ghostty-quick-terminal.md` (issue #134).

Reference: <https://ghostty.org/docs/config/reference>
(`quick-terminal-position`, `quick-terminal-size`, `toggle_quick_terminal`).

**Status on this machine (Ghostty 1.4.0, KDE Plasma Wayland):** the launcher
rides the terminal's **native quick terminal** with kitty's launch model: the
same command opens the panel on the first press and toggles show/hide on every
later press. On a Ghostty build without the mechanism (pre-1.4) the launcher
falls back to a **normal dedicated window** pinned by preset to a screen side.

## Requirements

- Ghostty **1.4.0 or newer** for the quick-terminal mode (verified against
  1.4.0 on Ubuntu 26.04). The dedicated-window fallback works since 1.3.1.
- `gdbus` (ships with the GLib that Ghostty's GTK build already requires) —
  the launcher drives the panel toggle through it (upstream defect below).
- Wayland with `wlr-layer-shell-v1`. Ghostty's quick terminal is
  **Wayland-only** on Linux; it is not available on X11, and GNOME cannot use
  it because GNOME does not implement `wlr-layer-shell-v1`. KDE Plasma only
  adds the slide-in animation when the "Sliding Popups" KWin plugin is
  enabled; on other compositors the panel appears without the animation.
- The `streambeats` binary (module `streambeats`, built from
  `cmd/streambeats`).

## How the quick-terminal mode works (issue #134)

Ghostty 1.4.0 exposed quick-terminal toggling to the outside world, with two
facts verified at runtime on the desktop:

1. **A dedicated instance can claim its own identity.** A ghostty app runs
   with `--gtk-single-instance=true` (the `detect` default never claims a
   bus name) and a **dotted** GTK application id — the launcher uses
   `org.streambeats.panel.<side>` (top/bottom/left/right) so two panels can
   never claim each other's name and the user's own default ghostty singleton
   (`com.mitchellh.ghostty`) is never touched.
2. **The panel toggles through `org.gtk.Actions.Activate`.** The upstream
   `ghostty +toggle-quick-terminal` CLI action exists, but its documented
   `--class=<appId>` flag is parsed and then **ignored** (`run()` hardcodes
   the `.detect` IPC target): every invocation reaches the default bus
   owner instead of the addressed instance. Until upstream wires the flag,
   the launcher calls the same GAction directly:

   ```sh
   gdbus call --session --dest org.streambeats.panel.right \
     --object-path /org/streambeats/panel/right \
     --method org.gtk.Actions.Activate toggle-quick-terminal '[]' '[]'
   ```

The host spawn is headless (`initial-window = false`); the only surface is
the quick terminal the toggle draws, so `quit-after-last-window-closed` (the
1.4.0 default) exits the app when the player quits — the next press respawns
it fresh, exactly like the kitty host.

The host runs the panel preset **preset-only** through a per-run virtual
`XDG_CONFIG_HOME`; the player child reads the user's real config home via a
generated wrapper (issue #80), so settings.json and user-dirs resolution
keep working inside the panel. The wrapper's command is configured per run
(`command = <confdir>/run-player.sh`) because the quick-terminal surface
cannot take a `-e` override through the toggle path.

## 1. Build the player

```sh
go build -o bin/streambeats ./cmd/streambeats
```

`bin/` is git-ignored. `scripts/streambeats-quick` prefers this repo build
and falls back to a `streambeats` found on `PATH`.

## 2. Pick a side preset

Each preset sets the anchor, a sensible strip size, a toggle keybind, and
the dedicated-window hints used by the fallback:

| Preset | `quick-terminal-position` | `quick-terminal-size` | Fallback window hint | Shape |
| --- | --- | --- | --- | --- |
| `assets/ghostty/streambeats-top.conf` | `top` | `30%` | `84 × 30` cells | full-width strip, 30% tall |
| `assets/ghostty/streambeats-bottom.conf` | `bottom` | `30%` | `84 × 30` cells | full-width strip, 30% tall |
| `assets/ghostty/streambeats-left.conf` | `left` | `22%` | `52 × 24` cells | full-height strip, 22% wide |
| `assets/ghostty/streambeats-right.conf` | `right` | `22%` | `54 × 24` cells | full-height strip, 22% wide |

Because `quick-terminal-size` takes a single value, it applies to the edge's
primary axis (height for top/bottom, width for left/right) and the other
axis stays maximized. Use a pair (`22%,600px`) if you also want to cap the
secondary axis.

Every preset also keeps the panel open when it loses focus
(`quick-terminal-autohide = false`, the Linux default) so playback controls
do not disappear the moment you click back into your editor.

The fallback values (`window-width` / `window-height` in cells, `title`,
`window-decoration`, `class = streambeats-panel`) only shape the dedicated
window; a real quick terminal ignores them. `class = streambeats-panel` is
the FALLBACK window identity — a quick-terminal panel is a layer-shell
surface, not a KWin window, and never matches window rules.

## 3. Launch it

```sh
scripts/streambeats-quick          # right edge (default)
scripts/streambeats-quick bottom   # bottom strip
scripts/streambeats-quick left     # left strip
scripts/streambeats-quick top      # top strip
```

The launcher resolves the preset relative to its own location, prints a
one-line summary plus the toggle note, then:

- **quick mode (1.4.0+ default):** toggles the claimed panel, or spawns the
  headless per-side host first (`XDG_CONFIG_HOME=<temp dir> ghostty
  --gtk-single-instance=true --initial-window=false --class=...`) and toggles
  once it claims its bus name. Running the command again toggles show/hide.
- **fallback (pre-1.4 }):** runs the player through Ghostty's canonical
  config path (`XDG_CONFIG_HOME=<temp dir> ghostty --gtk-single-instance=false
  -e <streambeats>`), one dedicated window per launch.

The temp dir contains `ghostty/config` plus the generated host keys and the
player wrapper; it is left in `${TMPDIR:-/tmp}`
(`streambeats-ghostty.XXXXXX`) for inspection, delete at will.

## 4. Keep the player pinned above your work (KDE Plasma)

In quick-terminal mode nothing is needed: the panel is a layer-shell surface
anchored to the screen edge it was launched for (a slide-in like a dropdown;
it is not a regular KWin window). Window rules only apply to the fallback
dedicated window.

For the fallback window (pre-1.4 hosts):

- Right-click the title bar → **More Actions** → **Keep Above Others** (or
  **Alt+F3** → same menu item), or KWin's **Window Rules** with a rule
  matching window class/title `streambeats panel` (see
  `assets/kwin/window-rules.md`).

## 5. Show and hide the panel

The panel toggles by re-running the launch command — the KDE global shortcut
(documented in `assets/kwin/window-rules.md` and wired through
`scripts/streambeats-panel`) doubles as the show/hide key on both hosts. The
first press opens; every later press toggles.

Each preset also binds, inside the panel:

```ini
keybind = ctrl+alt+s=toggle_quick_terminal
```

`ctrl+alt+s` toggles this instance's quick terminal — in quick mode it
hides/shows the panel itself; inside the fallback window it is harmless.

A system-wide shortcut line (Ghostty's own `global:` keybind) is available as
a commented line in each preset, but on this setup the KDE shortcut plus the
relay covers it.

## 6. herdr and gentle-shell

`herdr` keeps living in its own, adjacent Ghostty window, and your
gentle-shell (Pi) session keeps running inside it. The player panel is an
independent ghostty instance (S8):

- starting or closing the player never touches herdr sessions, panes, or
  agents;
- the panel sits beside or above the work window, so you can browse tracks
  without switching herdr windows or interrupting a running agent;
- keystrokes reach the panel only while it is focused, so editor and herdr
  shortcuts keep working the rest of the time;
- only one quick terminal exists per ghostty instance — and each panel side
  has its own per-side instance anyway, so switching sides never needs to
  quit another panel.

## macOS (darwin): the floating path without systemd

macOS is a **declared install target** (see docs/install.md, macOS notes).
The whole Linux desktop story (.desktop, KWin rules, keybind) is skipped by
the installer there; what remains is exactly what makes the per-side presets
useful:

1. Install Ghostty for macOS (<https://ghostty.org/docs/install>), then run
   `streambeats` for the TUI and `streambeats-quick <side>` for a panel.
2. Floating panel: the native **Ghostty global hotkey** is the official
   macOS path — one hotkey, no window rules or kdotool needed. Configure it
   in Ghostty settings; the panel then feels like the KDE keybind on Linux
   but lives entirely inside the app. (The D-Bus quick-toggle mechanism
   documented above is GTK-only; macOS uses Ghostty's own hotkey.)
3. `streambeats-panel` on macOS runs without systemd — the wrapper falls
   back to a detached background spawn (no journalctl, no per-side unit
   lock; the single-instance guard then relies on Ghostty itself).

Caveat: the install/uninstall cycle is CI-validated on a real darwin binary;
floating-panel live validation is accepted as out of scope (the owner has no
macOS hardware), so CI coverage is the support contract.

## Troubleshooting

- **`ghostty not found on PATH`** — install Ghostty
  (<https://ghostty.org/docs/install>) or add its binary to `PATH`.
- **`gdbus not found`** — the quick mode needs GLib's `gdbus`; if the ghostty
  build has GLib it is already there (see `gdbus` in the requirements).
- **`streambeats binary not found`** — run the build command from step 1.
- **`missing preset file`** — the script expects to live in `<repo>/scripts/`
  (or the installed share tree); run it from the repository checkout instead
  of a copy.
- **Two panels after a failed host spawn** — the launcher only retires the
  surfaceless hidden instance it spawned; it never kills a showing panel.
  Close the extra panel by toggle (`streambeats-quick`) or by quitting the
  player (`q`).
- **`ctrl+alt+s` does nothing** — on a pre-1.4 fallback panel the keybind
  cannot raise a normal window (the dedicated window closes and re-opens
  through the launcher); upgrade Ghostty for the quick-terminal mode.
- **`unknown side`** — valid sides are `top`, `bottom`, `left`, `right`.
- **No slide-in animation** — Linux animations require KDE with the "Sliding
  Popups" KWin plugin; other compositors show the panel without animating.
- **Validate a preset or a generated config** —
  `ghostty --config-file=<file> +validate-config`; note that Ghostty only
  takes `--config-file=` as a real flag on the action path when it rides
  before the `+action`, and a plain `--config=` style flag remains invalid
  (ghostty discussions #9434/#10058).
