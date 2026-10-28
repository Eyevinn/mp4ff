package mp4

import (
	"fmt"
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// DecodeTapt decodes the QuickTime track aperture mode dimensions atom tapt, a container of the clef, prof and
// enof atoms, as a GenericContainerBox.
func DecodeTapt(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	return decodeOrUnknown(hdr, startPos, data, DecodeGenericContainerBoxSR)
}

// DecodeTaptSR decodes the QuickTime track aperture mode dimensions atom tapt as a GenericContainerBox.
func DecodeTaptSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	return decodeOrUnknownSR(hdr, startPos, sr, DecodeGenericContainerBoxSR)
}

// TrackApertureDimensionsBox is a QuickTime track aperture dimensions atom in tapt: clef for the clean aperture,
// prof for the production aperture, or enof for the encoded pixels.
type TrackApertureDimensionsBox struct {
	name    string
	Version byte
	Flags   uint32
	Width   Fixed32
	Height  Fixed32
}

// NewTrackApertureDimensionsBox returns a track aperture dimensions atom of type boxType, clef, prof or enof.
func NewTrackApertureDimensionsBox(boxType string, width, height Fixed32) *TrackApertureDimensionsBox {
	return &TrackApertureDimensionsBox{name: boxType, Width: width, Height: height}
}

// DecodeTrackApertureDimensions - box-specific decode
func DecodeTrackApertureDimensions(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	return decodeOrUnknown(hdr, startPos, data, decodeTrackApertureDimensionsSR)
}

// DecodeTrackApertureDimensionsSR - box-specific decode
func DecodeTrackApertureDimensionsSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	return decodeOrUnknownSR(hdr, startPos, sr, decodeTrackApertureDimensionsSR)
}

func decodeTrackApertureDimensionsSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	if hdr.payloadLen() != 12 {
		return nil, fmt.Errorf("%s: payload of %d bytes, not 12", hdr.Name, hdr.payloadLen())
	}
	versionAndFlags := sr.ReadUint32()
	b := TrackApertureDimensionsBox{name: hdr.Name, Version: byte(versionAndFlags >> 24),
		Flags: versionAndFlags & flagsMask}
	b.Width = Fixed32(sr.ReadUint32())
	b.Height = Fixed32(sr.ReadUint32())
	return &b, sr.AccError()
}

// Type - box type
func (b *TrackApertureDimensionsBox) Type() string {
	return b.name
}

// Size - calculated size of box
func (b *TrackApertureDimensionsBox) Size() uint64 {
	return boxHeaderSize + 12
}

// Encode - write box to w
func (b *TrackApertureDimensionsBox) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - box-specific encode to slicewriter
func (b *TrackApertureDimensionsBox) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	sw.WriteUint32(uint32(b.Version)<<24 | b.Flags)
	sw.WriteUint32(uint32(b.Width))
	sw.WriteUint32(uint32(b.Height))
	return sw.AccError()
}

// Info - write box-specific information
func (b *TrackApertureDimensionsBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, int(b.Version), b.Flags)
	bd.write(" - width: %s", b.Width)
	bd.write(" - height: %s", b.Height)
	return bd.err
}
