# AGENTS.md

Instructions for coding agents working on mp4ff. [CONTRIBUTING.md](CONTRIBUTING.md) has the same
conventions for people.

## Committing

Pre-commit hooks are enforced. Activate the venv first: `source venv/bin/activate`.
`pre-commit install` (once per clone) installs both the pre-commit and the commit-msg hook.

Conventional Commits enforced via commitlint: `feat:`, `fix:`, `perf:`, `docs:`, `chore:`, `refactor:`, `test:`, `ci:`.

## Architecture

### Dual encoding/decoding paths

Two parallel I/O paths exist and both must be maintained:
1. **io.Reader/io.Writer** — standard, more flexible
2. **SliceReader/SliceWriter** — preferred for performance (2-10x faster, far fewer allocations)

New boxes follow the pattern in `prft.go` and are registered in both `decoders` (box.go) and `decodersSR` (boxsr.go).

### Sample numbering

External APIs use **1-based** sample numbers (sample 1 = first sample). Internal slice storage is 0-based.

## Key conventions

- Test roundtrips with `boxDiffAfterEncodeAndDecode(t, box)` helper
- Test both io.Reader and SliceReader decode paths where possible
- Primary spec: ISO/IEC 14496-12:2026 (8th edition)
- History (what was broken, why, measurements) goes only in commit messages and PR descriptions.
  Code comments, test comments and changelog entries describe current behaviour.

## Changelog

- Don't edit CHANGELOG.md for a change. Add a file to `changelog.d/` instead, as described in
  [changelog.d/README.md](changelog.d/README.md), so that pull requests don't conflict.
- Name the file after the change and use the Keep a Changelog sections (`### Added`, `### Fixed`,
  ...) with `- ` entries, as in CHANGELOG.md.
- Entries are one or two lines saying what changed for users and naming the affected API or tool.
  Merge related changes into one entry. End a breaking change with "This is a breaking change".
- Tests, CI and refactoring that users don't notice need no entry.
- `scripts/check-changelog.sh` checks the files; it runs in CI and as a pre-commit hook.

## Releases

Pre-1.0, a new API or a breaking change means a minor version, fixes only a patch version.
The release is prepared in a pull request `chore: prepare vX.Y.Z release` on a branch `release-vX.Y.Z`:

1. In CHANGELOG.md, add `## [X.Y.Z] - YYYY-MM-DD` below `## [Unreleased]`. Gather the entries of all
   files in `changelog.d/` under one heading per section, in the order Added, Changed, Deprecated,
   Removed, Fixed, Security, and edit them where it helps, for example to merge related entries.
2. Remove the entry files with `git rm`, so that `changelog.d/` holds only its README.
3. Update the compare links at the end of CHANGELOG.md: `[Unreleased]` compares from `vX.Y.Z`, and a
   new `[X.Y.Z]` link compares the previous version with `vX.Y.Z`.
4. If BENCHMARKS.md has a section titled "(unreleased)", rename it to the version.

While the latest version in CHANGELOG.md has no tag, `scripts/check-changelog.sh` fails if
`changelog.d/` holds entry files, so the `changelog` CI job on master must pass before `vX.Y.Z` is
tagged. Pushing the tag runs `.github/workflows/release.yml`, which runs the same check for the tag
and only then builds the release assets with goreleaser. A tag is permanent for Go modules, since the
module proxy caches it, so never move or reuse one.
