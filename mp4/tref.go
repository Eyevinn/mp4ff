package mp4

import (
	"fmt"
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// TrefBox -  // TrackReferenceBox - ISO/IEC 14496-12 Ed. 9 Sec. 8.3
type TrefBox struct {
	Children []Box
}

// AddChild - Add a child box
func (b *TrefBox) AddChild(box Box) {
	b.Children = append(b.Children, box)
}

// DecodeTref - box-specific decode
func DecodeTref(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	return DecodeTrefSR(hdr, startPos, bits.NewFixedSliceReader(data))
}

// DecodeTrefSR - box-specific decode
//
// Every child is a track reference type box, whatever its reference type, such as tmcd for a QuickTime
// timecode track. A child whose payload is not a list of track IDs is decoded as an UnknownBox.
func DecodeTrefSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	b := TrefBox{}
	end := startPos + hdr.Size
	for pos := startPos + uint64(hdr.Hdrlen); pos < end; {
		chdr, err := DecodeHeaderSR(sr)
		if err != nil {
			return nil, fmt.Errorf("tref: %w", err)
		}
		if chdr.Size > end-pos {
			return nil, fmt.Errorf("tref: child %s of size %d goes beyond the end of tref", chdr.Name, chdr.Size)
		}
		var child Box
		if chdr.payloadLen()%4 == 0 {
			child, err = DecodeTrefTypeSR(chdr, pos, sr)
		} else {
			child, err = DecodeUnknownSR(chdr, pos, sr)
		}
		if err != nil {
			return nil, fmt.Errorf("tref: %w", err)
		}
		b.AddChild(child)
		pos += chdr.Size
	}
	return &b, sr.AccError()
}

// Type - box type
func (b *TrefBox) Type() string {
	return "tref"
}

// Size - calculated size of box
func (b *TrefBox) Size() uint64 {
	return containerSize(b.Children)
}

// GetChildren - list of child boxes
func (b *TrefBox) GetChildren() []Box {
	return b.Children
}

// Encode - write tref container to w
func (b *TrefBox) Encode(w io.Writer) error {
	return EncodeContainer(b, w)
}

// EncodeSW - write tref container to sw
func (b *TrefBox) EncodeSW(sw bits.SliceWriter) error {
	return EncodeContainerSW(b, sw)
}

// Info - write box-specific information
func (b *TrefBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	return ContainerInfo(b, w, specificBoxLevels, indent, indentStep)
}

// TrefTypeBox - TrackReferenceTypeBox - ISO/IEC 14496-12 Ed. 9 Sec. 8.3
// Name is the reference type, such as hint, cdsc, font, hind, vdep, vplx or subt (ISO/IEC 14496-12),
// dpnd, ipir, mpod or sync (ISO/IEC 14496-14), or tmcd or chap (QuickTime)
type TrefTypeBox struct {
	Name     string
	TrackIDs []uint32
}

// DecodeTrefType - box-specific decode
func DecodeTrefType(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	sr := bits.NewFixedSliceReader(data)
	return DecodeTrefTypeSR(hdr, startPos, sr)
}

// DecodeTrefTypeSR - box-specific decode
func DecodeTrefTypeSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	nrIds := hdr.payloadLen() / 4
	b := TrefTypeBox{
		Name:     hdr.Name,
		TrackIDs: make([]uint32, nrIds),
	}
	for i := 0; i < nrIds; i++ {
		b.TrackIDs[i] = sr.ReadUint32()
	}
	return &b, sr.AccError()
}

// Type - box type
func (b *TrefTypeBox) Type() string {
	return b.Name
}

// Size - calculated size of box
func (b *TrefTypeBox) Size() uint64 {
	return uint64(boxHeaderSize + len(b.TrackIDs)*4)
}

// Encode - write box to w
func (t *TrefTypeBox) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(t.Size()))
	err := t.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - write box to sw
func (b *TrefTypeBox) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	for _, trackID := range b.TrackIDs {
		sw.WriteUint32(trackID)
	}
	return sw.AccError()
}

// Info - write box-specific information
func (b *TrefTypeBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, -1, 0)
	msg := " - trackIDs: "
	for _, trackID := range b.TrackIDs {
		msg += fmt.Sprintf(" %d", trackID)
	}
	bd.write(msg)
	return bd.err
}
