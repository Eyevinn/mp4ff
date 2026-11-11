package mp4

import (
	"encoding/hex"
	"fmt"
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

const (
	colrType                         = "colr"
	ColorTypeOnScreenColors          = "nclx" // on-screen colours acc. to ISO/IEC 14496-12 Sec. 12.1.5.2
	ColorTypeRestrictedICCProfile    = "rICC" // restricted ICC profile acc. to ISO/IEC 14496-12 Sec. 12.1.5.2
	ColorTypeUnrestrictedICCTProfile = "prof" // unrestricted ICC profile acc. to ISO/IEC 14496-12 Sec. 12.1.5.2
	// QuickTimeColorParameters defined in [nclc]
	//
	// [nclc]: https://developer.apple.com/library/archive/technotes/tn2162/_index.html#//apple_ref/doc/uid/DTS40013070-CH1-TNTAG10
	QuickTimeColorParameters = "nclc"
	fullRangeBit             = 0x80
)

// ColrBox is colr box defined in ISO/IEC 14496-12 2021 Sec. 12.1.5.
type ColrBox struct {
	ColorType               string
	ICCProfile              []byte
	ColorPrimaries          uint16
	TransferCharacteristics uint16
	MatrixCoefficients      uint16
	FullRangeFlag           bool
	UnknownPayload          []byte
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
	const colorTypeLen = 4
	c := ColrBox{
		ColorType: sr.ReadFixedLengthString(colorTypeLen),
	}
	switch c.ColorType {
	case ColorTypeOnScreenColors:
		c.ColorPrimaries = sr.ReadUint16()
		c.TransferCharacteristics = sr.ReadUint16()
		c.MatrixCoefficients = sr.ReadUint16()
		b := sr.ReadUint8()
		c.FullRangeFlag = (b & fullRangeBit) == fullRangeBit
	case ColorTypeRestrictedICCProfile, ColorTypeUnrestrictedICCTProfile:
		c.ICCProfile = sr.ReadBytes(hdr.payloadLen() - colorTypeLen)
	case QuickTimeColorParameters:
		c.ColorPrimaries = sr.ReadUint16()
		c.TransferCharacteristics = sr.ReadUint16()
		c.MatrixCoefficients = sr.ReadUint16()
	default:
		c.UnknownPayload = sr.ReadBytes(hdr.payloadLen() - colorTypeLen)
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
	case ColorTypeOnScreenColors:
		size += 7
	case ColorTypeRestrictedICCProfile, ColorTypeUnrestrictedICCTProfile:
		size += uint64(len(c.ICCProfile))
	case QuickTimeColorParameters:
		size += 6
	default:
		size += uint64(len(c.UnknownPayload))
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
	case ColorTypeOnScreenColors:
		sw.WriteUint16(c.ColorPrimaries)
		sw.WriteUint16(c.TransferCharacteristics)
		sw.WriteUint16(c.MatrixCoefficients)
		b := byte(0)
		if c.FullRangeFlag {
			b = fullRangeBit
		}
		sw.WriteUint8(b)
	case ColorTypeRestrictedICCProfile, ColorTypeUnrestrictedICCTProfile:
		sw.WriteBytes(c.ICCProfile)
	case QuickTimeColorParameters:
		sw.WriteUint16(c.ColorPrimaries)
		sw.WriteUint16(c.TransferCharacteristics)
		sw.WriteUint16(c.MatrixCoefficients)
	default:
		sw.WriteBytes(c.UnknownPayload)
	}
	return sw.AccError()
}

// Info writes box information
func (c *ColrBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) (err error) {
	bd := newInfoDumper(w, indent, c, -1, 0)
	bd.write(" - colorType: %s", c.ColorType)
	switch c.ColorType {
	case ColorTypeOnScreenColors:
		bd.write(" - %s, FullRange: %t", c.codePointsInfo(), c.FullRangeFlag)
	case ColorTypeRestrictedICCProfile, ColorTypeUnrestrictedICCTProfile:
		bd.write(" - ICCProfile: %s", hex.EncodeToString(c.ICCProfile))
	case QuickTimeColorParameters:
		bd.write(" - %s", c.codePointsInfo())
	default:
		bd.write(" - Payload: %s", hex.EncodeToString(c.UnknownPayload))
	}
	return bd.err
}

// codePointsInfo returns the colour primaries, transfer characteristics and matrix coefficients with their names,
// such as "ColorPrimaries: 9 (BT.2020), TransferCharacteristics: 16 (SMPTE ST 2084 PQ), ...".
func (c *ColrBox) codePointsInfo() string {
	return fmt.Sprintf("ColorPrimaries: %s, TransferCharacteristics: %s, MatrixCoefficients: %s",
		codePointInfo(c.ColorPrimaries, colorPrimariesNames),
		codePointInfo(c.TransferCharacteristics, transferCharacteristicsNames),
		codePointInfo(c.MatrixCoefficients, matrixCoefficientsNames))
}

// codePointInfo returns value with its name in names, if there is one.
func codePointInfo(value uint16, names map[uint16]string) string {
	if name, ok := names[value]; ok {
		return fmt.Sprintf("%d (%s)", value, name)
	}
	return fmt.Sprintf("%d", value)
}

// Names of the code points of ISO/IEC 23091-2 (ITU-T H.273), Tables 3, 4 and 5. Reserved values have no name.
var (
	colorPrimariesNames = map[uint16]string{
		1:  "BT.709",
		2:  "unspecified",
		4:  "BT.470 System M",
		5:  "BT.601 625",
		6:  "BT.601 525",
		7:  "SMPTE ST 240",
		8:  "generic film",
		9:  "BT.2020",
		10: "SMPTE ST 428-1 XYZ",
		11: "SMPTE RP 431-2 DCI-P3",
		12: "SMPTE EG 432-1 P3-D65",
		22: "EBU Tech. 3213-E",
	}
	transferCharacteristicsNames = map[uint16]string{
		1:  "BT.709",
		2:  "unspecified",
		4:  "BT.470 System M",
		5:  "BT.470 System B, G",
		6:  "BT.601",
		7:  "SMPTE ST 240",
		8:  "linear",
		9:  "logarithmic 100:1",
		10: "logarithmic 316:1",
		11: "IEC 61966-2-4 xvYCC",
		12: "BT.1361 extended",
		13: "IEC 61966-2-1 sRGB",
		14: "BT.2020 10-bit",
		15: "BT.2020 12-bit",
		16: "SMPTE ST 2084 PQ",
		17: "SMPTE ST 428-1",
		18: "ARIB STD-B67 HLG",
	}
	matrixCoefficientsNames = map[uint16]string{
		0:  "identity",
		1:  "BT.709",
		2:  "unspecified",
		4:  "FCC 73.682",
		5:  "BT.601 625",
		6:  "BT.601 525",
		7:  "SMPTE ST 240",
		8:  "YCgCo",
		9:  "BT.2020 non-constant luminance",
		10: "BT.2020 constant luminance",
		11: "SMPTE ST 2085",
		12: "chromaticity-derived non-constant luminance",
		13: "chromaticity-derived constant luminance",
		14: "BT.2100 ICtCp",
	}
)
