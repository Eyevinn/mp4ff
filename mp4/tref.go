package mp4

import (
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
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
func DecodeTref(hdr boxHeader, startPos uint64, r io.Reader) (Box, error) {
	children, err := DecodeContainerChildren(hdr, startPos+8, startPos+hdr.size, r)
	if err != nil {
		return nil, err
	}
	b := TrefBox{}
	for _, child := range children {
		b.AddChild(child)
	}
	return &b, nil
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

// Encode - write minf container to w
func (b *TrefBox) Encode(w io.Writer) error {
	return EncodeContainer(b, w)
}

// Info - write box-specific information
func (b *TrefBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	return ContainerInfo(b, w, specificBoxLevels, indent, indentStep)
}

// TrefTypeBox - TrackReferenceTypeBox - ISO/IEC 14496-12 Ed. 9 Sec. 8.3
// Name can be one of hint, cdsc, font, hind, vdep, vplx, subt (ISO/IEC 14496-12)
// dpnd, ipir, mpod, sync (ISO/IEC 14496-14)
type TrefTypeBox struct {
	Name     string
	TrackIDs []uint32
}

// DecodeTrefType - box-specific decode
func DecodeTrefType(hdr boxHeader, startPos uint64, r io.Reader) (Box, error) {
	b := TrefTypeBox{
		Name: hdr.name,
	}
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	for i := 0; i < len(data); i += 4 {
		trackID := binary.BigEndian.Uint32(data[i : i+4])
		b.TrackIDs = append(b.TrackIDs, trackID)
	}
	return &b, nil
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
func (b *TrefTypeBox) Encode(w io.Writer) error {
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	for _, trackID := range b.TrackIDs {
		err = binary.Write(w, binary.BigEndian, trackID)
		if err != nil {
			return err
		}
	}
	return nil
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
