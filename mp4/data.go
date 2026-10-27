package mp4

import (
	"encoding/hex"
	"fmt"
	"io"
	"unicode/utf16"

	"github.com/Eyevinn/mp4ff/bits"
)

// Well-known types of the DataBox type indicator.
// See https://developer.apple.com/documentation/quicktime-file-format/well-known_types
const (
	DataTypeImplicit      = 0  // no type indicated, such as for trkn and disk
	DataTypeUTF8          = 1  // UTF-8 text without count or NULL terminator
	DataTypeUTF16         = 2  // UTF-16BE text
	DataTypeUTF8Sort      = 4  // UTF-8 text stored for sorting only
	DataTypeUTF16Sort     = 5  // UTF-16BE text stored for sorting only
	DataTypeJPEG          = 13 // JPEG image in a JFIF wrapper
	DataTypePNG           = 14 // PNG image
	DataTypeBESignedInt   = 21 // big-endian signed integer in 1, 2, 3 or 4 bytes
	DataTypeBEUnsignedInt = 22 // big-endian unsigned integer in 1, 2, 3 or 4 bytes
	DataTypeBMP           = 27 // Windows bitmap image
)

// dataTypeNames are the names of the well-known types of the DataBox type indicator.
var dataTypeNames = map[uint32]string{
	0:  "implicit",
	1:  "UTF-8",
	2:  "UTF-16",
	3:  "S/JIS",
	4:  "UTF-8 sort",
	5:  "UTF-16 sort",
	13: "JPEG",
	14: "PNG",
	21: "BE Signed Integer",
	22: "BE Unsigned Integer",
	23: "BE Float32",
	24: "BE Float64",
	27: "BMP",
	28: "QuickTime Metadata atom",
	65: "8-bit Signed Integer",
	66: "BE 16-bit Signed Integer",
	67: "BE 32-bit Signed Integer",
	70: "BE PointF32",
	71: "BE DimensionsF32",
	72: "BE RectF32",
	74: "BE 64-bit Signed Integer",
	75: "8-bit Unsigned Integer",
	76: "BE 16-bit Unsigned Integer",
	77: "BE 32-bit Unsigned Integer",
	78: "BE 64-bit Unsigned Integer",
	79: "AffineTransformF64",
}

// DataBox - Value atom (data) holding the value of a metadata item, such as a child of ©too in ilst.
// Defined in https://developer.apple.com/documentation/quicktime-file-format/value_atom
//
// DataType is the type indicator, with the well-known type in its lower 24 bits, such as
// DataTypeUTF8 for text or DataTypeImplicit (the zero value) for data without type.
// Country and Language form the locale indicator. For each, 0 means any country or language,
// 1-255 is a 1-based index into the country or language list of the metadata, and other values
// are an ISO 3166 country code as two ASCII bytes, or a packed ISO 639-2/T language code as in
// MdhdBox.Language.
type DataBox struct {
	DataType uint32
	Country  uint16
	Language uint16
	Data     []byte
}

// DecodeData - box-specific decode
func DecodeData(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	sr := bits.NewFixedSliceReader(data)
	return DecodeDataSR(hdr, startPos, sr)
}

// DecodeDataSR - box-specific decode
func DecodeDataSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	payloadLen := hdr.payloadLen()
	if payloadLen < 8 {
		return nil, fmt.Errorf("data payload of %d bytes is shorter than the 8 bytes of type and locale", payloadLen)
	}
	b := &DataBox{
		DataType: sr.ReadUint32(),
		Country:  sr.ReadUint16(),
		Language: sr.ReadUint16(),
	}
	b.Data = sr.ReadBytes(payloadLen - 8)
	return b, sr.AccError()
}

// Type - box type
func (b *DataBox) Type() string {
	return "data"
}

// Size - calculated size of box
func (b *DataBox) Size() uint64 {
	return uint64(boxHeaderSize + 8 + len(b.Data))
}

// Encode - write box to w
func (b *DataBox) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - box-specific encode to slicewriter
func (b *DataBox) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	sw.WriteUint32(b.DataType)
	sw.WriteUint16(b.Country)
	sw.WriteUint16(b.Language)
	sw.WriteBytes(b.Data)
	return sw.AccError()
}

// Info - box-specific Info
func (b *DataBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, -1, 0)
	if name, ok := dataTypeNames[b.DataType]; ok {
		bd.write(" - dataType: %d (%s)", b.DataType, name)
	} else {
		bd.write(" - dataType: %d", b.DataType)
	}
	if b.Country != 0 || b.Language != 0 {
		bd.write(" - locale: country=%s language=%s", localeCountry(b.Country), localeLanguage(b.Language))
	}
	switch b.DataType {
	case DataTypeUTF8, DataTypeUTF8Sort:
		bd.write(" - value: %q", b.Data)
	case DataTypeUTF16, DataTypeUTF16Sort:
		if s, ok := decodeUTF16BE(b.Data); ok {
			bd.write(" - value: %q", s)
		} else {
			bd.write(" - value: %s", hex.EncodeToString(b.Data))
		}
	default:
		bd.write(" - value: %s", hex.EncodeToString(b.Data))
	}
	return bd.err
}

// decodeUTF16BE returns UTF-16BE text as a string, or false if data has an odd number of bytes.
func decodeUTF16BE(data []byte) (string, bool) {
	if len(data)%2 != 0 {
		return "", false
	}
	u := make([]uint16, len(data)/2)
	for i := range u {
		u[i] = uint16(data[2*i])<<8 | uint16(data[2*i+1])
	}
	return string(utf16.Decode(u)), true
}

// localeCountry returns a country indicator of a locale as its ISO 3166 code, or as a number.
func localeCountry(c uint16) string {
	switch {
	case c == 0:
		return "0"
	case c <= 255:
		return fmt.Sprintf("%d (list index)", c)
	}
	hi, lo := byte(c>>8), byte(c)
	if hi >= 'A' && hi <= 'Z' && lo >= 'A' && lo <= 'Z' {
		return string([]byte{hi, lo})
	}
	return fmt.Sprintf("0x%04x", c)
}

// localeLanguage returns a language indicator of a locale as its ISO 639-2/T code, or as a number.
func localeLanguage(l uint16) string {
	switch {
	case l == 0:
		return "0"
	case l <= 255:
		return fmt.Sprintf("%d (list index)", l)
	}
	code := make([]byte, 3)
	for i := range code {
		c := byte(l>>(5*(2-i))&0x1f) + charOffset
		if c < 'a' || c > 'z' {
			return fmt.Sprintf("0x%04x", l)
		}
		code[i] = c
	}
	return string(code)
}
