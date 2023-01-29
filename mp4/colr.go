package mp4

import (
	"encoding/hex"
	"fmt"
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

const (
	colrType            = "colr"
	onScreenColors      = "nclx"
	restrictedICCType   = "rICC"
	unrestrictedICCType = "prof"
	fullRangeBit        = 0x80
)

// ColrBox i colr box defined in ISO/IEC 14496-2 2021 12.1.5
type ColrBox struct {
	ColorType               string
	ICCProfile              []byte
	ColorPrimaries          uint16
	TransferCharacteristics uint16
	MatrixCoefficients      uint16
	FullRangeFlag           bool
}

// DecodeColr decodes a ColrBox
func DecodeColr(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	sr := bits.NewFixedSliceReader(data)
	return DecodeColrSR(hdr, startPos, sr)
}

// DecodeColrSR decodes a ColrBox from a SliceReader
func DecodeColrSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	c := ColrBox{
		ColorType: sr.ReadFixedLengthString(4),
	}
	switch c.ColorType {
	case onScreenColors:
		c.ColorPrimaries = sr.ReadUint16()
		c.TransferCharacteristics = sr.ReadUint16()
		c.MatrixCoefficients = sr.ReadUint16()
		b := sr.ReadUint8()
		c.FullRangeFlag = (b & fullRangeBit) == fullRangeBit
	case restrictedICCType, unrestrictedICCType:
		c.ICCProfile = sr.RemainingBytes()
	default:
		return nil, fmt.Errorf("unknown color type %q", c.ColorType)
	}
	return &c, sr.AccError()
}

// Type returns the box type
func (c *ColrBox) Type() string {
	return colrType
}

// Size returns the calculated size of the box
func (c *ColrBox) Size() uint64 {
	var size uint64 = 8 + 4
	switch c.ColorType {
	case onScreenColors:
		size += 7
	case restrictedICCType, unrestrictedICCType:
		size += uint64(len(c.ICCProfile))
	}
	return size
}

// Encode writes box to w
func (c *ColrBox) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(c.Size()))
	err := c.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW writes box to sw
func (c *ColrBox) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderSW(c, sw)
	if err != nil {
		return err
	}
	sw.WriteString(c.ColorType, false)
	switch c.ColorType {
	case onScreenColors:
		sw.WriteUint16(c.ColorPrimaries)
		sw.WriteUint16(c.TransferCharacteristics)
		sw.WriteUint16(c.MatrixCoefficients)
		b := byte(0)
		if c.FullRangeFlag {
			b = fullRangeBit
		}
		sw.WriteUint8(b)
	default:
		sw.WriteBytes(c.ICCProfile)
	}
	return sw.AccError()
}

// Info writes box information
func (c *ColrBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) (err error) {
	bd := newInfoDumper(w, indent, c, -1, 0)
	bd.write(" - colorType: %s", c.ColorType)
	switch c.ColorType {
	case onScreenColors:
		bd.write(" - ColorPrimaries: %d, TransferCharacteristics: %d, MatrixCoefficients: %d, FullRange: %t",
			c.ColorPrimaries, c.TransferCharacteristics, c.MatrixCoefficients, c.FullRangeFlag)
	default:
		bd.write(" - ICCProfile: %s", hex.EncodeToString(c.ICCProfile))

	}
	return bd.err
}
