package mp4

import (
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// IspeBox - Image Spatial Extents Property (ispe), ISO/IEC 23008-12:2025 Sec. 6.5.3
//
// Contained in: Item Property Container Box (ipco)
//
// It gives the width and height in pixels of the reconstructed image of the associated image item.
type IspeBox struct {
	Version     byte
	Flags       uint32
	ImageWidth  uint32
	ImageHeight uint32
}

// DecodeIspe - box-specific decode
func DecodeIspe(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	sr := bits.NewFixedSliceReader(data)
	return DecodeIspeSR(hdr, startPos, sr)
}

// DecodeIspeSR - box-specific decode
func DecodeIspeSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	versionAndFlags := sr.ReadUint32()
	b := IspeBox{
		Version:     byte(versionAndFlags >> 24),
		Flags:       versionAndFlags & flagsMask,
		ImageWidth:  sr.ReadUint32(),
		ImageHeight: sr.ReadUint32(),
	}
	return &b, sr.AccError()
}

// Type - box type
func (b *IspeBox) Type() string {
	return "ispe"
}

// Size - calculated size of box
func (b *IspeBox) Size() uint64 {
	return uint64(boxHeaderSize + 4 + 8)
}

// Encode - write box to w
func (b *IspeBox) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - box-specific encode to slicewriter
func (b *IspeBox) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	versionAndFlags := (uint32(b.Version) << 24) + b.Flags
	sw.WriteUint32(versionAndFlags)
	sw.WriteUint32(b.ImageWidth)
	sw.WriteUint32(b.ImageHeight)
	return sw.AccError()
}

// Info - write box-specific information
func (b *IspeBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, int(b.Version), b.Flags)
	bd.write(" - imageWidth: %d", b.ImageWidth)
	bd.write(" - imageHeight: %d", b.ImageHeight)
	return bd.err
}
