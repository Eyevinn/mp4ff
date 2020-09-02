package mp4

import (
	"io"
)

// MoovBox - Movie Box (moov - mandatory)
//
// Contains all meta-data. To be able to stream a file, the moov box should be placed before the mdat box.
type MoovBox struct {
	Mvhd     *MvhdBox
	Trak     []*TrakBox
	Mvex     *MvexBox
	Children []Box
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
	m.Children = append(m.Children, box)
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
	return containerSize(m.Children)
}

// GetChildren - list of child boxes
func (m *MoovBox) GetChildren() []Box {
	return m.Children
}

// Encode - write moov container to w
func (m *MoovBox) Encode(w io.Writer) error {
	return EncodeContainer(m, w)
}

func (m *MoovBox) Dump(w io.Writer, indent, indentStep string) error {
	return DumpContainer(m, w, indent, indentStep)
}
