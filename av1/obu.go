package av1

import (
	"errors"
	"fmt"
)

// OBU parsing errors
var (
	ErrTruncatedOBU    = errors.New("truncated data for AV1 OBU")
	ErrForbiddenBit    = errors.New("obu_forbidden_bit is not 0")
	ErrTruncatedLEB128 = errors.New("truncated LEB128 value")
	ErrTooLongLEB128   = errors.New("LEB128 value uses more than 8 bytes")
)

// OBUType is the type of an Open Bitstream Unit (AV1 spec 6.2.2, Table).
type OBUType uint8

const (
	OBUSequenceHeader       OBUType = 1
	OBUTemporalDelimiter    OBUType = 2
	OBUFrameHeader          OBUType = 3
	OBUTileGroup            OBUType = 4
	OBUMetadata             OBUType = 5
	OBUFrame                OBUType = 6
	OBURedundantFrameHeader OBUType = 7
	OBUTileList             OBUType = 8
	OBUPadding              OBUType = 15
)

func (t OBUType) String() string {
	switch t {
	case OBUSequenceHeader:
		return "SequenceHeader"
	case OBUTemporalDelimiter:
		return "TemporalDelimiter"
	case OBUFrameHeader:
		return "FrameHeader"
	case OBUTileGroup:
		return "TileGroup"
	case OBUMetadata:
		return "Metadata"
	case OBUFrame:
		return "Frame"
	case OBURedundantFrameHeader:
		return "RedundantFrameHeader"
	case OBUTileList:
		return "TileList"
	case OBUPadding:
		return "Padding"
	default:
		return fmt.Sprintf("Reserved(%d)", uint8(t))
	}
}

// OBUHeader is the parsed obu_header() (AV1 spec 5.3.2).
// AV1 OBUs carry no emulation-prevention bytes, so the header is read directly from the raw bytes.
type OBUHeader struct {
	Type          OBUType
	ExtensionFlag bool
	HasSizeField  bool
	TemporalID    byte // valid when ExtensionFlag is set
	SpatialID     byte // valid when ExtensionFlag is set
	HeaderSize    int  // number of header bytes (1 or 2), excluding any obu_size field
}

// ParseOBUHeader parses an obu_header() from the start of data.
// It is strict on obu_forbidden_bit (a reliable corruption signal) but does not
// validate the reserved bits, so that streams from future minor revisions still parse.
func ParseOBUHeader(data []byte) (OBUHeader, error) {
	if len(data) < 1 {
		return OBUHeader{}, ErrTruncatedOBU
	}
	b := data[0]
	if b&0x80 != 0 {
		return OBUHeader{}, ErrForbiddenBit
	}
	h := OBUHeader{
		Type:          OBUType((b >> 3) & 0x0f),
		ExtensionFlag: b&0x04 != 0,
		HasSizeField:  b&0x02 != 0,
		HeaderSize:    1,
	}
	if h.ExtensionFlag {
		if len(data) < 2 {
			return OBUHeader{}, ErrTruncatedOBU
		}
		e := data[1]
		h.TemporalID = e >> 5
		h.SpatialID = (e >> 3) & 0x03
		h.HeaderSize = 2
	}
	return h, nil
}

// ReadLEB128 reads an unsigned LEB128 value (AV1 spec 4.10.5) from the start of data.
// It returns the value and the number of bytes consumed. At most 8 bytes are read.
func ReadLEB128(data []byte) (value uint64, numBytes int, err error) {
	for i := 0; i < 8; i++ {
		if i >= len(data) {
			return 0, 0, ErrTruncatedLEB128
		}
		b := data[i]
		value |= uint64(b&0x7f) << (uint(i) * 7)
		numBytes++
		if b&0x80 == 0 {
			return value, numBytes, nil
		}
	}
	return 0, 0, ErrTooLongLEB128
}

// OBU is a parsed Open Bitstream Unit: its header plus the raw payload.
type OBU struct {
	Header  OBUHeader
	Payload []byte // OBU payload, excluding the header and any obu_size field
}

// SplitOBUs splits a byte slice into OBUs. The input can be an av1C configOBUs
// field, a coded sample, or a full temporal unit.
//
// OBUs with obu_has_size_field set use the signalled size. An OBU without a size
// field is only valid as the last OBU and is assumed to extend to the end of data
// (as in the low-overhead bitstream format and the final configOBU).
func SplitOBUs(data []byte) ([]OBU, error) {
	obus := make([]OBU, 0, 2)
	pos := 0
	for pos < len(data) {
		hdr, err := ParseOBUHeader(data[pos:])
		if err != nil {
			return nil, fmt.Errorf("OBU %d header: %w", len(obus), err)
		}
		pos += hdr.HeaderSize
		var payloadLen int
		if hdr.HasSizeField {
			size, n, err := ReadLEB128(data[pos:])
			if err != nil {
				return nil, fmt.Errorf("OBU %d size: %w", len(obus), err)
			}
			pos += n
			payloadLen = int(size)
		} else {
			payloadLen = len(data) - pos
		}
		if pos+payloadLen > len(data) {
			return nil, fmt.Errorf("OBU %d: payload length %d exceeds remaining data", len(obus), payloadLen)
		}
		obus = append(obus, OBU{Header: hdr, Payload: data[pos : pos+payloadLen]})
		pos += payloadLen
	}
	return obus, nil
}
