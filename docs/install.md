# Installing streambeats-player

`scripts/install.sh` turns any machine into a playable setup from one line:

```sh
curl -fsSL https://raw.githubusercontent.com/egkike/streambeats-player/main/scripts/install.sh | bash
```

or, pinned to an exact release (useful in scripts/CI):

```sh
curl -fsSL https://raw.githubusercontent.com/egkike/streambeats-player/main/scripts/install.sh | bash -s -- --version v1.1.0
```

## What it installs

Everything, user-level, under one `--prefix` (default `~/.local`):

| Path | Purpose |
| --- | --- |
| `$PREFIX/share/streambeats/bin/streambeats` | the TUI player binary |
| `$PREFIX/share/streambeats/scripts/streambeats-quick` | floating-panel launcher |
| `$PREFIX/share/streambeats/assets/ghostty/streambeats-<side>.conf` | top/bottom/left/right presets |
| `$PREFIX/share/streambeats/README.md`, `LICENSE` | shipped docs of that release |
| `$PREFIX/bin/streambeats` | wrapper → shared binary |
| `$PREFIX/bin/streambeats-quick` | wrapper → shared launcher |

The shared tree mirrors the repository layout (`bin/` + `scripts/` +
`assets/`) precisely so `streambeats-quick` keeps resolving its tree via
`scripts/..` — the same launcher works from a source checkout and from an
installed prefix.

## Trust model

- Release assets are downloaded from GitHub Releases and **verified
  SHA-256 against that release's `checksums.txt`** before a single file is
  written. A mismatched archive aborts the install.
- The launcher fallback deserves a caveat: release tags whose archive
  predates bundling of `scripts/`/`assets/` (v1.0.0) fetch the launcher and
  presets from `raw.githubusercontent.com` at the same tag, expecting the
  same redirect/transport trust as the `| bash` channel; those files are
  NOT covered by `checksums.txt`. From v1.1.0 on, everything ships inside
  the verified archive.
- `umask 077`, no sudo, no writes outside `--prefix`; temp dirs are cleaned
  via `EXIT`/`INT`/`TERM`/`HUP` traps.

## Requirements (what the installer checks)

| Check | Consequence when missing |
| --- | --- |
| Linux + amd64/arm64 | refusal: the runtime contract needs Unix sockets |
| `mpv` in PATH | advisory; TUI browses but cannot play |
| `ghostty` in PATH | advisory; only the floating panel needs it |
| `$PREFIX/bin` in PATH | advisory with the exact `export PATH` hint |

Network access to `bandcamp.com` and `api.github.com` / `raw.githubusercontent.com`
is required at install time; the player then only needs `bandcamp.com`.

## Uninstall

```sh
rm -rf ~/.local/share/streambeats ~/.local/bin/streambeats ~/.local/bin/streambeats-quick
```

(or with a custom prefix: drop the equivalent paths there).

## Testing the installer

`scripts/tests/install_test.sh` stages a fake release on disk (JSON +
`file://` artifact URLs — no network) and asserts the bundle layout, wrapper
execution, checksum-refusal behavior and the raw fallback: 26 assertions,
must exit 0. Shell hygiene is pinned by `shellcheck` (warm target for CI).
