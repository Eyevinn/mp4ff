package mp4

import "io"

// MinfBox -  Media Information Box (minf - mandatory)
//
// Contained in : Media Box (mdia)
//
// Status: partially decoded (hmhd - hint tracks - and nmhd - null media - are ignored)
type MinfBox struct {
	Vmhd  *VmhdBox
	Smhd  *SmhdBox
	Stbl  *StblBox
	Dinf  *DinfBox
	Hdlr  *HdlrBox
	boxes []Box
}

// NewMinfBox - Generate a new empty minf box
func NewMinfBox() *MinfBox {
	return &MinfBox{}
}

// AddChild - Add a child box
func (m *MinfBox) AddChild(box Box) {

	switch box.Type() {
	case "vmhd":
		m.Vmhd = box.(*VmhdBox)
	case "smhd":
		m.Smhd = box.(*SmhdBox)
	case "dinf":
		m.Dinf = box.(*DinfBox)
	case "stbl":
		m.Stbl = box.(*StblBox)
	}
	m.boxes = append(m.boxes, box)
}

// DecodeMinf - box-specific decode
func DecodeMinf(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	l, err := DecodeContainerChildren(hdr, startPos+8, r)
	if err != nil {
		return nil, err
	}
	m := NewMinfBox()
	for _, b := range l {
		m.AddChild(b)
	}
	return m, nil
}

// Type - box type
func (m *MinfBox) Type() string {
	return "minf"
}

// Size - calculated size of box
func (m *MinfBox) Size() uint64 {
	return containerSize(m.boxes)
}

// Dump - print box info
func (m *MinfBox) Dump() {
	m.Stbl.Dump()
}

// Encode - write box to w
func (m *MinfBox) Encode(w io.Writer) error {
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
