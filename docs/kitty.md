# Running streambeats in a kitty quick access terminal

The player panel can live in **kitty's native quick access terminal**
(Quake-style layer-shell panel, kitty 0.42+) instead of Ghostty. Spec:
`odd/tasks/panel-terminal-choice.md` (issue #90) — S1 (settings-menu choice),
S5 (launcher precedence), S6 (per-side presets + this doc).

Reference: <https://sw.kovidgoyal.net/kitty/kittens/quick-access-terminal/>
(`edge`, `lines`/`columns`, `hide_on_focus_loss`, `app_id`).

## Choosing the panel terminal

`streambeats-quick` resolves the hosting terminal in one explicit order:

| # | Source | Notes |
| --- | --- | --- |
| 1 | `--terminal=ghostty\|kitty` flag | wins over everything; a wish is validated against PATH, never substituted |
| 2 | `panelTerminal` in `~/.config/streambeats/settings.json` | written by the TUI **`c` → Terminal section**; the launcher reads it with a **top-level anchored scan** (a nested or value-embedded lookalike never resolves — issue #92 hardening); unreadable, malformed or foreign values fall through silently (fail closed) |
| 3 | builtin order | ghostty first (calibrated keycaps, issue #64), then kitty |

The anchored scan reads both machine shapes — the written-by-TUI
2-space MarshalIndent pretty file and a hand-minified flat file — and
treats hand-nested JSON as out of contract (fall-through to the builtin
order instead of a wrong host).

The settings screen commits the change and tells you the choice **applies
after restarting the app** — the terminal hosting the running TUI never
changes underneath you.

## Requirements

- kitty **0.42 or newer** (the quick access terminal landed there).
- `kitten` on PATH (ships with kitty).
- Wayland with `wlr-layer-shell-v1` — the same layer-shell requirement as
  Ghostty's quick terminal. KDE Plasma (KWin) implements it; GNOME does not.
  Kitty's quick access terminal also works on X11 (there is a fixed
  stay-on-top issue under KDE on X11 after a hide/show cycle — prefer
  Wayland).

## Launch and toggle

```sh
streambeats-quick --terminal=kitty right    # one-shot choice
streambeats-quick --terminal=kitty bottom
```

The launcher runs:

```sh
kitten quick-access-terminal --config <assets/kitty/streambeats-<side>.conf> <player binary>
```

Toggle: **run the same command again**. The kitten is single-instance per
instance-group, so a second spawn hides (or restores) the panel. That is
also how a KDE global shortcut binds:

- *System Settings → Keyboard → Shortcuts → Add New → Command or Script…* —
  bind `streambeats-quick` (or the same command with a side) **or the
  installed companion `streambeats-panel toggle <side>`** — the wrapper
  relays kitty on every press (issue #108), so one keybind toggles it
  without kdotool.
- The first press spawns/shows the panel, the next press hides it.

Edge panels stretch their secondary axis to the full screen height/width
(kitty layer-shell design — the panel is not mouse-draggable and has no
minimize button; show/hide is the toggle). To cap the secondary axis, edit
the preset's `columns`/`lines` px value and add `margin_*` offsets.

For a panel that outlives the launcher process (detached, toggle by
desktop entry), `streambeats-panel` (issue #70/#73) wraps this on the
Ghostty path with kdotool today; the kitty path toggle is the re-run
command above — no kdotool needed.

## Size mapping from the Ghostty side table

Ghostty presets use percentages (`quick-terminal-size = 22%`); the kitty
quick access terminal takes only fixed units, so the presets use **pixels**
for the edge's primary axis and leave the other axis maximized:

| Preset | `edge` | primary size | Ghostty equivalent |
| --- | --- | --- | --- |
| `assets/kitty/streambeats-top.conf` | top | `lines 324px` | `quick-terminal-size = 30%` (1080-equivalent) |
| `assets/kitty/streambeats-bottom.conf` | bottom | `lines 324px` | `30%` height |
| `assets/kitty/streambeats-left.conf` | left | `columns 424px` | `quick-terminal-size = 22%` (1920-equivalent) |
| `assets/kitty/streambeats-right.conf` | right | `columns 424px` | `22%` width |

Edit the px value in the preset (or `~/.config/kitty/quick-access-terminal.conf`
for your personal defaults) if your monitor differs. The presets mirror the
Ghostty doctrine on the rest: `hide_on_focus_loss no` (the panel stays open
when you click back into your editor, like `quick-terminal-autohide=false`),
`layer overlay`, and a dedicated `app_id streambeats-panel-kitty` so KDE
window rules can grip it if you customize further.

## Rendering contract inside kitty

The TUI detects the ghostty exact `TERM_PROGRAM=ghostty` and **fails closed
to the pure-ASCII transport tokens** everywhere else (issue #64 doctrine).

## Ghostty parity notes

- The queue engine, prefix caching and quit semantics are terminal-agnostic
  invariants (see the architecture contract); only the hosting window
  changes.
- Ghostty 1.3.1 spawned the panel as a dedicated normal window; kitty
  0.42+ actually opens a real edge-anchored quick access panel from the
  CLI, so the kitty path is today the **fully native** one.
- Stream URLs stay fresh per `loadfile` either way; the catalog cache never
  persists tokens (unchanged contract).
