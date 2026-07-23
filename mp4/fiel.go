package mp4

import (
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// FielBox - Field/frame information box (fiel)
// Defined in QuickTime File Format as video sample description extension.
// Used in mjpg sample entries produced by Apple mediafilesegmenter.
type FielBox struct {
	FieldCount    byte // 1 = progressive, 2 = interlaced
	FieldOrdering byte
}

// DecodeFiel - box-specific decode
func DecodeFiel(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	sr := bits.NewFixedSliceReader(data)
	return DecodeFielSR(hdr, startPos, sr)
}

// DecodeFielSR - box-specific decode
func DecodeFielSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	b := FielBox{}
	b.FieldCount = sr.ReadUint8()
	b.FieldOrdering = sr.ReadUint8()
	return &b, sr.AccError()
}

// Type - box type
func (b *FielBox) Type() string {
	return "fiel"
}

// Size - calculated size of box
func (b *FielBox) Size() uint64 {
	return uint64(boxHeaderSize + 2)
}

// Encode - write box to w
func (b *FielBox) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - box-specific encode to slicewriter
func (b *FielBox) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	sw.WriteUint8(b.FieldCount)
	sw.WriteUint8(b.FieldOrdering)
	return sw.AccError()
}

// Info - write box-specific information
func (b *FielBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, -1, 0)
	bd.write(" - fieldCount: %d, fieldOrdering: %d", b.FieldCount, b.FieldOrdering)
	return bd.err
}
