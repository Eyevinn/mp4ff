package mp4

import (
	"fmt"
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// ClliBox - Content Light Level Box (clli), ISO/IEC 14496-12 Sec. 12.1.6.
type ClliBox struct {
	MaxContentLightLevel    uint16
	MaxPicAverageLightLevel uint16
}

// CreateClliBox creates a new ClliBox with specified values.
func CreateClliBox(maxContentLightLevel, maxPicAverageLightLevel uint16) *ClliBox {
	return &ClliBox{
		MaxContentLightLevel:    maxContentLightLevel,
		MaxPicAverageLightLevel: maxPicAverageLightLevel,
	}
}

const clliBoxSize = boxHeaderSize + 2*2 // Header + 2 uint16s

// DecodeClli - box-specific decode.
func DecodeClli(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	if hdr.Hdrlen != boxHeaderSize || hdr.Size != clliBoxSize {
		return nil, fmt.Errorf("invalid box size %d", hdr.Size)
	}
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	sr := bits.NewFixedSliceReader(data)
	return DecodeClliSR(hdr, startPos, sr)
}

// DecodeClliSR - box-specific decode.
func DecodeClliSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	if hdr.Hdrlen != boxHeaderSize || hdr.Size != clliBoxSize {
		return nil, fmt.Errorf("invalid box size %d", hdr.Size)
	}
	b := &ClliBox{}
	b.MaxContentLightLevel = sr.ReadUint16()
	b.MaxPicAverageLightLevel = sr.ReadUint16()
	return b, sr.AccError()
}

// Type - box type.
func (b *ClliBox) Type() string {
	return "clli"
}

// Size - calculated size of box.
func (b *ClliBox) Size() uint64 {
	return clliBoxSize
}

// Encode - write box to w.
func (b *ClliBox) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - box-specific encode to slicewriter.
func (b *ClliBox) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	sw.WriteUint16(b.MaxContentLightLevel)
	sw.WriteUint16(b.MaxPicAverageLightLevel)
	return sw.AccError()
}

// Info - write box-specific information.
func (b *ClliBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, -1, 0)
	bd.write(" - maxContentLightLevel: %d", b.MaxContentLightLevel)
	bd.write(" - maxPicAverageLightLevel: %d", b.MaxPicAverageLightLevel)
	return bd.err
}
