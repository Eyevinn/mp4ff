package mp4

import (
	"fmt"
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// IinfBox - Item Information Box (iinf), ISO/IEC 14496-12:2026 Sec. 8.11.6
//
// Contained in: Meta Box (meta)
//
// The children are the item info entries. Their number is written as the entry count,
// which is 16 bits in version 0 and 32 bits in other versions.
type IinfBox struct {
	Version  byte
	Flags    uint32
	Infes    []*InfeBox
	Children []Box
}

// AddChild adds a child box
func (b *IinfBox) AddChild(child Box) {
	if infe, ok := child.(*InfeBox); ok {
		b.Infes = append(b.Infes, infe)
	}
	b.Children = append(b.Children, child)
}

// GetInfe returns the entry of the item with itemID, or nil if there is none.
func (b *IinfBox) GetInfe(itemID uint32) *InfeBox {
	for _, infe := range b.Infes {
		if infe.ItemID == itemID {
			return infe
		}
	}
	return nil
}

// DecodeIinf - box-specific decode
func DecodeIinf(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	sr := ioPayloadReader(data)
	return DecodeIinfSR(hdr, startPos, sr)
}

// DecodeIinfSR - box-specific decode
func DecodeIinfSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	versionAndFlags := sr.ReadUint32()
	b := IinfBox{
		Version: byte(versionAndFlags >> 24),
		Flags:   versionAndFlags & flagsMask,
	}
	var entryCount uint32
	if b.Version == 0 {
		entryCount = uint32(sr.ReadUint16())
	} else {
		entryCount = sr.ReadUint32()
	}
	if err := sr.AccError(); err != nil {
		return nil, fmt.Errorf("iinf: %w", err)
	}
	// Note higher startPos for children since not simple container.
	children, err := DecodeContainerChildrenSR(hdr, startPos+b.childrenOffset(), startPos+hdr.Size, sr)
	if err != nil {
		return nil, err
	}
	if uint64(entryCount) != uint64(len(children)) {
		return nil, fmt.Errorf("iinf: entry count %d but %d entries", entryCount, len(children))
	}
	for _, c := range children {
		b.AddChild(c)
	}
	return &b, nil
}

// childrenOffset returns the offset of the first child from the start of the box.
func (b *IinfBox) childrenOffset() uint64 {
	if b.Version == 0 {
		return boxHeaderSize + 4 + 2
	}
	return boxHeaderSize + 4 + 4
}

// Type - box type
func (b *IinfBox) Type() string {
	return "iinf"
}

// Size - calculated size of box
func (b *IinfBox) Size() uint64 {
	return containerSize(b.Children) - boxHeaderSize + b.childrenOffset()
}

// GetChildren - list of child boxes
func (b *IinfBox) GetChildren() []Box {
	return b.Children
}

// Encode - write box to w
func (b *IinfBox) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - box-specific encode to slicewriter
//
// It returns an error for more children than the 65535 of version 0.
func (b *IinfBox) EncodeSW(sw bits.SliceWriter) error {
	if b.Version == 0 && len(b.Children) > 0xffff {
		return fmt.Errorf("iinf: %d entries need version 1", len(b.Children))
	}
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	versionAndFlags := (uint32(b.Version) << 24) + b.Flags
	sw.WriteUint32(versionAndFlags)
	if b.Version == 0 {
		sw.WriteUint16(uint16(len(b.Children)))
	} else {
		sw.WriteUint32(uint32(len(b.Children)))
	}
	for _, c := range b.Children {
		err = c.EncodeSW(sw)
		if err != nil {
			return err
		}
	}
	return sw.AccError()
}

// Info - write box-specific information
func (b *IinfBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, int(b.Version), b.Flags)
	if bd.err != nil {
		return bd.err
	}
	for _, c := range b.Children {
		err := c.Info(w, specificBoxLevels, indent+indentStep, indentStep)
		if err != nil {
			return err
		}
	}
	return nil
}
