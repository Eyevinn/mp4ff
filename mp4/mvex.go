package mp4

import "io"

// MvexBox - MovieExtendsBox (mevx)
//
// Contained in : Movie Box (moov)
//
// Its presence signals a fragmented asset
type MvexBox struct {
	Mehd     *MehdBox
	Trex     *TrexBox
	Trexs    []*TrexBox
	Children []Box
}

// NewMvexBox - Generate a new empty mvex box
func NewMvexBox() *MvexBox {
	return &MvexBox{}
}

// AddChild - Add a child box
func (m *MvexBox) AddChild(box Box) {

	switch box.Type() {
	case "mehd":
		m.Mehd = box.(*MehdBox)
	case "trex":
		if m.Trex == nil {
			m.Trex = box.(*TrexBox)
		}
		m.Trexs = append(m.Trexs, box.(*TrexBox))
	}
	m.Children = append(m.Children, box)
}

// DecodeMvex - box-specific decode
func DecodeMvex(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	l, err := DecodeContainerChildren(hdr, startPos+8, startPos+hdr.size, r)
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
	return containerSize(m.Children)
}

// GetChildren - list of child boxes
func (t *MvexBox) GetChildren() []Box {
	return t.Children
}

// Encode - write mvex container to w
func (m *MvexBox) Encode(w io.Writer) error {
	return EncodeContainer(m, w)
}

func (m *MvexBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	return ContainerInfo(m, w, specificBoxLevels, indent, indentStep)
}
