package mp4

import (
	"encoding/hex"
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// TrgrBox - Track Group Box (trgr), ISO/IEC 14496-12 Sec. 8.3.4. A container in a
// trak holding one TrackGroupTypeBox per track group the track belongs to.
type TrgrBox struct {
	Children []Box
}

// DecodeTrgr - box-specific decode
func DecodeTrgr(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	children, err := DecodeContainerChildren(hdr, startPos+8, startPos+hdr.Size, r)
	if err != nil {
		return nil, err
	}
	b := TrgrBox{Children: make([]Box, 0, len(children))}
	for _, c := range children {
		b.AddChild(c)
	}
	return &b, nil
}

// DecodeTrgrSR - box-specific decode
func DecodeTrgrSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	children, err := DecodeContainerChildrenSR(hdr, startPos+8, startPos+hdr.Size, sr)
	if err != nil {
		return nil, err
	}
	b := TrgrBox{Children: make([]Box, 0, len(children))}
	for _, c := range children {
		b.AddChild(c)
	}
	return &b, nil
}

// AddChild - add a child box
func (b *TrgrBox) AddChild(child Box) {
	b.Children = append(b.Children, child)
}

// Type - box type
func (b *TrgrBox) Type() string {
	return "trgr"
}

// Size - calculated size of box
func (b *TrgrBox) Size() uint64 {
	return containerSize(b.Children)
}

// GetChildren - list of child boxes
func (b *TrgrBox) GetChildren() []Box {
	return b.Children
}

// Encode - write trgr container to w
func (b *TrgrBox) Encode(w io.Writer) error {
	return EncodeContainer(b, w)
}

// EncodeSW - write trgr container to sw
func (b *TrgrBox) EncodeSW(sw bits.SliceWriter) error {
	return EncodeContainerSW(b, sw)
}

// Info - write box info
func (b *TrgrBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	return ContainerInfo(b, w, specificBoxLevels, indent, indentStep)
}

// TrackGroupTypeBox - a track group type box inside trgr, ISO/IEC 14496-12
// Sec. 8.3.4. The box type is the track_group_type: 'msrc', 'ster' (14496-12)
// or 'cstg' (complete subset track grouping for L-HEVC, ISO/IEC 14496-15
// Sec. 9.5.1). Tracks containing a TrackGroupTypeBox of the same type and
// TrackGroupID belong to the same group. Any type-specific data after
// track_group_id (such as the StereoVideoGroupBox 'ster' left_view_flag) is
// preserved in Payload.
type TrackGroupTypeBox struct {
	Version      byte
	Flags        uint32
	TrackGroupID uint32
	Payload      []byte
	boxType      string
}

// CreateTrackGroupTypeBox creates a TrackGroupTypeBox of the given type and group ID.
func CreateTrackGroupTypeBox(boxType string, trackGroupID uint32) *TrackGroupTypeBox {
	return &TrackGroupTypeBox{
		boxType:      boxType,
		TrackGroupID: trackGroupID,
	}
}

// DecodeTrackGroupType - decode a track group type box (msrc, ster, ...)
func DecodeTrackGroupType(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	sr := bits.NewFixedSliceReader(data)
	return DecodeTrackGroupTypeSR(hdr, startPos, sr)
}

// DecodeTrackGroupTypeSR - decode a track group type box
func DecodeTrackGroupTypeSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	versionAndFlags := sr.ReadUint32()
	b := TrackGroupTypeBox{
		boxType:      hdr.Name,
		Version:      byte(versionAndFlags >> 24),
		Flags:        versionAndFlags & flagsMask,
		TrackGroupID: sr.ReadUint32(),
	}
	if n := hdr.payloadLen() - 8; n > 0 {
		b.Payload = sr.ReadBytes(n)
	}
	return &b, sr.AccError()
}

// Type - box type (the track_group_type)
func (b *TrackGroupTypeBox) Type() string {
	return b.boxType
}

// Size - calculated size of box
func (b *TrackGroupTypeBox) Size() uint64 {
	return uint64(boxHeaderSize + 4 + 4 + len(b.Payload)) // header + version/flags + track_group_id + payload
}

// Encode - write box to w
func (b *TrackGroupTypeBox) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	if err := b.EncodeSW(sw); err != nil {
		return err
	}
	_, err := w.Write(sw.Bytes())
	return err
}

// EncodeSW - write box to sw
func (b *TrackGroupTypeBox) EncodeSW(sw bits.SliceWriter) error {
	if err := EncodeHeaderSW(b, sw); err != nil {
		return err
	}
	versionAndFlags := (uint32(b.Version) << 24) | b.Flags
	sw.WriteUint32(versionAndFlags)
	sw.WriteUint32(b.TrackGroupID)
	if len(b.Payload) > 0 {
		sw.WriteBytes(b.Payload)
	}
	return sw.AccError()
}

// Info - box-specific Info
func (b *TrackGroupTypeBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, int(b.Version), b.Flags)
	bd.write(" - trackGroupID: %d", b.TrackGroupID)
	if len(b.Payload) > 0 {
		bd.write(" - payload: %s", hex.EncodeToString(b.Payload))
	}
	return bd.err
}
