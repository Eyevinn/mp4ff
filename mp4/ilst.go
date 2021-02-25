package mp4

import (
	"io"
)

// IlstBox - iTunes Metadata Item List Atom (ilst)
// See https://developer.apple.com/library/archive/documentation/QuickTime/QTFF/Metadata/Metadata.html
type IlstBox struct {
	Children []Box
}

// AddChild - Add a child box and update SampleCount
func (s *IlstBox) AddChild(child Box) {
	s.Children = append(s.Children, child)
}

// DecodeIlst - box-specific decode
func DecodeIlst(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	children, err := DecodeContainerChildren(hdr, startPos+8, startPos+hdr.size, r)
	if err != nil {
		return nil, err
	}
	b := &IlstBox{}
	for _, c := range children {
		b.AddChild(c)
	}
	return b, nil
}

// Type - box-specific type
func (s *IlstBox) Type() string {
	return "ilst"
}

// Size - box-specific type
func (b *IlstBox) Size() uint64 {
	return containerSize(b.Children)
}

// GetChildren - list of child boxes
func (b *IlstBox) GetChildren() []Box {
	return b.Children
}

// Encode - box-specific encode of stsd - not a usual container
func (b *IlstBox) Encode(w io.Writer) error {
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	for _, c := range b.Children {
		err = c.Encode(w)
		if err != nil {
			return err
		}
	}
	return nil
}

func (b *IlstBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	return ContainerInfo(b, w, specificBoxLevels, indent, indentStep)
}
