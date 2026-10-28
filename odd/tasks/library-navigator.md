# Navigable library view with XDG music root (issue #76, follows #75)

The `l` view hardcodes an English `$HOME/Music/StreamBeats` scan root
(sync.go:30). On the owner's es-locale system the real XDG music dir is
`$HOME/Música`, so the view showed "(0 files)" in a folder they never use.
Branch: `feat/library-navigator`.

## Specs

- S1: "el default del root leo la carpeta de música de tu configuración
  XDG (xdg-user-dirs, que en tu sistema dice Música)" — default
  libraryRoot = XDG user music dir (en systems: `Music`, unchanged);
  missing xdg config → `$HOME/Music` fallback.
- S2: "ese default se persiste en settings.json (libraryRoot)" — the
  resolved default persists to settings.json and is user-editable; the
  Config screen shows the current root.
- S3: "la vista lista solo la carpeta actual: directorios primero,
  archivos de audio después (Enter en directorio = entrar; .. = subir;
  Enter en audio = reproducir vía mpv)" — lazy per-directory listing,
  NO recursive walk (a $HOME root must not trigger a full-disk scan).
- S4: "el sync pasa a escribir en <libraryRoot>/StreamBeats" — en
  systems unchanged ($HOME/Music/StreamBeats); es systems land in
  Música/StreamBeats. Uninstall/docs updated accordingly.
- S5: untouched: keybind table (l opens the library view as today),
  playback state machine, idle-active guard.

## Tasks

- T1: branch + this doc + Engram mirror.
- T2 (S1): RED — XDG music dir resolution (parsed from user-dirs.dirs,
  en fallback).
- T3 (S3): RED — lazy navigator state (list current dir, descend, ascend,
  play absolute path); fake fs, no network.
- T4 (S2): settings persistence round-trip for libraryRoot + Config row.
- T5 (S4): sync destination = <root>/StreamBeats + uninstall/docs.
- T6: full pipeline + native review + PR Closes #76.

## Log

- L1 (owner, 2026-10-28): the library view showed
  "Local library (0 files) /home/kike/Music/StreamBeats" while their real
  folder is $HOME/Música; owner asked for a navigable root defaulting to
  $HOME. Agreed design: lazy navigation + XDG default + settings
  persistence. Owner: "Dale a los 2 ciclos".
