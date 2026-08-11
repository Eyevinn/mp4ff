# CLAUDE.md

## Committing

Pre-commit hooks are enforced. Activate the venv first: `source venv/bin/activate`.

Conventional Commits enforced via commitlint: `feat:`, `fix:`, `docs:`, `chore:`, `refactor:`, `test:`, `ci:`.

## Architecture

### File model

`mp4.File` is the top-level container:
- **Progressive**: Ftyp + Moov + Mdat (all metadata in Moov, samples in Mdat)
- **Fragmented**: Init (Ftyp + Moov) + Segments, each containing Fragments (Moof + Mdat pairs)

### Box implementation pattern

Every box type follows a strict pattern (see `prft.go` for a minimal example):
- File: `{boxname}.go`, struct: `{BoxName}Box`
- Required methods: `Type()`, `Size()`, `Encode(io.Writer)`, `EncodeSW(bits.SliceWriter)`, `Info()`
- Decoding functions: `Decode{BoxName}(hdr, startPos, r)` and `Decode{BoxName}SR(hdr, startPos, sr)`
- Registered in dispatch tables: `decoders` and `decodersSR`

### Dual encoding/decoding paths

Two parallel I/O paths exist and both must be maintained:
1. **io.Reader/io.Writer** — standard, more flexible
2. **SliceReader/SliceWriter** — preferred for performance (2-10x faster, far fewer allocations)

### Container box pattern

Container boxes hold `Children []Box` plus direct references to common children (e.g., `MoovBox` has `Trak *TrakBox` and `Traks []*TrakBox`). `AddChild()` updates both.

### Streaming/lazy processing

- `DecModeLazyMdat` — skips reading mdat payload into memory
- `StreamFile` / `InitDecodeStream` / `ProcessFragments` — incremental fragment processing with callbacks
- `BoxSeekReader` — emulates seeking on non-seekable streams

### Sample numbering

External APIs use **1-based** sample numbers (sample 1 = first sample). Internal slice storage is 0-based.

## Key conventions

- Test roundtrips with `boxDiffAfterEncodeAndDecode(t, box)` helper
- Test both io.Reader and SliceReader decode paths where possible
- Primary spec: ISO/IEC 14496-12:2026 (8th edition)
