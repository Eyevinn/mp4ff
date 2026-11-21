package mp4

import (
	"fmt"
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// IrotBox - Image Rotation Property (irot), ISO/IEC 23008-12:2025 Sec. 6.5.10
//
// Contained in: Item Property Container Box (ipco)
//
// Angle is 0 to 3, and tells that the reconstructed image of the associated image item is rotated
// Angle times 90 degrees anti-clockwise.
type IrotBox struct {
	Angle byte
}

// DecodeIrot - box-specific decode
func DecodeIrot(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	sr := bits.NewFixedSliceReader(data)
	return DecodeIrotSR(hdr, startPos, sr)
}

// DecodeIrotSR - box-specific decode
func DecodeIrotSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	b := IrotBox{Angle: sr.ReadUint8() & 0x03}
	return &b, sr.AccError()
}

// Type - box type
func (b *IrotBox) Type() string {
	return "irot"
}

// Size - calculated size of box
func (b *IrotBox) Size() uint64 {
	return uint64(boxHeaderSize + 1)
}

// Encode - write box to w
func (b *IrotBox) Encode(w io.Writer) error {
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
// It returns an error for an angle above 3.
func (b *IrotBox) EncodeSW(sw bits.SliceWriter) error {
	if b.Angle > 3 {
		return fmt.Errorf("irot: angle %d is more than 3", b.Angle)
	}
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	sw.WriteUint8(b.Angle)
	return sw.AccError()
}

// Info - write box-specific information
func (b *IrotBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, -1, 0)
	bd.write(" - angle: %d (%d degrees anti-clockwise)", b.Angle, int(b.Angle)*90)
	return bd.err
}
