# curl-installer — One-line install for streambeats-player

curl-able installer (mcp-appointments-crm style) that ships the full player
experience (TUI binary + quick-terminal launcher + Ghostty presets) into the
user's home, plus README/docs coverage of prerequisites.

## Specs

- S1: "Lo primero que vamos a hacer es que nuestro proyecto sea Instalable con curl (similar a lo que tenemos para linux en el proyecto mcp que revisaste antes)."
- S2: "(2) Luego documentar todo esto de los pre-requisitos y forma de install en el README.md y donde corresponda."
- S3 (user decision): install scope = "Player completo (recomendado)": binario + scripts/streambeats-quick + presets Ghostty bajo ~/.local/share/streambeats con wrappers en ~/.local/bin; `streambeats-quick right` tras instalar deja andando el panel flotante.
- S4 (user decision): install destination = "~/.local/bin (recomendado)": user-level, sin sudo.
- S5 (prerequisite facts to document, from the session-3 analysis): runtime is Linux-focused by contract; mpv >= 0.41 in PATH is required for playback (TUI degrades gracefully without it); Ghostty 1.3.1+ is required only for the floating panel (binary runs in any terminal); network access to bandcamp.com is required for catalog/streaming; the launch rebuild guard needs Go only for dev checkouts.

## Tasks

- T1 (S1): GoReleaser archive ships the full experience: binary + scripts/streambeats-quick + assets/ghostty/streambeats-*.conf. Commit.
- T2 (S1,S3,S4): scripts/install.sh — resolve latest (or --version) release of egkike/streambeats-player, detect OS/arch, download tar.gz + checksums.txt, verify SHA256, extract bundle to ~/.local/share/streambeats, wrapper scripts in ~/.local/bin, PATH check, mpv/ghostty advisory checks. Commit 50ef0b1 ✅ (live también verificada en local contra GH real: v1.0.0 + raw fallback + "última release" + binario real y presets).
- T3 (S1): installer test harness — staged fake release layout; asserts install layout, wrapper behavior, checksum-mismatch refusal. Commit 50ef0b1 ✅ (26/26) + validación en vivo contra la API real de GitHub (última release, presets, binario real vía wrapper).
- T4 (S2): README (Install + expanded Requirements), docs/install.md, Architecture pointer. Commit (below) ✅.

## Log

- L1 (user): "Antes de seguir me gustaría entender algunas cosas de lo que ya hicimos como: 1- La app que tenemos funciona solo para linux? 2- Funciona solo si tienes instalado Ghostty? Qué otros pre-requisitos se deben tener para que la app funcione? veamos esto y luego te digo cómo seguir" — answered with the dependency map that feeds S5.
- L2 (user): "Lo primero que vamos a hacer es que nuestro proyecto sea Instalable con curl (similar a lo que tenemos para linux en el proyecto mcp que revisaste antes). Luego documentar todo esto de los pre-requisitos y forma de install en el README.md y donde corresponda. Qué opinas?" + decisions S3/S4 via Q&A.
- L3 (agent decisions): installer pulls release artifacts from GitHub Releases (checksum-verified tarball); launcher assets missing from the current v1.0.0 archive are fetched from raw.githubusercontent at the same tag (graceful fallback until the reshaped archive ships); launcher repo-root resolution preserved by keeping the extracted layout bin/ + scripts/ + assets/ under the shared prefix.

- L4 (closing, agent): T4 delivered. README now opens with the curl one-liner and an expanded Requirements map (Linux-only runtime, mpv 0.41+, bandcamp network, Ghostty only for the panel, Go only for checkouts); docs/install.md details the installed layout, trust model (SHA-256 before any write; raw fallback NOT checksum covered from v1.0.0 era tags), requirement checks and uninstall. Slice boundary: T1+T2+T3 commits 1a296cf + 50ef0b1 reached 628 lines; this docs slice ships alone. Verification: gofmt/build/vet/test green, shellcheck clean on both scripts, harness 26/26, plus live installs against the real GitHub API (--version and latest paths) with the real binary answering help and presets in place. Note: the curl one-liner becomes fully archive-carrying only after a follow-up release with the reshaped goreleaser archive (v1.1.0+); until then raw fallback covers the launcher assets.
- L5 (delivery, user authorized "Dale todo"): PR #2 opened (CI green: Detect+Verify Go jobs), merged to main (0134d68, merge commit per repo convention), annotated tag v1.1.0 pushed; release workflow green (Publish release 1m0s). Post-release verification: gh release assets include the 5 platform archives + checksums.txt; fresh real install with --version v1.1.0 shows NO raw-fallback warning — the one-liner is now 100% archive/cchecksum-backed; artifacts/README/LICENSE ride in every archive. Feature CLOSED.
