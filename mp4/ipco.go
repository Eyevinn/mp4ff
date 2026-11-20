package mp4

import (
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// IpcoBox - Item Property Container Box (ipco), ISO/IEC 14496-12:2026 Sec. 8.11.14
//
// Contained in: Item Properties Box (iprp)
//
// The children are the item properties. An ipma box refers to them by their one-based index among
// all children, so their order matters and a free box counts as well.
type IpcoBox struct {
	Children []Box
}

// AddChild adds a child box
func (b *IpcoBox) AddChild(child Box) {
	b.Children = append(b.Children, child)
}

// GetProperty returns the property with the one-based index, or nil if there is none.
func (b *IpcoBox) GetProperty(index uint16) Box {
	if index == 0 || int(index) > len(b.Children) {
		return nil
	}
	return b.Children[index-1]
}

// DecodeIpco - box-specific decode
func DecodeIpco(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	children, err := DecodeContainerChildren(hdr, startPos+8, startPos+hdr.Size, r)
	if err != nil {
		return nil, err
	}
	return &IpcoBox{Children: children}, nil
}

// DecodeIpcoSR - box-specific decode
func DecodeIpcoSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	children, err := DecodeContainerChildrenSR(hdr, startPos+8, startPos+hdr.Size, sr)
	if err != nil {
		return nil, err
	}
	return &IpcoBox{Children: children}, nil
}

// Type - box type
func (b *IpcoBox) Type() string {
	return "ipco"
}

// Size - calculated size of box
func (b *IpcoBox) Size() uint64 {
	return containerSize(b.Children)
}

// GetChildren - list of child boxes
func (b *IpcoBox) GetChildren() []Box {
	return b.Children
}

// Encode - write ipco container to w
func (b *IpcoBox) Encode(w io.Writer) error {
	return EncodeContainer(b, w)
}

// EncodeSW - write container using slice writer
func (b *IpcoBox) EncodeSW(sw bits.SliceWriter) error {
	return EncodeContainerSW(b, sw)
}

// Info - write box-specific information
func (b *IpcoBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	return ContainerInfo(b, w, specificBoxLevels, indent, indentStep)
}
