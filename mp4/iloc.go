package mp4

import (
	"fmt"
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// Construction methods of an item in IlocBox, telling what the offsets of its extents refer to.
const (
	IlocConstructionFile = 0 // offsets into the file, or into the data referenced by DataReferenceIndex
	IlocConstructionIdat = 1 // offsets into the idat box of the same meta box
	IlocConstructionItem = 2 // offsets into the item given by ItemReferenceIndex
)

// ilocMaxFieldSize is the maximum size in bytes of an offset, length, base offset or index field.
const ilocMaxFieldSize = 8

// IlocBox - Item Location Box (iloc), ISO/IEC 14496-12:2026 Sec. 8.11.3
//
// Contained in: Meta Box (meta)
//
// It tells where the data of each item is stored, as a list of extents.
//
// OffsetSize, LengthSize, BaseOffsetSize and IndexSize are the sizes in bytes of the Offset, Length,
// BaseOffset and ItemReferenceIndex fields. The specification allows 0, 4 and 8, where 0 means that
// the field is absent and its value is 0. All sizes from 0 to 8 are decoded and encoded.
// IndexSize and ConstructionMethod are only present in version 1 and 2.
// Item IDs are 16 bits in version 0 and 1, and 32 bits in version 2.
type IlocBox struct {
	Version        byte
	Flags          uint32
	OffsetSize     byte
	LengthSize     byte
	BaseOffsetSize byte
	IndexSize      byte
	Items          []IlocItem
}

// IlocItem is the location of one item in IlocBox.
type IlocItem struct {
	ItemID             uint32
	ConstructionMethod byte
	DataReferenceIndex uint16
	BaseOffset         uint64
	Extents            []IlocExtent
}

// IlocExtent is a contiguous part of the data of an item. Offset is relative to BaseOffset of the item.
// Length 0 means all of the referenced data.
type IlocExtent struct {
	ItemReferenceIndex uint64
	Offset             uint64
	Length             uint64
}

// DecodeIloc - box-specific decode
func DecodeIloc(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	sr := bits.NewFixedSliceReader(data)
	return DecodeIlocSR(hdr, startPos, sr)
}

// DecodeIlocSR - box-specific decode
func DecodeIlocSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	initPos := sr.GetPos()
	remaining := func() int {
		return hdr.payloadLen() - (sr.GetPos() - initPos)
	}
	versionAndFlags := sr.ReadUint32()
	b := IlocBox{
		Version: byte(versionAndFlags >> 24),
		Flags:   versionAndFlags & flagsMask,
	}
	if b.Version > 2 {
		return nil, fmt.Errorf("iloc: version %d is not supported", b.Version)
	}
	sizes := sr.ReadUint16()
	b.OffsetSize = byte(sizes >> 12)
	b.LengthSize = byte(sizes>>8) & 0x0f
	b.BaseOffsetSize = byte(sizes>>4) & 0x0f
	if b.Version > 0 {
		b.IndexSize = byte(sizes) & 0x0f
	}
	if err := b.checkFieldSizes(); err != nil {
		return nil, err
	}
	var itemCount uint32
	if b.Version < 2 {
		itemCount = uint32(sr.ReadUint16())
	} else {
		itemCount = sr.ReadUint32()
	}
	if err := sr.AccError(); err != nil {
		return nil, fmt.Errorf("iloc: %w", err)
	}
	if uint64(itemCount)*uint64(b.itemSize(0)) > uint64(remaining()) {
		return nil, fmt.Errorf("iloc: %d items do not fit in %d bytes", itemCount, remaining())
	}
	b.Items = make([]IlocItem, itemCount)
	for i := range b.Items {
		item := &b.Items[i]
		if b.Version < 2 {
			item.ItemID = uint32(sr.ReadUint16())
		} else {
			item.ItemID = sr.ReadUint32()
		}
		if b.Version > 0 {
			item.ConstructionMethod = byte(sr.ReadUint16() & 0x0f)
		}
		item.DataReferenceIndex = sr.ReadUint16()
		item.BaseOffset = readUintN(sr, b.BaseOffsetSize)
		extentCount := int(sr.ReadUint16())
		if err := sr.AccError(); err != nil {
			return nil, fmt.Errorf("iloc: %w", err)
		}
		if extentCount*b.extentSize() > remaining() {
			return nil, fmt.Errorf("iloc: %d extents of item %d do not fit in %d bytes",
				extentCount, item.ItemID, remaining())
		}
		if extentCount > 1 && b.extentSize() == 0 {
			return nil, fmt.Errorf("iloc: %d extents of item %d without any field", extentCount, item.ItemID)
		}
		item.Extents = make([]IlocExtent, extentCount)
		for j := range item.Extents {
			if b.Version > 0 {
				item.Extents[j].ItemReferenceIndex = readUintN(sr, b.IndexSize)
			}
			item.Extents[j].Offset = readUintN(sr, b.OffsetSize)
			item.Extents[j].Length = readUintN(sr, b.LengthSize)
		}
	}
	if err := sr.AccError(); err != nil {
		return nil, fmt.Errorf("iloc: %w", err)
	}
	if n := remaining(); n != 0 {
		return nil, fmt.Errorf("iloc: %d bytes after the last item", n)
	}
	return &b, nil
}

