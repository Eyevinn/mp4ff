package mp4

import (
	"fmt"
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// TextSampleEntryBox is the 3GPP timed text sample entry tx3g, defined in 3GPP TS 26.245 clause 5.16, as in
// subtitle tracks that ffmpeg writes from mov_text and in the chapter tracks of Apple audio books. ffmpeg also
// writes this layout with the type text, for chapter tracks.
type TextSampleEntryBox struct {
	name               string
	DataReferenceIndex uint16
	// DisplayFlags are flags such as 0x20 scroll in, 0x40 scroll out and 0x20000 vertical text.
	DisplayFlags uint32
	// HorizontalJustification and VerticalJustification are 0 for left or top, 1 for centered and -1 for right
	// or bottom.
	HorizontalJustification int8
	VerticalJustification   int8
	BackgroundColorRGBA     [4]byte
	DefaultTextBox          TextBoxRecord
	DefaultStyle            TextStyleRecord
	// Ftab is the font table, the first box after the fields.
	Ftab *FtabBox
	// Children are the boxes after the fields, starting with Ftab.
	Children []Box
}

// TextBoxRecord is a text box, in pixels relative to the track.
type TextBoxRecord struct {
	Top, Left, Bottom, Right int16
}

// TextStyleRecord is a style record of 3GPP timed text, defined in 3GPP TS 26.245 clause 5.15.
type TextStyleRecord struct {
	// StartChar and EndChar are the range of characters of the style, both 0 in a sample entry.
	StartChar uint16
	EndChar   uint16
	FontID    uint16
	// FaceStyleFlags are 1 for bold, 2 for italic and 4 for underline.
	FaceStyleFlags byte
	FontSize       byte
	TextColorRGBA  [4]byte
}

// textSampleEntryFieldsSize is the size of the fields of a 3GPP timed text sample entry after the box header,
// up to the font table.
const textSampleEntryFieldsSize = 38

// NewTextSampleEntryBox returns a 3GPP timed text sample entry of type boxType, tx3g or text.
func NewTextSampleEntryBox(boxType string) *TextSampleEntryBox {
	return &TextSampleEntryBox{name: boxType}
}

// DecodeTx3g - box-specific decode
func DecodeTx3g(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	return decodeOrUnknown(hdr, startPos, data, decodeTextSampleEntrySR)
}

// DecodeTx3gSR - box-specific decode
func DecodeTx3gSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	return decodeOrUnknownSR(hdr, startPos, sr, decodeTextSampleEntrySR)
}

func decodeTextSampleEntrySR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	if hdr.payloadLen() < textSampleEntryFieldsSize {
		return nil, fmt.Errorf("%s: payload of %d bytes is too short", hdr.Name, hdr.payloadLen())
	}
	sr.SkipBytes(6) // reserved
	b := TextSampleEntryBox{name: hdr.Name, DataReferenceIndex: sr.ReadUint16()}
	b.DisplayFlags = sr.ReadUint32()
	b.HorizontalJustification = int8(sr.ReadUint8())
	b.VerticalJustification = int8(sr.ReadUint8())
	copy(b.BackgroundColorRGBA[:], sr.ReadBytes(4))
	b.DefaultTextBox = readTextBoxRecord(sr)
	b.DefaultStyle = readTextStyleRecord(sr)
	start := startPos + uint64(hdr.Hdrlen) + textSampleEntryFieldsSize
	children, err := DecodeContainerChildrenSR(hdr, start, startPos+hdr.Size, sr)
	if err != nil {
		return nil, err
	}
	for _, c := range children {
		b.AddChild(c)
	}
	return &b, sr.AccError()
}

func readTextBoxRecord(sr bits.SliceReader) TextBoxRecord {
	var r TextBoxRecord
	r.Top = sr.ReadInt16()
	r.Left = sr.ReadInt16()
	r.Bottom = sr.ReadInt16()
	r.Right = sr.ReadInt16()
	return r
}

func writeTextBoxRecord(sw bits.SliceWriter, r TextBoxRecord) {
	sw.WriteInt16(r.Top)
	sw.WriteInt16(r.Left)
	sw.WriteInt16(r.Bottom)
	sw.WriteInt16(r.Right)
}

func readTextStyleRecord(sr bits.SliceReader) TextStyleRecord {
	var s TextStyleRecord
	s.StartChar = sr.ReadUint16()
	s.EndChar = sr.ReadUint16()
	s.FontID = sr.ReadUint16()
	s.FaceStyleFlags = sr.ReadUint8()
	s.FontSize = sr.ReadUint8()
	copy(s.TextColorRGBA[:], sr.ReadBytes(4))
	return s
}

func writeTextStyleRecord(sw bits.SliceWriter, s TextStyleRecord) {
	sw.WriteUint16(s.StartChar)
	sw.WriteUint16(s.EndChar)
	sw.WriteUint16(s.FontID)
	sw.WriteUint8(s.FaceStyleFlags)
	sw.WriteUint8(s.FontSize)
	sw.WriteBytes(s.TextColorRGBA[:])
}

// AddChild adds a child box, and sets Ftab if it is the first font table.
func (b *TextSampleEntryBox) AddChild(child Box) {
	if ftab, ok := child.(*FtabBox); ok && b.Ftab == nil {
		b.Ftab = ftab
	}
	b.Children = append(b.Children, child)
}

