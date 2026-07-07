package av1

import (
	"bytes"
	"fmt"

	"github.com/Eyevinn/mp4ff/bits"
)

// TileRange marks the byte range of a single tile's coded data (the decode_tile structure)
// within an AV1 sample. Everything in the sample outside these ranges (OBU headers and size
// fields, sequence/frame headers, tile-group headers and tile-size fields) is clear and must
// not be encrypted under AV1 common encryption.
type TileRange struct {
	Offset int // byte offset within the sample
	Length int // number of bytes of tile data
}

// GetTileRanges parses a coded AV1 sample (a temporal unit of OBUs) in decode order and
// returns the byte ranges of tile data across all Frame and Tile Group OBUs. The decoder
// state is advanced, so samples must be supplied in decode order.
//
// It parses every OBU_FRAME / OBU_FRAME_HEADER frame header and the following tile group,
// requiring the tile-size fields to account exactly for each OBU payload; a mismatch returns
// an error rather than silently producing wrong ranges.
func (d *FrameHeaderDecoder) GetTileRanges(sample []byte) ([]TileRange, error) {
	var ranges []TileRange
	pos := 0
	for pos < len(sample) {
		hdr, err := ParseOBUHeader(sample[pos:])
		if err != nil {
			return nil, fmt.Errorf("OBU %d header: %w", len(ranges), err)
		}
		pos += hdr.HeaderSize
		var payloadLen int
		if hdr.HasSizeField {
			size, n, err := ReadLEB128(sample[pos:])
			if err != nil {
				return nil, fmt.Errorf("OBU size: %w", err)
			}
			pos += n
			if size > uint64(len(sample)-pos) {
				return nil, fmt.Errorf("OBU payload length %d exceeds remaining data", size)
			}
			payloadLen = int(size)
		} else {
			payloadLen = len(sample) - pos
		}
		payload := sample[pos : pos+payloadLen]

		switch hdr.Type {
		case OBUFrame:
			fh, err := d.ParseFrameHeader(hdr.TemporalID, hdr.SpatialID, payload)
			if err != nil {
				return nil, fmt.Errorf("frame OBU header: %w", err)
			}
			if !fh.ShowExistingFrame {
				tgOffset := pos + fh.HeaderBytes
				tiles, err := tileGroupRanges(sample[tgOffset:pos+payloadLen], fh)
				if err != nil {
					return nil, fmt.Errorf("frame OBU tile group: %w", err)
				}
				for _, t := range tiles {
					ranges = append(ranges, TileRange{Offset: tgOffset + t.Offset, Length: t.Length})
				}
			}
		case OBUFrameHeader, OBURedundantFrameHeader:
			if _, err := d.ParseFrameHeader(hdr.TemporalID, hdr.SpatialID, payload); err != nil {
				return nil, fmt.Errorf("frame header OBU: %w", err)
			}
		case OBUTileGroup:
			if d.lastFrameHeader == nil {
				return nil, fmt.Errorf("tile group OBU before any frame header")
			}
			tiles, err := tileGroupRanges(payload, d.lastFrameHeader)
			if err != nil {
				return nil, fmt.Errorf("tile group OBU: %w", err)
			}
			for _, t := range tiles {
				ranges = append(ranges, TileRange{Offset: pos + t.Offset, Length: t.Length})
			}
		}
		pos += payloadLen
	}
	return ranges, nil
}

// tileGroupRanges implements tile_group_obu() (spec 5.11.1) enough to return the byte range
// of each tile's data within the given tile-group bytes. It requires the tile-size fields to
// account for the whole tile group exactly.
func tileGroupRanges(tg []byte, fh *FrameHeader) ([]TileRange, error) {
	numTiles := fh.NumTiles()
	if numTiles <= 0 {
		return nil, fmt.Errorf("invalid tile count %d", numTiles)
	}
	r := bits.NewReader(bytes.NewReader(tg))
	tileStartAndEndPresent := false
	if numTiles > 1 {
		tileStartAndEndPresent = r.ReadFlag()
	}
	tgStart, tgEnd := 0, numTiles-1
	if numTiles > 1 && tileStartAndEndPresent {
		tileBits := fh.TileColsLog2 + fh.TileRowsLog2
		tgStart = int(r.Read(tileBits))
		tgEnd = int(r.Read(tileBits))
	}
	r.ByteAlign()
	if err := r.AccError(); err != nil {
		return nil, err
	}
	pos := r.NrBytesRead()

	var ranges []TileRange
	for tileNum := tgStart; tileNum <= tgEnd; tileNum++ {
		lastTile := tileNum == tgEnd
		var tileSize int
		if lastTile {
			tileSize = len(tg) - pos
		} else {
			if pos+fh.TileSizeBytes > len(tg) {
				return nil, fmt.Errorf("tile size field exceeds tile group")
			}
			tileSize = int(leToUint(tg[pos:pos+fh.TileSizeBytes])) + 1
			pos += fh.TileSizeBytes
		}
		if tileSize < 0 || pos+tileSize > len(tg) {
			return nil, fmt.Errorf("tile %d size %d exceeds tile group", tileNum, tileSize)
		}
		ranges = append(ranges, TileRange{Offset: pos, Length: tileSize})
		pos += tileSize
	}
	if pos != len(tg) {
		return nil, fmt.Errorf("tile data did not consume tile group exactly (%d of %d bytes)", pos, len(tg))
	}
	return ranges, nil
}

// leToUint reads a little-endian unsigned integer from up to 4 bytes (le(n), spec 4.10.4).
func leToUint(b []byte) uint32 {
	var v uint32
	for i := len(b) - 1; i >= 0; i-- {
		v = (v << 8) | uint32(b[i])
	}
	return v
}
