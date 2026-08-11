package mp4

import (
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// UdtaBox - User Data Box is a container for User Data
//
// Contained in : moov, trak, moof, or traf
type UdtaBox struct {
	// Labls are the labl children in order. There may be multiple ones, e.g. one per language.
	Labls    []*LablBox
	Children []Box
}

// AddChild - Add a child box
func (b *UdtaBox) AddChild(box Box) {
	if labl, ok := box.(*LablBox); ok {
		b.Labls = append(b.Labls, labl)
	}
	b.Children = append(b.Children, box)
}

// GroupLabl returns the group label of the labl group labelID, or nil if there is none.
// A group label carries the summary or title of all labels sharing its labelID,
// e.g. the name to present in a selection menu.
func (b *UdtaBox) GroupLabl(labelID uint16) *LablBox {
	for _, labl := range b.Labls {
		if labl.LabelID == labelID && labl.IsGroupLabel() {
			return labl
		}
	}
	return nil
}

// DecodeUdta - box-specific decode
func DecodeUdta(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	children, err := DecodeContainerChildren(hdr, startPos+8, startPos+hdr.Size, r)
	if err != nil {
		return nil, err
	}
	b := UdtaBox{Children: make([]Box, 0, len(children))}
	for _, c := range children {
		b.AddChild(c)
	}
	return &b, nil
}

// DecodeUdtaSR - box-specific decode
func DecodeUdtaSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	children, err := DecodeContainerChildrenSR(hdr, startPos+8, startPos+hdr.Size, sr)
	if err != nil {
		return nil, err
	}
	b := UdtaBox{Children: make([]Box, 0, len(children))}
	for _, c := range children {
		b.AddChild(c)
	}
	return &b, nil
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

// Encode - write udta container to sw
func (b *UdtaBox) EncodeSW(sw bits.SliceWriter) error {
	return EncodeContainerSW(b, sw)
}

// Info - write box-specific information
func (b *UdtaBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	return ContainerInfo(b, w, specificBoxLevels, indent, indentStep)
}
