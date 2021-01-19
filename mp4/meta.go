package mp4

import (
	"encoding/binary"
	"io"
)

// MetaBox - MetaBox meta ISO/IEC 14496-12 Ed. 6 2020 Section 8.11
type MetaBox struct {
	Version  byte
	Flags    uint32
	Hdlr     *HdlrBox
	Children []Box
}

// CreateMetaBox - Create a new MetaBox
func CreateMetaBox(version byte, hdlr *HdlrBox) *MetaBox {
	b := &MetaBox{
		Version: version,
		Flags:   0,
	}
	b.AddChild(hdlr)
	return b
}

// AddChild - Add a child box
func (b *MetaBox) AddChild(box Box) {

	switch box.Type() {
	case "hdlr":
		b.Hdlr = box.(*HdlrBox)
	}
	b.Children = append(b.Children, box)
}

// DecodeMinf - box-specific decode
func DecodeMeta(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	var versionAndFlags uint32
	err := binary.Read(r, binary.BigEndian, &versionAndFlags)
	if err != nil {
		return nil, err
	}
	//Note higher startPos below since not simple container
	children, err := DecodeContainerChildren(hdr, startPos+12, startPos+hdr.size, r)
	if err != nil {
		return nil, err
	}
	b := &MetaBox{
		Version: byte(versionAndFlags >> 24),
		Flags:   versionAndFlags & flagsMask,
	}
	for _, child := range children {
		b.AddChild(child)
	}
	return b, nil
}

// Type - box type
func (b *MetaBox) Type() string {
	return "meta"
}

// Size - calculated size of box
func (b *MetaBox) Size() uint64 {
	return 4 + containerSize(b.Children)
}

// GetChildren - list of child boxes
func (b *MetaBox) GetChildren() []Box {
	return b.Children
}

// Encode - write minf container to w
func (b *MetaBox) Encode(w io.Writer) error {
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	versionAndFlags := (uint32(b.Version) << 24) + b.Flags
	err = binary.Write(w, binary.BigEndian, versionAndFlags)
	if err != nil {
		return err
	}
	for _, b := range b.Children {
		err = b.Encode(w)
		if err != nil {
			return err
		}
	}
	return nil
}

func (b *MetaBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, int(b.Version), b.Flags)
	if bd.err != nil {
		return bd.err
	}
	var err error
	for _, c := range b.Children {
		err = c.Info(w, specificBoxLevels, indent+indentStep, indentStep)
		if err != nil {
			return err
		}
	}
	return err
}
