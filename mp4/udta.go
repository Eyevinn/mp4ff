package mp4

import "io"

// User Data Box - container for User Data
//
// Contained in : moov, trak, moof, or traf
//
type UdtaBox struct {
	Children []Box
}

// AddChild - Add a child box
func (b *UdtaBox) AddChild(box Box) {
	b.Children = append(b.Children, box)
}

// DecodeUdta - box-specific decode
func DecodeUdta(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	children, err := DecodeContainerChildren(hdr, startPos+8, startPos+hdr.size, r)
	if err != nil {
		return nil, err
	}
	b := &UdtaBox{}
	for _, child := range children {
		b.AddChild(child)
	}
	return b, nil
}

// Type - box type
func (b *UdtaBox) Type() string {
	return "udta"
}

// Size - calculated size of box
func (b *UdtaBox) Size() uint64 {
	return containerSize(b.Children)
}

// GetChildren - list of child boxes
func (b *UdtaBox) GetChildren() []Box {
	return b.Children
}

// Encode - write udta container to w
func (b *UdtaBox) Encode(w io.Writer) error {
	return EncodeContainer(b, w)
}

func (b *UdtaBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	return ContainerInfo(b, w, specificBoxLevels, indent, indentStep)
}
