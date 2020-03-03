package mp4

import "io"

// MvexBox - MovieExtendsBox (mevx)
//
// Contained in : Movie Box (moov)
//
// Its presence signals a fragmented asset
type MvexBox struct {
	//Mehd *TkhdBox
	Trex  *TrexBox
	boxes []Box
}

// NewMvexBox - Generate a new empty mvex box
func NewMvexBox() *MvexBox {
	return &MvexBox{}
}

// AddChild - Add a child box
func (m *MvexBox) AddChild(box Box) {

	switch box.Type() {
	case "trex":
		m.Trex = box.(*TrexBox)
	}
	m.boxes = append(m.boxes, box)
}

// DecodeMvex - box-specific decode
func DecodeMvex(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	l, err := DecodeContainerChildren(hdr, startPos+8, r)
	if err != nil {
		return nil, err
	}
	m := NewMvexBox()
	for _, b := range l {
		m.AddChild(b)
	}
	return m, nil
}

// Type - return box type
func (m *MvexBox) Type() string {
	return "mvex"
}

// Size - return calculated size
func (m *MvexBox) Size() uint64 {
	return containerSize(m.boxes)
}

// Encode - write box to w
func (m *MvexBox) Encode(w io.Writer) error {
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
