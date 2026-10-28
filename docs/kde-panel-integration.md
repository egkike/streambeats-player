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

## Toggle hide/show — shipped (kdotool toggle, issue #70)

`streambeats-panel toggle <side>` implements the two-press toggle: focused
panel → `windowminimize` (audio keeps playing); unfocused → `windowactivate`
(restores minimized, raises visible-unfocused). kdotool 0.3 has no
minimized-state query, so the decision uses the ACTIVE window only. The
installer warns with both install paths when kdotool is missing (Linux +
ghostty). Plain `streambeats-panel <side>` keeps the spawn-and-guard
behavior, and quitting inside the panel (`q`) still stops the world.
