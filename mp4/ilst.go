package mp4

import (
	"encoding/binary"
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// IlstBox - iTunes Metadata Item List Atom (ilst)
// See https://developer.apple.com/library/archive/documentation/QuickTime/QTFF/Metadata/Metadata.html
type IlstBox struct {
	Children []Box
}

// AddChild adds a metadata item, and tells its data boxes the type of the item and the name of a freeform item,
// so that their Info tells what their values mean.
func (b *IlstBox) AddChild(child Box) {
	setItemContext(child)
	b.Children = append(b.Children, child)
}

// setItemContext tells the data boxes of the metadata item the type of the item and, for a freeform item ----,
// its name.
func setItemContext(item Box) {
	c, ok := item.(interface{ GetChildren() []Box })
	if !ok {
		return
	}
	freeform := ""
	for _, ch := range c.GetChildren() {
		if n, ok := ch.(*NameBox); ok && n.FullBox {
			freeform = n.Name
		}
	}
	for _, ch := range c.GetChildren() {
		if d, ok := ch.(*DataBox); ok {
			d.item, d.freeform = item.Type(), freeform
		}
	}
}

// DecodeIlstSR - box-specific decode
//
// A child of a type that mp4ff does not know, but which holds data, mean, name or itif boxes, as metadata items
// do, decodes as a GenericContainerBox. That is the case for iTunes items such as trkn and covr, and for the
// items of QuickTime metadata, whose types are the indices of their keys in the keys atom.
func DecodeIlstSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	children, err := DecodeContainerChildrenSR(hdr, startPos+8, startPos+hdr.Size, sr)
	if err != nil {
		return nil, err
	}
	b := &IlstBox{}
	for _, c := range itemsFromUnknown(children, startPos+8) {
		b.AddChild(c)
	}
	return b, nil
}

// DecodeIlst - box-specific decode, as DecodeIlstSR
func DecodeIlst(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	children, err := DecodeContainerChildren(hdr, startPos+8, startPos+hdr.Size, r)
	if err != nil {
		return nil, err
	}
	b := &IlstBox{}
	for _, c := range itemsFromUnknown(children, startPos+8) {
		b.AddChild(c)
	}
	return b, nil
}

// Type - box-specific type
func (b *IlstBox) Type() string {
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

// Encode - write ilst container to w
func (b *IlstBox) Encode(w io.Writer) error {
	return EncodeContainer(b, w)
}

// EncodeSW - write ilst container to sw
func (b *IlstBox) EncodeSW(sw bits.SliceWriter) error {
	return EncodeContainerSW(b, sw)
}

// Info - write box-specific information
func (b *IlstBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	return ContainerInfo(b, w, specificBoxLevels, indent, indentStep)
}

// itemsFromUnknown decodes the children of ilst that decoded as UnknownBox, but hold the boxes of a metadata
// item, as GenericContainerBox. The first child starts at position pos.
func itemsFromUnknown(children []Box, pos uint64) []Box {
	for i, c := range children {
		size := c.Size()
		u, ok := c.(*UnknownBox)
		if ok && size == uint64(boxHeaderSize+len(u.Payload())) && isItemPayload(u.Payload()) {
			hdr := BoxHeader{Name: u.Type(), Size: size, Hdrlen: boxHeaderSize}
			sr := bits.NewFixedSliceReader(u.Payload())
			if item, err := DecodeGenericContainerBoxSR(hdr, pos, sr); err == nil && sr.NrRemainingBytes() == 0 {
				children[i] = item
			}
		}
		pos += size
	}
	return children
}

// isItemPayload tells if payload starts with a data, mean, name or itif box, as the payload of a metadata item does.
func isItemPayload(payload []byte) bool {
	if len(payload) < boxHeaderSize {
		return false
	}
	size := binary.BigEndian.Uint32(payload)
	switch string(payload[4:8]) {
	case "data", "mean", "name", "itif":
		return size >= boxHeaderSize && int(size) <= len(payload)
	}
	return false
}
