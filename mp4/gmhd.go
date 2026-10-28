package mp4

import (
	"fmt"
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// DecodeGmhd decodes the QuickTime base media information header atom gmhd, in the minf of tracks such as
// timecode and text tracks, as a GenericContainerBox. It holds a gmin atom, and for a timecode track, a tmcd
// atom with a tcmi atom.
func DecodeGmhd(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	return decodeOrUnknown(hdr, startPos, data, DecodeGenericContainerBoxSR)
}

// DecodeGmhdSR decodes the QuickTime base media information header atom gmhd as a GenericContainerBox.
func DecodeGmhdSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	return decodeOrUnknownSR(hdr, startPos, sr, DecodeGenericContainerBoxSR)
}

// GminBox is the QuickTime base media info atom gmin, in gmhd.
type GminBox struct {
	Version byte
	Flags   uint32
	// GraphicsMode is the QuickDraw transfer mode, such as 0x40 for dither copy.
	GraphicsMode uint16
	// Opcolor is the red, green and blue color for the transfer mode.
	Opcolor  [3]uint16
	Balance  int16
	Reserved uint16
}

// DecodeGmin - box-specific decode
func DecodeGmin(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	return decodeOrUnknown(hdr, startPos, data, decodeGminSR)
}

// DecodeGminSR - box-specific decode
func DecodeGminSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	return decodeOrUnknownSR(hdr, startPos, sr, decodeGminSR)
}

func decodeGminSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	if hdr.payloadLen() != 16 {
		return nil, fmt.Errorf("gmin: payload of %d bytes, not 16", hdr.payloadLen())
	}
	versionAndFlags := sr.ReadUint32()
	b := GminBox{Version: byte(versionAndFlags >> 24), Flags: versionAndFlags & flagsMask}
	b.GraphicsMode = sr.ReadUint16()
	for i := range b.Opcolor {
		b.Opcolor[i] = sr.ReadUint16()
	}
	b.Balance = sr.ReadInt16()
	b.Reserved = sr.ReadUint16()
	return &b, sr.AccError()
}

// Type - box type
func (b *GminBox) Type() string {
	return "gmin"
}

// Size - calculated size of box
func (b *GminBox) Size() uint64 {
	return boxHeaderSize + 16
}

// Encode - write box to w
func (b *GminBox) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - box-specific encode to slicewriter
func (b *GminBox) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	sw.WriteUint32(uint32(b.Version)<<24 | b.Flags)
	sw.WriteUint16(b.GraphicsMode)
	for _, c := range b.Opcolor {
		sw.WriteUint16(c)
	}
	sw.WriteInt16(b.Balance)
	sw.WriteUint16(b.Reserved)
	return sw.AccError()
}

// Info - write box-specific information
func (b *GminBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, int(b.Version), b.Flags)
	bd.write(" - graphicsMode: %#x", b.GraphicsMode)
	bd.write(" - opcolor: %d %d %d", b.Opcolor[0], b.Opcolor[1], b.Opcolor[2])
	bd.write(" - balance: %d", b.Balance)
	return bd.err
}

// TcmiBox is the QuickTime timecode media information atom tcmi, in the tmcd atom of gmhd. It tells how to show
// the timecode as text.
type TcmiBox struct {
	Version byte
	Flags   uint32
	// TextFont is the font number, or 0 for the system font. FontName overrides it.
	TextFont uint16
	// TextFace is the style: bit 0 bold, 1 italic, 2 underline, 3 outline, 4 shadow, 5 condense and 6 extend.
	TextFace uint16
	TextSize uint16
	// Reserved is a field that QuickTime and ffmpeg write after the text size, but that the specification lacks.
	Reserved        uint16
	TextColor       [3]uint16
	BackgroundColor [3]uint16
	FontName        string
}

// DecodeTcmi - box-specific decode
func DecodeTcmi(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	return decodeOrUnknown(hdr, startPos, data, decodeTcmiSR)
}

// DecodeTcmiSR - box-specific decode
func DecodeTcmiSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	return decodeOrUnknownSR(hdr, startPos, sr, decodeTcmiSR)
}

func decodeTcmiSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	if hdr.payloadLen() < 25 {
		return nil, fmt.Errorf("tcmi: payload of %d bytes is too short", hdr.payloadLen())
	}
	end := sr.GetPos() + hdr.payloadLen()
	versionAndFlags := sr.ReadUint32()
	b := TcmiBox{Version: byte(versionAndFlags >> 24), Flags: versionAndFlags & flagsMask}
	b.TextFont = sr.ReadUint16()
	b.TextFace = sr.ReadUint16()
	b.TextSize = sr.ReadUint16()
	b.Reserved = sr.ReadUint16()
	for i := range b.TextColor {
		b.TextColor[i] = sr.ReadUint16()
	}
	for i := range b.BackgroundColor {
		b.BackgroundColor[i] = sr.ReadUint16()
	}
	n := int(sr.ReadUint8())
	if left := end - sr.GetPos(); n != left {
		return nil, fmt.Errorf("tcmi: font name of %d bytes, but %d bytes left", n, left)
	}
	b.FontName = sr.ReadFixedLengthString(n)
	return &b, sr.AccError()
}

// Type - box type
func (b *TcmiBox) Type() string {
	return "tcmi"
}

// Size - calculated size of box
func (b *TcmiBox) Size() uint64 {
	return uint64(boxHeaderSize + 25 + len(b.FontName))
}

// Encode - write box to w
func (b *TcmiBox) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - box-specific encode to slicewriter
func (b *TcmiBox) EncodeSW(sw bits.SliceWriter) error {
	if len(b.FontName) > 255 {
		return fmt.Errorf("tcmi: font name of %d bytes is longer than 255", len(b.FontName))
	}
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	sw.WriteUint32(uint32(b.Version)<<24 | b.Flags)
	sw.WriteUint16(b.TextFont)
	sw.WriteUint16(b.TextFace)
	sw.WriteUint16(b.TextSize)
	sw.WriteUint16(b.Reserved)
	for _, c := range b.TextColor {
		sw.WriteUint16(c)
	}
	for _, c := range b.BackgroundColor {
		sw.WriteUint16(c)
	}
	sw.WriteUint8(byte(len(b.FontName)))
	sw.WriteString(b.FontName, false)
	return sw.AccError()
}

// Info - write box-specific information
func (b *TcmiBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, int(b.Version), b.Flags)
	bd.write(" - textFont: %d", b.TextFont)
	bd.write(" - textFace: %#x", b.TextFace)
	bd.write(" - textSize: %d", b.TextSize)
	bd.write(" - textColor: %d %d %d", b.TextColor[0], b.TextColor[1], b.TextColor[2])
	bd.write(" - backgroundColor: %d %d %d", b.BackgroundColor[0], b.BackgroundColor[1], b.BackgroundColor[2])
	bd.write(" - fontName: %q", b.FontName)
	return bd.err
}
