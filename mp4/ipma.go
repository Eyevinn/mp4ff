package mp4

import (
	"fmt"
	"io"
	"strings"

	"github.com/Eyevinn/mp4ff/bits"
)

// IpmaFlagLargeIndex is the IpmaBox flag for 15-bit property indexes instead of 7-bit ones.
const IpmaFlagLargeIndex = 0x000001

// IpmaBox - Item Property Association Box (ipma), ISO/IEC 14496-12:2026 Sec. 8.11.14
//
// Contained in: Item Properties Box (iprp)
//
// It associates items with properties in the ipco box of the same iprp box.
// Item IDs are 16 bits in version 0 and 32 bits in other versions.
// Property indexes are 7 bits, or 15 bits with the flag IpmaFlagLargeIndex.
type IpmaBox struct {
	Version byte
	Flags   uint32
	Entries []IpmaEntry
}

// IpmaEntry is the ordered list of properties associated with one item.
type IpmaEntry struct {
	ItemID       uint32
	Associations []IpmaAssociation
}

// IpmaAssociation associates an item with the property at the one-based PropertyIndex among the
// children of ipco. Index 0 means no property. Essential tells that the property must be understood
// to process the item.
type IpmaAssociation struct {
	Essential     bool
	PropertyIndex uint16
}

// DecodeIpma - box-specific decode
func DecodeIpma(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	sr := bits.NewFixedSliceReader(data)
	return DecodeIpmaSR(hdr, startPos, sr)
}

// DecodeIpmaSR - box-specific decode
func DecodeIpmaSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	initPos := sr.GetPos()
	remaining := func() int {
		return hdr.payloadLen() - (sr.GetPos() - initPos)
	}
	versionAndFlags := sr.ReadUint32()
	b := IpmaBox{
		Version: byte(versionAndFlags >> 24),
		Flags:   versionAndFlags & flagsMask,
	}
	entryCount := sr.ReadUint32()
	if err := sr.AccError(); err != nil {
		return nil, fmt.Errorf("ipma: %w", err)
	}
	if uint64(entryCount)*uint64(b.itemIDSize()+1) > uint64(remaining()) {
		return nil, fmt.Errorf("ipma: %d entries do not fit in %d bytes", entryCount, remaining())
	}
	b.Entries = make([]IpmaEntry, entryCount)
	for i := range b.Entries {
		entry := &b.Entries[i]
		if b.Version == 0 {
			entry.ItemID = uint32(sr.ReadUint16())
		} else {
			entry.ItemID = sr.ReadUint32()
		}
		associationCount := int(sr.ReadUint8())
		if err := sr.AccError(); err != nil {
			return nil, fmt.Errorf("ipma: %w", err)
		}
		if associationCount*b.indexSize() > remaining() {
			return nil, fmt.Errorf("ipma: %d associations of item %d do not fit in %d bytes",
				associationCount, entry.ItemID, remaining())
		}
		entry.Associations = make([]IpmaAssociation, associationCount)
		for j := range entry.Associations {
			if b.hasLargeIndex() {
				v := sr.ReadUint16()
				entry.Associations[j] = IpmaAssociation{Essential: v&0x8000 != 0, PropertyIndex: v & 0x7fff}
			} else {
				v := sr.ReadUint8()
				entry.Associations[j] = IpmaAssociation{Essential: v&0x80 != 0, PropertyIndex: uint16(v & 0x7f)}
			}
		}
	}
	if err := sr.AccError(); err != nil {
		return nil, fmt.Errorf("ipma: %w", err)
	}
	if n := remaining(); n != 0 {
		return nil, fmt.Errorf("ipma: %d bytes after the last entry", n)
	}
	return &b, nil
}

// hasLargeIndex tells if the property indexes are 15 bits instead of 7.
func (b *IpmaBox) hasLargeIndex() bool {
	return b.Flags&IpmaFlagLargeIndex != 0
}

// indexSize returns the size in bytes of an association.
func (b *IpmaBox) indexSize() int {
	if b.hasLargeIndex() {
		return 2
	}
	return 1
}

// itemIDSize returns the size in bytes of an item ID.
func (b *IpmaBox) itemIDSize() int {
	if b.Version == 0 {
		return 2
	}
	return 4
}

// Type - box type
func (b *IpmaBox) Type() string {
	return "ipma"
}

// Size - calculated size of box
func (b *IpmaBox) Size() uint64 {
	size := uint64(boxHeaderSize + 4 + 4)
	for i := range b.Entries {
		size += uint64(b.itemIDSize() + 1 + len(b.Entries[i].Associations)*b.indexSize())
	}
	return size
}

// Encode - write box to w
func (b *IpmaBox) Encode(w io.Writer) error {
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
// It returns an error for an item ID or a property index that does not fit given the version and
// the flags, and for more than 255 associations of an item.
func (b *IpmaBox) EncodeSW(sw bits.SliceWriter) error {
	maxIndex := uint16(0x7f)
	if b.hasLargeIndex() {
		maxIndex = 0x7fff
	}
	for i := range b.Entries {
		entry := &b.Entries[i]
		if b.Version == 0 && entry.ItemID > 0xffff {
			return fmt.Errorf("ipma: item ID %d needs version 1", entry.ItemID)
		}
		if len(entry.Associations) > 0xff {
			return fmt.Errorf("ipma: item %d has %d associations, more than 255", entry.ItemID, len(entry.Associations))
		}
		for _, a := range entry.Associations {
			if a.PropertyIndex > maxIndex {
				return fmt.Errorf("ipma: property index %d of item %d is more than %d",
					a.PropertyIndex, entry.ItemID, maxIndex)
			}
		}
	}
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	versionAndFlags := (uint32(b.Version) << 24) + b.Flags
	sw.WriteUint32(versionAndFlags)
	sw.WriteUint32(uint32(len(b.Entries)))
	for i := range b.Entries {
		entry := &b.Entries[i]
		if b.Version == 0 {
			sw.WriteUint16(uint16(entry.ItemID))
		} else {
			sw.WriteUint32(entry.ItemID)
		}
		sw.WriteUint8(byte(len(entry.Associations)))
		for _, a := range entry.Associations {
			var essential uint16
			if a.Essential {
				essential = 1
			}
			if b.hasLargeIndex() {
				sw.WriteUint16(essential<<15 | a.PropertyIndex)
			} else {
				sw.WriteUint8(byte(essential<<7 | a.PropertyIndex))
			}
		}
	}
	return sw.AccError()
}

// Info - write box-specific information
//
// Level 1 adds a line for each item with its property indexes.
func (b *IpmaBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, int(b.Version), b.Flags)
	bd.write(" - entryCount: %d", len(b.Entries))
	if getInfoLevel(b, specificBoxLevels) > 0 {
		for i := range b.Entries {
			var sb strings.Builder
			for j, a := range b.Entries[i].Associations {
				if j > 0 {
					sb.WriteByte(' ')
				}
				fmt.Fprintf(&sb, "%d", a.PropertyIndex)
				if a.Essential {
					sb.WriteString("(essential)")
				}
			}
			bd.write(" - entry[%d]: itemID=%d propertyIndexes=[%s]", i+1, b.Entries[i].ItemID, sb.String())
		}
	}
	return bd.err
}
