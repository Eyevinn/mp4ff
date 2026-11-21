package mp4

import (
	"fmt"
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// IrefBox - Item Reference Box (iref), ISO/IEC 14496-12:2026 Sec. 8.11.12
//
// Contained in: Meta Box (meta)
//
// The children are the references between items, one IrefTypeBox for each referring item and
// reference type. Their item IDs are 16 bits in version 0 and 32 bits in version 1.
type IrefBox struct {
	Version  byte
	Flags    uint32
	Refs     []*IrefTypeBox
	Children []Box
}

// AddChild adds a child box
func (b *IrefBox) AddChild(child Box) {
	if ref, ok := child.(*IrefTypeBox); ok {
		ref.large = b.Version != 0
		b.Refs = append(b.Refs, ref)
	}
	b.Children = append(b.Children, child)
}

// setItemIDSizes makes the children use the item ID size of the version of b.
func (b *IrefBox) setItemIDSizes() {
	large := b.Version != 0
	for _, c := range b.Children {
		if ref, ok := c.(*IrefTypeBox); ok && ref.large != large {
			ref.large = large
		}
	}
}

// DecodeIref - box-specific decode
func DecodeIref(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	return DecodeIrefSR(hdr, startPos, bits.NewFixedSliceReader(data))
}

// DecodeIrefSR - box-specific decode
//
// Every child is decoded as an IrefTypeBox, whatever its box type.
func DecodeIrefSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	versionAndFlags := sr.ReadUint32()
	b := IrefBox{
		Version: byte(versionAndFlags >> 24),
		Flags:   versionAndFlags & flagsMask,
	}
	if err := sr.AccError(); err != nil {
		return nil, fmt.Errorf("iref: %w", err)
	}
	if b.Version > 1 {
		return nil, fmt.Errorf("iref: version %d is not supported", b.Version)
	}
	end := startPos + hdr.Size
	for pos := startPos + uint64(hdr.Hdrlen) + 4; pos < end; {
		if end-pos < boxHeaderSize {
			return nil, fmt.Errorf("iref: %d bytes after the last reference", end-pos)
		}
		chdr, err := DecodeHeaderSR(sr)
		if err != nil {
			return nil, fmt.Errorf("iref: %w", err)
		}
		if chdr.Size > end-pos {
			return nil, fmt.Errorf("iref: child %s of size %d goes beyond the end of iref", chdr.Name, chdr.Size)
		}
		child, err := decodeIrefTypeSR(chdr, sr, b.Version != 0)
		if err != nil {
			return nil, fmt.Errorf("iref: %w", err)
		}
		b.AddChild(child)
		pos += chdr.Size
	}
	return &b, nil
}

// Type - box type
func (b *IrefBox) Type() string {
	return "iref"
}

// Size - calculated size of box
func (b *IrefBox) Size() uint64 {
	b.setItemIDSizes()
	return containerSize(b.Children) + 4
}

// GetChildren - list of child boxes
func (b *IrefBox) GetChildren() []Box {
	return b.Children
}

