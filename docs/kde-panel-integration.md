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
# RECOMMENDED: fully keyboard-driven usage — the owner's verified combo:
#   Ctrl+Alt+M → toggle right (no conflicts with default Ghostty/Plasma
#   keybinds on KDE Plasma Wayland); any free combo works.
$HOME/.local/bin/streambeats-panel toggle right
```

Press it anywhere: Ghostty 1.4.0+ quick hosts and kitty toggle the panel
natively (first press opens, next press hides — the wrapper relay handles
it); a pre-1.4 Ghostty host spawns the detached unit on the first press and
the legacy kdotool toggle takes over afterwards.

### Binding styles (Command or Script vs Application)

The global keybind can be attached two ways; the toggle is identical, the
taskbar differs:

| Style | Where (System Settings → Keyboard → Shortcuts) | On every toggle |
|-------|------------------------------------------------|-----------------|
| **Command or Script** → `$HOME/.local/bin/streambeats-panel toggle right` | Add New → Command or Script… | **no icon pulse** — recommended for the daily keybind (owner-verified on Plasma 6) |
| **Application** → the generated `streambeats panel` entry | that entry's own Custom Shortcut | a transient app icon appears next to the other apps for a few seconds, then drops |

Why the pulse: a `.desktop` Application activation registers the app in
Plasma's "starting" queue; the panel itself is an overlay/layer-shell
surface and never becomes a taskbar window, so the pending launcher task
times out and disappears. Harmless and host-agnostic (kitty quick-access
and the ghostty quick terminal behave identically; the same single
quick-host instance serves every toggle — no cold start, no leftover
process).

Switching styles later: in Shortcuts, empty the shortcut on the entry you
no longer want and add it to the other one (Custom Shortcut on the command
entry). The generated `streambeats-panel.desktop` stays installed either
way — its right-click **Actions** (top/bottom/left/right) live there and
need no keybind.

## KRunner / Application Launcher

The installer generates `~/.local/share/applications/streambeats-panel.desktop`
with right-click **Actions** for each side. Exec/Icon are absolute — no PATH
dependence. The Exec launches the **toggle** (v1.7.2+): launching the app or
binding it to a global keybind spawns the panel on first press and
shows/hides it afterwards — binding the app icon is the full toggle story,
no separate command needed (kdotool optional, see the advisory).

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

### Ghostty 1.4.0+ hosts relay instead (issue #134)

On a Ghostty build with the quick-terminal mechanism (see `docs/ghostty.md`),
the wrapper detects the host the same way the launcher does, relays
`streambeats-quick <side>` detached on EVERY press and never engages the
unit guard or kdotool: the launcher's quick mode spawns/claims the per-side
panel on first press and toggles it after. Pre-1.4 ghostty hosts fail the
zero-side-effect capability probe (`ghostty +help | grep
toggle-quick-terminal`) and keep the legacy toggle exactly as documented
above — same wrapper, same shortcut.

The relay's notification is unconditional by design (matches the kitty
relay, issue #108): the press always dispatched a detached launcher and the
launcher owns its own failure messaging (native review R4-001, issue #134
escalated-closure disposition).
