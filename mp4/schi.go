package mp4

import "io"

// SchiBox -  Schema Information Box
type SchiBox struct {
	Children []Box
}

// AddChild - Add a child box
func (b *SchiBox) AddChild(box Box) {
	b.Children = append(b.Children, box)
}

// DecodeSchi - box-specific decode
func DecodeSchi(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	children, err := DecodeContainerChildren(hdr, startPos+8, startPos+hdr.size, r)
	if err != nil {
		return nil, err
	}
	b := &SchiBox{}
	for _, child := range children {
		b.AddChild(child)
	}
	return b, nil
}

// Type - box type
func (b *SchiBox) Type() string {
	return "schi"
}

// Size - calculated size of box
func (b *SchiBox) Size() uint64 {
	return containerSize(b.Children)
}

// GetChildren - list of child boxes
func (b *SchiBox) GetChildren() []Box {
	return b.Children
}

// Encode - write minf container to w
func (b *SchiBox) Encode(w io.Writer) error {
	return EncodeContainer(b, w)
}

func (b *SchiBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	return ContainerInfo(b, w, specificBoxLevels, indent, indentStep)
}
