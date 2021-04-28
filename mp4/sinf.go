package mp4

import (
	"io"
)

// SchiBox -  Protection Scheme Information Box
type SinfBox struct {
	Frma     *FrmaBox // Mandatory
	Schm     *SchmBox // Optional
	Schi     *SchiBox // Optional
	Children []Box
}

// AddChild - Add a child box
func (b *SinfBox) AddChild(box Box) {
	switch box.Type() {
	case "frma":
		b.Frma = box.(*FrmaBox)
	case "schm":
		b.Schm = box.(*SchmBox)
	case "schi":
		b.Schi = box.(*SchiBox)
	}
	b.Children = append(b.Children, box)
}

// DecodeSinf - box-specific decode
func DecodeSinf(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	children, err := DecodeContainerChildren(hdr, startPos+8, startPos+hdr.size, r)
	if err != nil {
		return nil, err
	}
	b := &SinfBox{}
	for _, child := range children {
		b.AddChild(child)
	}
	return b, nil
}

// Type - box type
func (b *SinfBox) Type() string {
	return "sinf"
}

// Size - calculated size of box
func (b *SinfBox) Size() uint64 {
	return containerSize(b.Children)
}

// GetChildren - list of child boxes
func (b *SinfBox) GetChildren() []Box {
	return b.Children
}

// Encode - write minf container to w
func (b *SinfBox) Encode(w io.Writer) error {
	return EncodeContainer(b, w)
}

func (b *SinfBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	return ContainerInfo(b, w, specificBoxLevels, indent, indentStep)
}