// readUintN reads an unsigned big-endian integer of n bytes, where n is at most 8.
func readUintN(sr bits.SliceReader, n byte) uint64 {
	switch n {
	case 0:
		return 0
	case 4:
		return uint64(sr.ReadUint32())
	case 8:
		return sr.ReadUint64()
	}
	var v uint64
	for range n {
		v = v<<8 | uint64(sr.ReadUint8())
	}
	return v
}

// writeUintN writes v as an unsigned big-endian integer of n bytes, where n is at most 8.
func writeUintN(sw bits.SliceWriter, v uint64, n byte) {
	for i := int(n) - 1; i >= 0; i-- {
		sw.WriteUint8(byte(v >> (8 * i)))
	}
}

// fitsUintN tells if v fits in an unsigned integer of n bytes.
func fitsUintN(v uint64, n byte) bool {
	return n >= 8 || v>>(8*n) == 0
}

// checkFieldSizes checks that no field size is beyond what fits in the fields of the box.
func (b *IlocBox) checkFieldSizes() error {
	for _, size := range []byte{b.OffsetSize, b.LengthSize, b.BaseOffsetSize, b.IndexSize} {
		if size > ilocMaxFieldSize {
			return fmt.Errorf("iloc: field size %d is more than %d bytes", size, ilocMaxFieldSize)
		}
	}
	return nil
}

// extentSize returns the size in bytes of an extent.
func (b *IlocBox) extentSize() int {
	size := int(b.OffsetSize) + int(b.LengthSize)
	if b.Version > 0 {
		size += int(b.IndexSize)
	}
	return size
}

// itemSize returns the size in bytes of an item with nrExtents extents.
func (b *IlocBox) itemSize(nrExtents int) int {
	size := 2 + 2 + int(b.BaseOffsetSize) + 2 // item ID, data reference index, base offset, extent count
	if b.Version == 2 {
		size += 2 // 32-bit item ID
	}
	if b.Version > 0 {
		size += 2 // construction method
	}
	return size + nrExtents*b.extentSize()
}

// Type - box type
func (b *IlocBox) Type() string {
	return "iloc"
}

// Size - calculated size of box
func (b *IlocBox) Size() uint64 {
	size := uint64(boxHeaderSize + 4 + 2 + 2) // sizes and 16-bit item count
	if b.Version == 2 {
		size += 2 // 32-bit item count
	}
	for i := range b.Items {
		size += uint64(b.itemSize(len(b.Items[i].Extents)))
	}
	return size
}

