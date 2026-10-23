# streambeats — Terminal music player for StreamBeats

Floating Ghostty quick-terminal TUI player that streams/plays the StreamBeats
catalog while the user works in (Ghostty + herdr + gentle-shell).

## Specs

- S1: "quiero arrancar un proyecto para hacer un reproductor de música stream que utilice las músicas de https://streambeats.bandcamp.com/music"
- S2: "El reproductor debe ser para mi ambiente de trabajo en la Terminal (Ghostty + herdr + gentle-shell)"
- S3: "debe ser como una ventanita flotante que yo pueda ubicarla en cualquiera de los costados de mi pantalla de terminal"
- S4: "la idea es que lo pueda utilizar mientras trabajo en desarrollo y escuchar música"
- S5 (decision Q&A): fuente mixta — "Mixto (cache + streaming)": librería local como base, y opcionalmente búsqueda/resolución streaming para tracks que no tenga descargados.
- S6 (decision Q&A): stack — "Go — Bubbletea": bubbletea+lipgloss+bubbles para la UI, audio via mpv con IPC.
- S7 (decision Q&A): ventana — "Quick Terminal Ghostty": panel deslizable toggled con atajo, configurable por costado (top/bottom/left/right).
- S8 (Q&A): el entorno del usuario incluye herdr (multiplexer de terminal con server); el player corre en su propio quick terminal de Ghostty, independiente de herdr.

## Tasks

- T1 (S1,S6): Scaffold Go repo: module `streambeats`, cmd CLI entry, package layout. Commit.
- T2 (S1): Catalog: scrape álbumes/tracks de streambeats.bandcamp.com, cache local JSON del catálogo. Commit.
- T3 (S1,S6): Audio backend: cliente mpv --input-ipc-server (play/pause/stop/seek/volume, eventos). Commit.
- T4 (S5): Local library: sync de álbumes Bandcamp ($0) a ~/Music/StreamBeats, player local-first. Commit.
- T5 (S5): Streaming resolver: resolver track → YouTube/SoundCloud via yt-dlp, opcional cache de audio. Commit.
- T6 (S1,S4): Bubbletea TUI: browse catálogo/álbumes, now playing, controles, playlists, atajos de teclado. Commit.
- T7 (S3,S7,S8): Ghostty integration: configs preset por costado (quick-terminal-position), keybind, launcher `streambeats-quick`. Commit.
- T8 (S1..S8): End-to-end verify + README/docs. Commit.

## Log

- L1 (user, verbatim): "Hola capo, quiero arrancar un proyecto para hacer un reproductor de música stream que utilice las músicas de https://streambeats.bandcamp.com/music ... El reproductor debe ser para mi ambiente de trabajo en la Terminal (Ghostty + herdr + gentle-shell) y debe ser como una ventanita flotante que yo pueda ubicarla en cualquiera de los costados de mi pantalla de terminal... la idea es que lo pueda utilizar mientras trabajo en desarrollo y escuchar música. Si necesitas más datos hazme preguntas y te las voy respindiendo"
- L2 (user decisions Q&A): Mixto (cache + streaming) / Go Bubbletea / Quick Terminal Ghostty / herdr = runtime de coding agents (multiplexer).
- L3 (environment): Ubuntu 26.04.1, Go 1.27.3, Ghostty 1.3.1, apt disponible; mpv/yt-dlp/bandcamp-dl NO instalados aún (bloquea T3/T5 en runtime, no en código).