// Encode - write box to w
func (b *IrefBox) Encode(w io.Writer) error {
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
// It returns an error for a version other than 0 and 1, and for a child that cannot be encoded.
func (b *IrefBox) EncodeSW(sw bits.SliceWriter) error {
	if b.Version > 1 {
		return fmt.Errorf("iref: version %d is not supported", b.Version)
	}
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	versionAndFlags := (uint32(b.Version) << 24) + b.Flags
	sw.WriteUint32(versionAndFlags)
	for _, c := range b.Children {
		err = c.EncodeSW(sw)
		if err != nil {
			return err
		}
	}
	return sw.AccError()
}

// Info - write box-specific information
func (b *IrefBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	b.setItemIDSizes()
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

// IrefTypeBox - Single Item Type Reference Box, ISO/IEC 14496-12:2026 Sec. 8.11.12
//
// Contained in: Item Reference Box (iref)
//
// It tells that the item FromItemID refers to the items ToItemIDs, whose order matters.
// Name is the reference type, which is the box type. ISO/IEC 23008-12:2025 defines thmb from a
// thumbnail to its image (Sec. 6.4.4), auxl from an auxiliary image, such as an alpha plane, to its
// image (Sec. 6.4.5), dimg from a derived image, such as a grid, to its input images (Sec. 6.6.1),
// and cdsc from a metadata item to the image it describes (Sec. 6.7).
//
// The item IDs are written with 16 bits, or with 32 bits in an IrefBox of version 1.
type IrefTypeBox struct {
	Name       string
	FromItemID uint32
	ToItemIDs  []uint32
	large      bool // 32-bit item IDs
}

// itemIDSize returns the size in bytes of an item ID.
func (b *IrefTypeBox) itemIDSize() int {
	if b.large {
		return 4
	}
	return 2
}

// decodeIrefTypeSR decodes the payload of a child of an iref box, with 32-bit item IDs if large.
func decodeIrefTypeSR(hdr BoxHeader, sr bits.SliceReader, large bool) (*IrefTypeBox, error) {
	b := IrefTypeBox{Name: hdr.Name, large: large}
	idSize := b.itemIDSize()
	payloadLen := hdr.payloadLen()
	if payloadLen < idSize+2 {
		return nil, fmt.Errorf("%s: %d bytes are too few for an item reference", hdr.Name, payloadLen)
	}
	readItemID := func() uint32 {
		if large {
			return sr.ReadUint32()
		}
		return uint32(sr.ReadUint16())
	}
	b.FromItemID = readItemID()
	refCount := int(sr.ReadUint16())
	if refBytes := payloadLen - idSize - 2; refBytes != refCount*idSize {
		return nil, fmt.Errorf("%s: %d references do not match %d bytes", hdr.Name, refCount, refBytes)
	}
	if refCount > 0 {
		b.ToItemIDs = make([]uint32, refCount)
		for i := range b.ToItemIDs {
			b.ToItemIDs[i] = readItemID()
		}
	}
	if err := sr.AccError(); err != nil {
		return nil, fmt.Errorf("%s: %w", hdr.Name, err)
	}
	return &b, nil
}

// Type - box type
func (b *IrefTypeBox) Type() string {
	return b.Name
}

// Size - calculated size of box
func (b *IrefTypeBox) Size() uint64 {
	idSize := b.itemIDSize()
	return uint64(boxHeaderSize + idSize + 2 + len(b.ToItemIDs)*idSize)
}

// Encode - write box to w
func (b *IrefTypeBox) Encode(w io.Writer) error {
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
// It returns an error for a name that is not four bytes, for more than 65535 references and
// for an item ID that does not fit in its field.
func (b *IrefTypeBox) EncodeSW(sw bits.SliceWriter) error {
	if len(b.Name) != 4 {
		return fmt.Errorf("item reference type %q is not 4 bytes", b.Name)
	}
	if len(b.ToItemIDs) > 0xffff {
		return fmt.Errorf("%s: %d references are more than 65535", b.Name, len(b.ToItemIDs))
	}
	if !b.large {
		if b.FromItemID > 0xffff {
			return fmt.Errorf("%s: item ID %d needs iref version 1", b.Name, b.FromItemID)
		}
		for _, id := range b.ToItemIDs {
			if id > 0xffff {
				return fmt.Errorf("%s: item ID %d needs iref version 1", b.Name, id)
			}
		}
	}
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	writeItemID := func(id uint32) {
		if b.large {
			sw.WriteUint32(id)
		} else {
			sw.WriteUint16(uint16(id))
		}
	}
	writeItemID(b.FromItemID)
	sw.WriteUint16(uint16(len(b.ToItemIDs)))
	for _, id := range b.ToItemIDs {
		writeItemID(id)
	}
	return sw.AccError()
}

// Info - write box-specific information
func (b *IrefTypeBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, -1, 0)
	bd.write(" - fromItemID: %d", b.FromItemID)
	bd.write(" - toItemIDs: %v", b.ToItemIDs)
	return bd.err
}
