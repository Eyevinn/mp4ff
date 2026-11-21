package mp4

import (
	"fmt"
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// Axis values of ImirBox.
const (
	ImirAxisVertical   = 0 // the top and bottom parts of the image are exchanged
	ImirAxisHorizontal = 1 // the left and right parts of the image are exchanged
)

// ImirBox - Image Mirroring Property (imir), ISO/IEC 23008-12:2025 Sec. 6.5.12
//
// Contained in: Item Property Container Box (ipco)
//
// Axis tells how the image of the associated image item is mirrored: ImirAxisVertical or
// ImirAxisHorizontal.
type ImirBox struct {
	Axis byte
}

// DecodeImir - box-specific decode
func DecodeImir(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	sr := bits.NewFixedSliceReader(data)
	return DecodeImirSR(hdr, startPos, sr)
}

// DecodeImirSR - box-specific decode
func DecodeImirSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	b := ImirBox{Axis: sr.ReadUint8() & 0x01}
	return &b, sr.AccError()
}

// Type - box type
func (b *ImirBox) Type() string {
	return "imir"
}

// Size - calculated size of box
func (b *ImirBox) Size() uint64 {
	return uint64(boxHeaderSize + 1)
}

// Encode - write box to w
func (b *ImirBox) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - box-specific encode to slicewriter
//
// It returns an error for an axis above 1.
func (b *ImirBox) EncodeSW(sw bits.SliceWriter) error {
	if b.Axis > 1 {
		return fmt.Errorf("imir: axis %d is more than 1", b.Axis)
	}
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	sw.WriteUint8(b.Axis)
	return sw.AccError()
}

// Info - write box-specific information
func (b *ImirBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, -1, 0)
	desc := "vertical, top and bottom exchanged"
	if b.Axis == ImirAxisHorizontal {
		desc = "horizontal, left and right exchanged"
	}
	bd.write(" - axis: %d (%s)", b.Axis, desc)
	return bd.err
}
