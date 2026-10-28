# streambeats panel — KDE window rules + global keybind

Optional but complete integration with KDE Plasma, so the panel opens with a
global shortcut and behaves like a pinned floating sheet. All steps are
user-level; nothing touches system files nor other ghosts.

The preset `title = streambeats-panel` (added in every
`assets/ghostty/streambeats-<side>.conf`) is what the rules and spawns below
match.

## 1. Global keybind (2 minutes, fully supported path)

KDE Plasma 6: **System Settings → Keyboard → Shortcuts → Add New → Command or
Script…** and bind it to your shortcut (e.g. `Meta+S`):

```sh
$HOME/.local/bin/streambeats-panel toggle right
```

- `toggle` makes the keybind the full show/hide switch (v1.7.1+): a focused
  panel minimizes (audio keeps playing); a minimized or unfocused panel is
  restored/raised. Requires kdotool — without it the installer advisory
  names both install paths (distro package; upstream jinliu/kdotool
  prebuilt). kdotool 0.3 has no minimized-state query, so the decision
  uses the ACTIVE window only (two-press doctrine).
- It spawns the panel **detached** (systemd user unit
  `streambeats-panel-<side>.service`), so closing terminals never kills it.
  The first press spawns; subsequent presses toggle instead of duplicating
  (the per-side single-instance guard stays underneath: plain
  `streambeats-panel <side>` without `toggle` keeps the legacy spawn/guard
  behavior — "already running").
- Want a different side on the same shortcut? Change the trailing argument to
  `top|bottom|left|right`. Different sides are separate units on purpose, so
  you can keep two at once.

Keyboard-only alternative: the same `streambeats-panel` appears in KRunner
and the Application Launcher via the installed
`streambeats-panel.desktop` (right-click it for the four side actions).

## 2. Window rules (floating/keep-above/no-taskbar feel)

System Settings → **Window Management → Window Rules → Add New…**

Match (use the second tab too if you prefer specificity):

- Window class (application) → `streambeats-panel` (or leave blank and match
  the title instead — see below)
- Window title → `streambeats-panel`

Force in the first rule:

| Property | Check |
|---|---|
| Keep above | Force → Yes |
| Skip taskbar | Force → Yes |
| Skip pager | Force → Yes |

Optional floating-chrome rules (apply to the same rule or a second one):

| Property | Suggested |
|---|---|
| Titlebar color scheme | whatever you use elsewhere |
| No titlebar and frame | No — keep it draggable, per the product promise |
| Size | check "yes" and set your panel size (e.g. 608×720 px) |
| Position | check "yes" (e.g. Right/Center) |

Save. Applies to every future panel spawn; a live-until-change window can be
re-titled or re-spawned to pick it up.

## 3. Scripted rule insertion — deliberately NOT automated

We do not ship a `kwriteconfig`/`kwinrulesrc` snippet: group numbering and
rule keys are Plasma-version-specific implementation details that cannot be
asserted honestly without live per-release validation (repo standard: no
guess-patching). Use the supported System Settings UI in step 2. If you
script it yourself, back up `~/.config/kwinrulesrc` first and verify one
spawn behaves afterwards.
