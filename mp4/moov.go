package mp4

import (
	"fmt"
	"io"
)

// MoovBox - Movie Box (moov - mandatory)
//
// Contains all meta-data. To be able to stream a file, the moov box should be placed before the mdat box.
type MoovBox struct {
	Mvhd  *MvhdBox
	Trak  []*TrakBox
	Mvex  *MvexBox
	boxes []Box
}

// NewMoovBox - Generate a new empty moov box
func NewMoovBox() *MoovBox {
	return &MoovBox{}
}

// AddChild - Add a child box
func (m *MoovBox) AddChild(box Box) {

	switch box.Type() {
	case "mvhd":
		m.Mvhd = box.(*MvhdBox)
	case "trak":
		m.Trak = append(m.Trak, box.(*TrakBox))
	case "mvex":
		m.Mvex = box.(*MvexBox)
	}
	m.boxes = append(m.boxes, box)
}

// DecodeMoov - box-specific decode
func DecodeMoov(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	l, err := DecodeContainerChildren(hdr, startPos+8, startPos+hdr.size, r)
	if err != nil {
		return nil, err
	}
	m := NewMoovBox()
	for _, b := range l {
		m.AddChild(b)
	}
	return m, err
}

// Type - box type
func (m *MoovBox) Type() string {
	return "moov"
}

// Size - calculated size of box
func (m *MoovBox) Size() uint64 {
	return containerSize(m.boxes)
}

// Dump - print box info
func (m *MoovBox) Dump() {
	m.Mvhd.Dump()
	for i, t := range m.Trak {
		fmt.Println("Track", i)
		t.Dump()
	}
}

// Encode - write box to w
func (m *MoovBox) Encode(w io.Writer) error {
	err := EncodeHeader(m, w)
	if err != nil {
		return err
	}
	for _, b := range m.boxes {
		err = b.Encode(w)
		if err != nil {
			return err
		}
	}
	return nil
}
