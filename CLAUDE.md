# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What is mp4ff?

Go library and tools for parsing, writing, and manipulating MP4/ISOBMFF (ISO Base Media File Format) files. Specialized for fragmented MP4 (MPEG-DASH, MSS, HLS fMP4), with support for many video codecs (AVC, HEVC, AV1, VVC, AVS3), audio codecs (AAC, AC-3, Opus, FLAC), subtitle formats (WebVTT, TTML), and CENC/CBCS encryption.

## Commands

```bash
make build          # Build all CLI tools and examples
make test           # go test ./...
make check          # Full quality check (prepare + pre-commit + codespell)
make coverage       # Generate coverage reports (HTML + text)
go test ./mp4/      # Test a single package
go test ./mp4/ -run TestName  # Run a single test
```

Pre-commit hooks (trailing-whitespace, go-fmt, golangci-lint, go-unit-tests, commitlint) are enforced. Activate the venv before committing: `source venv/bin/activate`.

## Commit Convention

Conventional Commits enforced via commitlint: `feat:`, `fix:`, `docs:`, `chore:`, `refactor:`, `test:`, `ci:`.

## Architecture

### Package structure

- **mp4** — Core package. Box encoding/decoding, file/fragment/segment structures, crypto, streaming.
- **bits** — Bit-level I/O primitives (SliceReader/SliceWriter, FixedSliceReader/FixedSliceWriter).
- **avc**, **hevc**, **vvc**, **av1** — Video codec handling (NALU parsing, parameter set extraction).
- **sei** — Supplementary Enhancement Information messages for AVC/HEVC.
- **aac** — AAC audio (ADTS headers, codec descriptions).
- **cmd/** — CLI tools (mp4ff-info, mp4ff-crop, mp4ff-encrypt, mp4ff-decrypt, etc.).
- **examples/** — Usage demonstrations.

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

- golangci-lint with max line length 140 (`lll` linter)
- Only external dependency: `github.com/go-test/deep`
- Test roundtrips with `boxDiffAfterEncodeAndDecode(t, box)` helper
- Test both io.Reader and SliceReader decode paths where possible
- Primary spec: ISO/IEC 14496-12:2021 (7th edition)
