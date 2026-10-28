package mp4

import (
	"fmt"
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// QuickTimeTextSampleEntryBox is the QuickTime text sample entry text, with the layout of the text description
// of QuickTime.
//
// The fields from StyleStart to ForegroundColor are the default text style, laid out as in the text
// description of the QuickTime API. The QuickTime File Format specification calls the first eight bytes of them
// reserved, and lists fields that are one byte longer in all.
type QuickTimeTextSampleEntryBox struct {
	DataReferenceIndex uint16
	// DisplayFlags are flags such as 0x2 don't auto scale, 0x1000 drop shadow and 0x2000 anti-alias.
	DisplayFlags uint32
	// TextJustification is 0 for left, 1 for centered and -1 for right.
	TextJustification int32
	BackgroundColor   [3]uint16
	DefaultTextBox    TextBoxRecord
	// StyleStart is the first character of the default style, 0.
	StyleStart uint32
	LineHeight int16
	Ascent     int16
	FontNumber uint16
	// FontFace is the style: bit 0 bold, 1 italic, 2 underline, 3 outline, 4 shadow, 5 condense and 6 extend.
	FontFace byte
	// Reserved is the byte after FontFace.
	Reserved        byte
	FontSize        uint16
	ForegroundColor [3]uint16
	FontName        string
	// Children are the atoms after the font name.
	Children []Box
}

// quickTimeTextEntryFieldsSize is the size of the fields of a QuickTime text sample entry after the box header,
// up to the font name.
const quickTimeTextEntryFieldsSize = 50

// DecodeText decodes a text atom: as a TextSampleEntryBox if it has the layout of a 3GPP timed text sample
// entry, with a font table box after the fields, and otherwise as a QuickTimeTextSampleEntryBox. A payload in
// neither layout, such as that of the undocumented text atom in the gmhd of a text track, decodes as an
// UnknownBox.
func DecodeText(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	return DecodeTextSR(hdr, startPos, bits.NewFixedSliceReader(data))
}

// DecodeTextSR decodes a text atom, as DecodeText.
func DecodeTextSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	fourCC := make([]byte, 4)
	if sr.LookAhead(textSampleEntryFieldsSize+4, fourCC) == nil && string(fourCC) == "ftab" {
		return decodeOrUnknownSR(hdr, startPos, sr, decodeTextSampleEntrySR)
	}
	return decodeOrUnknownSR(hdr, startPos, sr, decodeQuickTimeTextSampleEntrySR)
}

func decodeQuickTimeTextSampleEntrySR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	if hdr.payloadLen() < quickTimeTextEntryFieldsSize+1 {
		return nil, fmt.Errorf("text: payload of %d bytes is too short", hdr.payloadLen())
	}
	end := sr.GetPos() + hdr.payloadLen()
	sr.SkipBytes(6) // reserved
	b := QuickTimeTextSampleEntryBox{DataReferenceIndex: sr.ReadUint16()}
	b.DisplayFlags = sr.ReadUint32()
	b.TextJustification = sr.ReadInt32()
	for i := range b.BackgroundColor {
		b.BackgroundColor[i] = sr.ReadUint16()
	}
	b.DefaultTextBox = readTextBoxRecord(sr)
	b.StyleStart = sr.ReadUint32()
	b.LineHeight = sr.ReadInt16()
	b.Ascent = sr.ReadInt16()
	b.FontNumber = sr.ReadUint16()
	b.FontFace = sr.ReadUint8()
	b.Reserved = sr.ReadUint8()
	b.FontSize = sr.ReadUint16()
	for i := range b.ForegroundColor {
		b.ForegroundColor[i] = sr.ReadUint16()
	}
	n := int(sr.ReadUint8())
	if n > end-sr.GetPos() {
		return nil, fmt.Errorf("text: font name of %d bytes does not fit", n)
	}
	b.FontName = sr.ReadFixedLengthString(n)
	start := startPos + uint64(hdr.Hdrlen) + quickTimeTextEntryFieldsSize + 1 + uint64(n)
	children, err := DecodeContainerChildrenSR(hdr, start, startPos+hdr.Size, sr)
	if err != nil {
		return nil, err
	}
	if len(children) > 0 {
		b.Children = children
	}
	return &b, sr.AccError()
}

// Type - box type
func (b *QuickTimeTextSampleEntryBox) Type() string {
	return "text"
}

// Size - calculated size of box
func (b *QuickTimeTextSampleEntryBox) Size() uint64 {
	size := uint64(boxHeaderSize+quickTimeTextEntryFieldsSize+1) + uint64(len(b.FontName))
	for _, c := range b.Children {
		size += c.Size()
	}
	return size
}

// GetChildren - list of child boxes
func (b *QuickTimeTextSampleEntryBox) GetChildren() []Box {
	return b.Children
}

// Encode - write box to w
func (b *QuickTimeTextSampleEntryBox) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - box-specific encode to slicewriter
func (b *QuickTimeTextSampleEntryBox) EncodeSW(sw bits.SliceWriter) error {
	if len(b.FontName) > 255 {
		return fmt.Errorf("text: font name of %d bytes is longer than 255", len(b.FontName))
	}
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	sw.WriteZeroBytes(6)
	sw.WriteUint16(b.DataReferenceIndex)
	sw.WriteUint32(b.DisplayFlags)
	sw.WriteInt32(b.TextJustification)
	for _, c := range b.BackgroundColor {
		sw.WriteUint16(c)
	}
	writeTextBoxRecord(sw, b.DefaultTextBox)
	sw.WriteUint32(b.StyleStart)
	sw.WriteInt16(b.LineHeight)
	sw.WriteInt16(b.Ascent)
	sw.WriteUint16(b.FontNumber)
	sw.WriteUint8(b.FontFace)
	sw.WriteUint8(b.Reserved)
	sw.WriteUint16(b.FontSize)
	for _, c := range b.ForegroundColor {
		sw.WriteUint16(c)
	}
	sw.WriteUint8(byte(len(b.FontName)))
	sw.WriteString(b.FontName, false)
	for _, c := range b.Children {
		if err := c.EncodeSW(sw); err != nil {
			return err
		}
	}
	return sw.AccError()
}

// Info - write box-specific information
func (b *QuickTimeTextSampleEntryBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, -1, 0)
	bd.write(" - dataReferenceIndex: %d", b.DataReferenceIndex)
	bd.write(" - displayFlags: %#x", b.DisplayFlags)
	bd.write(" - textJustification: %d", b.TextJustification)
	bd.write(" - backgroundColor: %d %d %d", b.BackgroundColor[0], b.BackgroundColor[1], b.BackgroundColor[2])
	r := b.DefaultTextBox
	bd.write(" - defaultTextBox: top=%d left=%d bottom=%d right=%d", r.Top, r.Left, r.Bottom, r.Right)
	bd.write(" - lineHeight: %d", b.LineHeight)
	bd.write(" - ascent: %d", b.Ascent)
	bd.write(" - fontNumber: %d", b.FontNumber)
	bd.write(" - fontFace: %#x", b.FontFace)
	bd.write(" - fontSize: %d", b.FontSize)
	bd.write(" - foregroundColor: %d %d %d", b.ForegroundColor[0], b.ForegroundColor[1], b.ForegroundColor[2])
	bd.write(" - fontName: %q", b.FontName)
	if bd.err != nil {
		return bd.err
	}
	for _, c := range b.Children {
		if err := c.Info(w, specificBoxLevels, indent+indentStep, indentStep); err != nil {
			return err
		}
	}
	return nil
}
