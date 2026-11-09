# Contributing to streambeats

Thanks for taking the time to help. Read this before you open anything — it is
short, and it is the whole policy.

## What this repository accepts: issues only

This project takes **issues, not pull requests**.

| You want to… | Do this |
|---|---|
| Report something broken | Open a [Bug Report](https://github.com/egkike/streambeats-player/issues/new?template=bug_report.yml) |
| Propose a feature or a change | Open a [Feature Request](https://github.com/egkike/streambeats-player/issues/new?template=feature_request.yml) |
| Ask about install, panels or usage | Read the [install guide](./docs/install.md), [Ghostty](./docs/ghostty.md), [kitty](./docs/kitty.md) and [KDE Plasma](./docs/kde-panel-integration.md) pages first; open a Bug Report if a documented step fails or the docs are wrong |

Pull requests are closed automatically, without review, with a link back to this
file (`.github/workflows/close-unsolicited-prs.yml`). That is a stated policy,
not a judgement about your change.

**Why:** delivery here is single-owner and runs through an internal gate. Every
candidate is implemented task by task with tracked evidence and must pass a
review gate that only the maintainer can drive. Code arriving from outside that
gate cannot be evaluated or granted a receipt, so it cannot be merged. Issues are
the input that flow accepts.

**Scope:** this rule binds outside contributors. The maintainer's own changes —
documentation included — land as issue-linked pull requests through the gates
described in [`AGENTS.md`](./AGENTS.md).

You do not need to label, assign, claim, or comment "I'll take this" — there is
nothing for you to reserve, and the issue form stamps its one `type:` label by
itself.

## Before you open an issue

1. **Search first** — open *and* closed issues. Duplicates get closed, not answered.
2. **Read the doc that should have covered it** — [install, requirements and
   uninstall](./docs/install.md), [Ghostty panels](./docs/ghostty.md) (which has a
   troubleshooting section), [kitty panels](./docs/kitty.md), [KDE Plasma
   integration](./docs/kde-panel-integration.md).
3. **One issue per report.** Two bugs are two issues.

## Reporting a bug

Include:

- **Version** — the release you installed (for example `v1.1.0`), or
  `git rev-parse --short HEAD` if you built from source. There is no `--version`
  flag yet; do not guess one, and do not report its absence as a bug.
- **Platform** — Linux (KDE Plasma Wayland), another Wayland compositor, X11, or
  macOS (Apple Silicon / Intel).
- **Install path** — the one-line installer, a pinned `--version`, a release
  tarball, or built from source.
- **mpv** — whether it is installed and its version (`mpv --version`). Playback
  needs mpv; browsing the catalog does not.
- **Steps to reproduce**, minimal and in order.
- **Expected** vs **actual** — quote the status line the player displayed. Failures
  are user-facing text by design, so the wording you saw is the most useful thing
  you can paste.
- **Evidence** — run `streambeats` (or `./bin/streambeats` from a checkout) in a
  shell and paste what it printed, plus your `$TERM`.

> **Keep your machine yours.** This project has no accounts, no API keys and no
> server: there is nothing secret to leak, but redact personal paths and hostnames
> if you paste a log.

## Requesting a feature

Lead with the problem, not the solution: what you cannot do today, and what it
costs you. Then the shape you have in mind, then what you already considered and
discarded. A request that names the problem can be redesigned; one that only
names an implementation usually gets closed as out of scope.

## What happens after you submit

- The maintainer triages every issue, alone. There is no SLA; silence means "not
  planned yet", never "rejected".
- You may be asked for information. An issue that stays unanswered for a long
  time may be closed — reopening it is fine.
- Accepted work is implemented and delivered by the maintainer under the internal
  gates described in [`AGENTS.md`](./AGENTS.md). That file is internal: it is worth
  reading for how this codebase is built, but nothing in it is a task assigned to
  you.

## Security

**Do not report a vulnerability in a public issue.** Use the *Security* tab of
this repository → *Report a vulnerability*, which opens a private advisory only
the maintainer can read. Give a reasonable window to fix it before disclosing
anything publicly; if the private form is unavailable, contact `@egkike` directly
instead.

The surface worth reporting: the curl-based installer and its checksum
verification, Bandcamp stream URL handling, and the local mpv unix socket.

## Code of conduct

Be direct and be decent. Technical disagreement is welcome; contempt is not.
