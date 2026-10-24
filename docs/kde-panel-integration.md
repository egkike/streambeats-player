# KDE Plasma integration — keybind + floating rules

One action spawns the detached side panel; window rules make it behave like a
pinned floating sheet. Two places matter:

- **Steps** (keybind command, window-rule values): `assets/kwin/window-rules.md`
  — installed at `~/.local/share/streambeats/assets/kwin/window-rules.md`,
  summarized here so the repo READMEs stay short.
- **The launcher**: `streambeats-panel [top|bottom|left|right]` — resolves a
  systemd user unit per side (`streambeats-panel-<side>.service`), refuses to
  duplicate a running panel (best-effort focus + notify instead), and falls
  back to a detached background spawn where systemd is absent.

## Keybind (System Settings → Keyboard → Shortcuts → Add New → Command or Script…)

```sh
$HOME/.local/bin/streambeats-panel right
```

Second press: no duplicate — the per-side lock answers with a notification.
Different sides are separate units on purpose (you may keep two panels).

## KRunner / Application Launcher

The installer generates `~/.local/share/applications/streambeats-panel.desktop`
with right-click **Actions** for each side. Exec/Icon are absolute — no PATH
dependence.

## Window rules (System Settings → Window Management → Window Rules)

Match on **Window class = streambeats-panel** (the presets now set
`class = streambeats-panel`; verified against the local Ghostty's
`+show-config --default`) or title `streambeats player`. Force
Keep-above / Skip-taskbar / Skip-pager; chrome/size/position to taste.
The presets adopt the values the merged ghostty config already accepts
(window-mode, autotype...); when a GTK dialog reports a preset field, see
`docs/ghostty.md` (pseudo-config gotchas, L9/L10).

## Toggle hide/show — deliberately NOT here

No shipped automation toggles visibility from outside the window (Ghostty
1.3.1 lacks a quick-terminal CLI action; KWin scripting would need live
per-release validation). Call it a `q` away: quit inside the panel stops
world (audio included), and the keybind respawns fresh. A separate issue
will explore KWin scripting/kdotool with real KDE evidence when the owner
wants the full toggle dance.