// Type - box type, tx3g unless set to text
func (b *TextSampleEntryBox) Type() string {
	if b.name == "" {
		return "tx3g"
	}
	return b.name
}

// Size - calculated size of box
func (b *TextSampleEntryBox) Size() uint64 {
	size := uint64(boxHeaderSize + textSampleEntryFieldsSize)
	for _, c := range b.Children {
		size += c.Size()
	}
	return size
}

// GetChildren - list of child boxes
func (b *TextSampleEntryBox) GetChildren() []Box {
	return b.Children
}

// Encode - write box to w
func (b *TextSampleEntryBox) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - box-specific encode to slicewriter
func (b *TextSampleEntryBox) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	sw.WriteZeroBytes(6)
	sw.WriteUint16(b.DataReferenceIndex)
	sw.WriteUint32(b.DisplayFlags)
	sw.WriteUint8(byte(b.HorizontalJustification))
	sw.WriteUint8(byte(b.VerticalJustification))
	sw.WriteBytes(b.BackgroundColorRGBA[:])
	writeTextBoxRecord(sw, b.DefaultTextBox)
	writeTextStyleRecord(sw, b.DefaultStyle)
	for _, c := range b.Children {
		if err := c.EncodeSW(sw); err != nil {
			return err
		}
	}
	return sw.AccError()
}

// Info - write box-specific information
func (b *TextSampleEntryBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, -1, 0)
	bd.write(" - dataReferenceIndex: %d", b.DataReferenceIndex)
	bd.write(" - displayFlags: %#x", b.DisplayFlags)
	bd.write(" - justification: horizontal=%d vertical=%d", b.HorizontalJustification, b.VerticalJustification)
	bd.write(" - backgroundColorRGBA: %x", b.BackgroundColorRGBA)
	r := b.DefaultTextBox
	bd.write(" - defaultTextBox: top=%d left=%d bottom=%d right=%d", r.Top, r.Left, r.Bottom, r.Right)
	s := b.DefaultStyle
	bd.write(" - defaultStyle: startChar=%d endChar=%d fontID=%d faceStyleFlags=%#x fontSize=%d textColorRGBA=%x",
		s.StartChar, s.EndChar, s.FontID, s.FaceStyleFlags, s.FontSize, s.TextColorRGBA)
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

// FtabBox is the font table box ftab of a 3GPP timed text sample entry, defined in 3GPP TS 26.245 clause 5.16.
type FtabBox struct {
	Fonts []FontRecord
}

// FontRecord is a font in a font table.
type FontRecord struct {
	ID uint16
	// Name is a comma-separated list of font names in order of preference, in UTF-8, or in UTF-16 after a byte
	// order mark. The names Serif, Sans-serif and Monospace are generic.
	Name string
}

// DecodeFtab - box-specific decode
func DecodeFtab(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	return decodeOrUnknown(hdr, startPos, data, decodeFtabSR)
}

// DecodeFtabSR - box-specific decode
func DecodeFtabSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	return decodeOrUnknownSR(hdr, startPos, sr, decodeFtabSR)
}

func decodeFtabSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	if hdr.payloadLen() < 2 {
		return nil, fmt.Errorf("ftab: payload of %d bytes is too short", hdr.payloadLen())
	}
	end := sr.GetPos() + hdr.payloadLen()
	count := int(sr.ReadUint16())
	b := FtabBox{}
	for i := 0; i < count; i++ {
		if end-sr.GetPos() < 3 {
			return nil, fmt.Errorf("ftab: font %d of %d does not fit", i+1, count)
		}
		id := sr.ReadUint16()
		n := int(sr.ReadUint8())
		if n > end-sr.GetPos() {
			return nil, fmt.Errorf("ftab: name of font %d does not fit", i+1)
		}
		b.Fonts = append(b.Fonts, FontRecord{ID: id, Name: sr.ReadFixedLengthString(n)})
	}
	return &b, sr.AccError()
}

// Type - box type
func (b *FtabBox) Type() string {
	return "ftab"
}

// Size - calculated size of box
func (b *FtabBox) Size() uint64 {
	size := uint64(boxHeaderSize + 2)
	for _, f := range b.Fonts {
		size += 3 + uint64(len(f.Name))
	}
	return size
}

// Encode - write box to w
func (b *FtabBox) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - box-specific encode to slicewriter
func (b *FtabBox) EncodeSW(sw bits.SliceWriter) error {
	if len(b.Fonts) > 0xffff {
		return fmt.Errorf("ftab: %d fonts are more than 65535", len(b.Fonts))
	}
	for _, f := range b.Fonts {
		if len(f.Name) > 255 {
			return fmt.Errorf("ftab: font name of %d bytes is longer than 255", len(f.Name))
		}
	}
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	sw.WriteUint16(uint16(len(b.Fonts)))
	for _, f := range b.Fonts {
		sw.WriteUint16(f.ID)
		sw.WriteUint8(byte(len(f.Name)))
		sw.WriteString(f.Name, false)
	}
	return sw.AccError()
}

// Info - write box-specific information
func (b *FtabBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, -1, 0)
	for _, f := range b.Fonts {
		bd.write(" - font: id=%d name=%q", f.ID, f.Name)
	}
	return bd.err
}
