# Quality sweep — informational review advisories (issue #21)

Address the informational WARNINGs recorded by the approved RDD reviews of
the 2026-10-25 cycle. All reviews closed APPROVED; this sweep re-derives each
finding against the current merged tree, fixes the real ones and documents
the no-ops.

## Specs

- S1 (issue #21, derived R3-002): "The removal loop calls `os.RemoveAll` on
  **every** target, including targets already counted absent by the `Lstat`
  pre-count. `RemoveAll` returns `nil` for a nonexistent path, so absent
  installs are printed as `removed: …`. The uninstall output can lie about
  what it deleted." → Skip targets already absent; truthful `removed`/`absent`.
- S2 (issue #21, derived R3-003): "runUninstall's doc comment promises exit
  code `1` 'when removals were attempted and failed'; the implementation has
  a single `return 0`." → exit 1 when any removal failed.
- S3 (issue #21, derived R4-001): summary line reports removed/absent but not
  failed → `uninstalled: N path(s) removed, M already absent, K failed`.
- S4 (issue #21, derived R4-002): the uninstaller never says which target set
  it used → print the source (manifest + entry count) or the known-layout
  fallback notice.
- S5 (issue #21, derived R3-001): two different manifest rejections (relative
  line vs outside-prefix line) print the same message → differentiate them.
- S6 (issue #21, derived R1-manifest-path-scope): manifest lines are validated
  lexically only → resolve each remaining target with `EvalSymlinks` and
  re-verify prefix containment before removal; reject with a warning otherwise.
- S7 (issue #21, derived macOS R4-001): failed `mkdir` of the desktop
  integration directories kills the whole install → degrade to warn + skip the
  optional integration; the core install (binary, wrappers, manifest) always
  succeeds.
- S8 (issue #21 disposition duty): curl-installer and KDE-panel advisory
  locations re-judged against the current code; every not-fixed item gets an
  explicit `no-op (reason)` disposition in the PR and the ODD log.
- S9 (negative controls): `TestUninstallCacheFlagReachesUninstallerR31` and
  `TestOtherCommandsStillRejectExtraArguments` stay green; the cache test
  keeps pinning XDG_CACHE_HOME; differential check of sibling commands
  (`help`, unknown, `uninstall` idempotent) vs main shows only intentional
  output changes; queue/stream/mpv invariants untouched.

## Tasks

- T1 (S1-S6,S9): uninstaller fixes in `cmd/streambeats` (runtime + tests,
  RED-first per rule). Commit.
- T2 (S7): `scripts/install.sh` optional-dirs degrade + `install_test.sh`
  scenario. Commit.
- T3 (S8,S9): no-op dispositions documented in odd doc log + PR body; full
  pipeline + differential check. Commit.

## Log

- L1 (2026-10-25): User: "El punto 3 de qué se trata? Qué es eso pendiente?" —
  agent re-derived the receipt warnings (receipts store ID+location only, no
  prose) and opened issue #21 with the disposition table.
- L2 (2026-10-25): User: "Hagamos el pulido de eso y aprovechamos y metemos
  todos los ajustes. Te parece bien?" — sweep authorized; issue-first flow.
- L3 (2026-10-25): Issue #21 opened with scope/negative controls; owner:
  "Approved y dale". Branch `fix/quality-sweep`; spec source locked.
- L4 (2026-10-25): Adversive dispositions for the items re-judged and not
  fixing code (S8):
  - curl-installer `R2-001` (install.sh env-var block at the reviewed tree,
    ~:46) → `no-op`: the STREAMBEATS_* override hooks are a published
    contract since v1.1.0 (documented in docs/install.md); renaming them
    would break user environments for no behavior gain.
  - curl-installer `R3-001` (OS_FLAVOR case, ~:57) → `no-op`: style-only,
    the case/if is 2.2-compliant and shellcheck-clean.
  - curl-installer `R3-002` (curl|bash usage example, ~:108) → `no-op`:
    the trust model (installer executes before any artifact checksum; raw
    fallback not checksum-covered) is documented in docs/install.md since
    the feature; the README one-liner is the published curl entry point.
  - curl-installer `R3-003` (install_test.sh:102 readability) → `no-op`:
    the harness has been rewritten twice since (43 → 66 assertions); the
    flagged shape no longer exists.
  - curl-installer `R4-001` (PREFIX home fallback ~:45) → `no-op`:
    `${HOME:-$PWD}` is malformed-environment defense, not a correctness
    claim; the harness pins its own prefix.
  - curl-installer `R4-002` (fetch/verify region ~:205-260) → `no-op`:
    archive names are version+os+arch unique so the basename anchor is
    unambiguous; checksum mismatch refusal is pinned by scenario 2. The
    basename-uniqueness class was the REAL bug fixed in the KDE cycle
    (b9c6f8a release-JSON parsing) — that fix already covers the class
    where it actually bit.
  - kde-panel `R2-001` (window-rules.md example keybind "e.g. Meta+S" and
    :37 area) → `no-op`: the example is illustrative; the owner's real
    keybind (Ctrl+Alt+M) is recorded in odd/tasks/kde-panel-integration.md
    L5 and the doc makes no claim that it must match.
  - kde-panel `R3-001/002`, `R4-001/002` (streambeats-panel guard
    region advisories) → `no-op`: the per-side guard semantics were live
    -validated on the owner's KDE session (L4/L5) and the script is
    covered by desktop_integration_test.sh 21/21; judge the concrete
    findings as superseded by that live evidence.
- L5 (2026-10-25, T1/T2 delivered): commits `b2a4500` (uninstaller
  truthfulness/contract/hardening, 6 RED→GREEN tests) and `5b08204`
  (optional desktop dirs degrade + harness scenario 9). Pipeline: gofmt
  clean, vet/build/test green, install harness 66/66, desktop 21/21,
  shellcheck clean; differential vs main shows ONLY the intentional S4
  source-notice line for sibling commands (help/bogus byte-identical).
  Race detector n/a: IPC/event packages untouched. Native review of the
  committed candidate follows as the next step (governance step 2).
