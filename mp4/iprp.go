package mp4

import (
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// IprpBox - Item Properties Box (iprp), ISO/IEC 14496-12:2026 Sec. 8.11.14
//
// Contained in: Meta Box (meta)
//
// It holds the item properties in Ipco, and their association with items in one or more ipma boxes.
type IprpBox struct {
	Ipco     *IpcoBox
	Ipmas    []*IpmaBox
	Children []Box
}

// AddChild adds a child box
func (b *IprpBox) AddChild(child Box) {
	switch box := child.(type) {
	case *IpcoBox:
		b.Ipco = box
	case *IpmaBox:
		b.Ipmas = append(b.Ipmas, box)
	}
	b.Children = append(b.Children, child)
}

// DecodeIprp - box-specific decode
func DecodeIprp(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	children, err := DecodeContainerChildren(hdr, startPos+8, startPos+hdr.Size, r)
	if err != nil {
		return nil, err
	}
	b := &IprpBox{}
	for _, c := range children {
		b.AddChild(c)
	}
	return b, nil
}

// DecodeIprpSR - box-specific decode
func DecodeIprpSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	children, err := DecodeContainerChildrenSR(hdr, startPos+8, startPos+hdr.Size, sr)
	if err != nil {
		return nil, err
	}
	b := &IprpBox{}
	for _, c := range children {
		b.AddChild(c)
	}
	return b, nil
}

// Type - box type
func (b *IprpBox) Type() string {
	return "iprp"
}

// Size - calculated size of box
func (b *IprpBox) Size() uint64 {
	return containerSize(b.Children)
}

// GetChildren - list of child boxes
func (b *IprpBox) GetChildren() []Box {
	return b.Children
}

// Encode - write iprp container to w
func (b *IprpBox) Encode(w io.Writer) error {
	return EncodeContainer(b, w)
}

// EncodeSW - write container using slice writer
func (b *IprpBox) EncodeSW(sw bits.SliceWriter) error {
	return EncodeContainerSW(b, sw)
}

// Info - write box-specific information
func (b *IprpBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	return ContainerInfo(b, w, specificBoxLevels, indent, indentStep)
}
