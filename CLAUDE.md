# CLAUDE.md

## Committing

Pre-commit hooks are enforced. Activate the venv first: `source venv/bin/activate`.

Conventional Commits enforced via commitlint: `feat:`, `fix:`, `docs:`, `chore:`, `refactor:`, `test:`, `ci:`.

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