// Encode - write box to w
func (b *IlocBox) Encode(w io.Writer) error {
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
// It returns an error for a value that does not fit in its field, given the version and the field sizes.
func (b *IlocBox) EncodeSW(sw bits.SliceWriter) error {
	if err := b.checkEncodable(); err != nil {
		return err
	}
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	versionAndFlags := (uint32(b.Version) << 24) + b.Flags
	sw.WriteUint32(versionAndFlags)
	sizes := uint16(b.OffsetSize)<<12 | uint16(b.LengthSize)<<8 | uint16(b.BaseOffsetSize)<<4
	if b.Version > 0 {
		sizes |= uint16(b.IndexSize)
	}
	sw.WriteUint16(sizes)
	if b.Version < 2 {
		sw.WriteUint16(uint16(len(b.Items)))
	} else {
		sw.WriteUint32(uint32(len(b.Items)))
	}
	for i := range b.Items {
		item := &b.Items[i]
		if b.Version < 2 {
			sw.WriteUint16(uint16(item.ItemID))
		} else {
			sw.WriteUint32(item.ItemID)
		}
		if b.Version > 0 {
			sw.WriteUint16(uint16(item.ConstructionMethod))
		}
		sw.WriteUint16(item.DataReferenceIndex)
		writeUintN(sw, item.BaseOffset, b.BaseOffsetSize)
		sw.WriteUint16(uint16(len(item.Extents)))
		for _, e := range item.Extents {
			if b.Version > 0 {
				writeUintN(sw, e.ItemReferenceIndex, b.IndexSize)
			}
			writeUintN(sw, e.Offset, b.OffsetSize)
			writeUintN(sw, e.Length, b.LengthSize)
		}
	}
	return sw.AccError()
}

// checkEncodable checks that the version is supported and that all values fit in their fields.
func (b *IlocBox) checkEncodable() error {
	if b.Version > 2 {
		return fmt.Errorf("iloc: version %d is not supported", b.Version)
	}
	if err := b.checkFieldSizes(); err != nil {
		return err
	}
	maxItems, maxItemID := uint64(0xffff), uint32(0xffff)
	if b.Version == 2 {
		maxItems, maxItemID = 0xffffffff, 0xffffffff
	}
	if uint64(len(b.Items)) > maxItems {
		return fmt.Errorf("iloc: %d items are too many for version %d", len(b.Items), b.Version)
	}
	for i := range b.Items {
		item := &b.Items[i]
		switch {
		case item.ItemID > maxItemID:
			return fmt.Errorf("iloc: item ID %d needs version 2", item.ItemID)
		case item.ConstructionMethod > 0x0f || (b.Version == 0 && item.ConstructionMethod != 0):
			return fmt.Errorf("iloc: construction method %d of item %d does not fit in version %d",
				item.ConstructionMethod, item.ItemID, b.Version)
		case !fitsUintN(item.BaseOffset, b.BaseOffsetSize):
			return fmt.Errorf("iloc: base offset %d of item %d does not fit in %d bytes",
				item.BaseOffset, item.ItemID, b.BaseOffsetSize)
		case len(item.Extents) > 0xffff:
			return fmt.Errorf("iloc: item %d has %d extents, more than 65535", item.ItemID, len(item.Extents))
		case len(item.Extents) > 1 && b.extentSize() == 0:
			return fmt.Errorf("iloc: %d extents of item %d without any field", len(item.Extents), item.ItemID)
		}
		for _, e := range item.Extents {
			switch {
			case !fitsUintN(e.Offset, b.OffsetSize):
				return fmt.Errorf("iloc: extent offset %d of item %d does not fit in %d bytes",
					e.Offset, item.ItemID, b.OffsetSize)
			case !fitsUintN(e.Length, b.LengthSize):
				return fmt.Errorf("iloc: extent length %d of item %d does not fit in %d bytes",
					e.Length, item.ItemID, b.LengthSize)
			case b.Version == 0 && e.ItemReferenceIndex != 0, !fitsUintN(e.ItemReferenceIndex, b.IndexSize):
				return fmt.Errorf("iloc: item reference index %d of item %d does not fit in %d bytes of version %d",
					e.ItemReferenceIndex, item.ItemID, b.IndexSize, b.Version)
			}
		}
	}
	return nil
}

// Info - write box-specific information
//
// Level 1 adds a line for each item and extent.
func (b *IlocBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, int(b.Version), b.Flags)
	bd.write(" - offsetSize: %d", b.OffsetSize)
	bd.write(" - lengthSize: %d", b.LengthSize)
	bd.write(" - baseOffsetSize: %d", b.BaseOffsetSize)
	if b.Version > 0 {
		bd.write(" - indexSize: %d", b.IndexSize)
	}
	bd.write(" - itemCount: %d", len(b.Items))
	if getInfoLevel(b, specificBoxLevels) > 0 {
		for i := range b.Items {
			item := &b.Items[i]
			msg := fmt.Sprintf(" - item[%d]: itemID=%d", i+1, item.ItemID)
			if b.Version > 0 {
				msg += fmt.Sprintf(" constructionMethod=%d", item.ConstructionMethod)
			}
			bd.write("%s dataReferenceIndex=%d baseOffset=%d extentCount=%d", msg, item.DataReferenceIndex,
				item.BaseOffset, len(item.Extents))
			for j, e := range item.Extents {
				msg = fmt.Sprintf("   - extent[%d]:", j+1)
				if b.Version > 0 && b.IndexSize > 0 {
					msg += fmt.Sprintf(" itemReferenceIndex=%d", e.ItemReferenceIndex)
				}
				bd.write("%s offset=%d length=%d", msg, e.Offset, e.Length)
			}
		}
	}
	return bd.err
}
