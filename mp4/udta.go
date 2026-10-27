package mp4

import (
	"fmt"
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
	// TrailingBytes are bytes after the last child that are too few for a box header, such as
	// the 32-bit zero that may end the user data list in QuickTime files. They are written back
	// after the children.
	TrailingBytes []byte
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
	var trailing []byte
	children, err := decodeContainerChildren(hdr, startPos+8, startPos+hdr.Size, r, &trailing)
	if err != nil {
		return nil, err
	}
	b := UdtaBox{Children: make([]Box, 0, len(children)), TrailingBytes: trailing}
	for _, c := range children {
		b.AddChild(c)
	}
	return &b, nil
}

// DecodeUdtaSR - box-specific decode
func DecodeUdtaSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	var trailing []byte
	children, err := decodeContainerChildrenSR(hdr, startPos+8, startPos+hdr.Size, sr, make([]Box, 0, 8), &trailing)
	if err != nil {
		return nil, err
	}
	b := UdtaBox{Children: make([]Box, 0, len(children)), TrailingBytes: trailing}
	for _, c := range children {
		b.AddChild(c)
	}
	return &b, sr.AccError()
}

// Type - box type
func (b *UdtaBox) Type() string {
	return "udta"
}

// Size - calculated size of box
func (b *UdtaBox) Size() uint64 {
	return containerSize(b.Children) + uint64(len(b.TrailingBytes))
}

// GetChildren - list of child boxes
func (b *UdtaBox) GetChildren() []Box {
	return b.Children
}

// Encode - write udta container to w
func (b *UdtaBox) Encode(w io.Writer) error {
	err := EncodeContainer(b, w)
	if err != nil || len(b.TrailingBytes) == 0 {
		return err
	}
	_, err = w.Write(b.TrailingBytes)
	return err
}

// EncodeSW - write udta container to sw
func (b *UdtaBox) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeContainerSW(b, sw)
	if err != nil {
		return err
	}
	sw.WriteBytes(b.TrailingBytes)
	return sw.AccError()
}

// Info - write box-specific information
func (b *UdtaBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	err := ContainerInfo(b, w, specificBoxLevels, indent, indentStep)
	if err != nil || len(b.TrailingBytes) == 0 {
		return err
	}
	_, err = fmt.Fprintf(w, "%s - trailingBytes: %x\n", indent, b.TrailingBytes)
	return err
}
