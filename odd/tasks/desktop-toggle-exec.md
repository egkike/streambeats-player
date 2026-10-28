- L3 (2026-10-28): owner live validation AFTER v1.7.2 install — toggle
  still one-directional via the app-bound keybind even with the new
  Exec on disk. Debug chain: instrumented installed script (set -x +
  PPID trace, restored byte-exact after diagnosis) proved the keybind
  launch still ran the OLD Exec (args "right", MODE=spawn) — the stale
  KService lives in the PLASMA SESSION's memory (parent process was
  systemd --user; Plasma 6 launches app shortcuts via transient units),
  and neither kbuildsycoca6 nor a System-Settings re-register refreshed
  it. Session logout/login re-resolved the entry: owner confirmed both
  toggle directions working ("Funciona perfecto"). No code defect in
  v1.7.2; the caching is upstream Plasma behavior worth an advisory in
  docs (users upgrading the desktop entry need a session restart).
- L3 (2026-10-28): owner live validation AFTER v1.7.2 install — toggle
  still one-directional via the app-bound keybind even with the new
  Exec on disk. Debug chain: instrumented installed script (set -x +
  PPID trace, restored byte-exact after diagnosis) proved the keybind
  launch still ran the OLD Exec (args "right", MODE=spawn) — the stale
  KService lives in the PLASMA SESSION's memory (parent process was
  systemd --user; Plasma 6 launches app shortcuts via transient units),
  and neither kbuildsycoca6 nor a System-Settings re-register refreshed
  it. Session logout/login re-resolved the entry: owner confirmed both
  toggle directions working ("Funciona perfecto"). No code defect in
  v1.7.2; the caching is upstream Plasma behavior worth an advisory in
  docs (users upgrading the desktop entry need a session restart).
