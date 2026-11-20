package mp4

import (
	"fmt"
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// PitmBox - Primary Item Box (pitm), ISO/IEC 14496-12:2026 Sec. 8.11.4
//
// Contained in: Meta Box (meta)
//
// ItemID is 16 bits in version 0 and 32 bits in other versions.
type PitmBox struct {
	Version byte
	Flags   uint32
	ItemID  uint32
}

// DecodePitm - box-specific decode
func DecodePitm(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	sr := bits.NewFixedSliceReader(data)
	return DecodePitmSR(hdr, startPos, sr)
}

// DecodePitmSR - box-specific decode
func DecodePitmSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	versionAndFlags := sr.ReadUint32()
	b := PitmBox{
		Version: byte(versionAndFlags >> 24),
		Flags:   versionAndFlags & flagsMask,
	}
	if b.Version == 0 {
		b.ItemID = uint32(sr.ReadUint16())
	} else {
		b.ItemID = sr.ReadUint32()
	}
	return &b, sr.AccError()
}

// Type - box type
func (b *PitmBox) Type() string {
	return "pitm"
}

// Size - calculated size of box
func (b *PitmBox) Size() uint64 {
	if b.Version == 0 {
		return uint64(boxHeaderSize + 4 + 2)
	}
	return uint64(boxHeaderSize + 4 + 4)
}

// Encode - write box to w
func (b *PitmBox) Encode(w io.Writer) error {
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
// It returns an error for an item ID that needs more than the 16 bits of version 0.
func (b *PitmBox) EncodeSW(sw bits.SliceWriter) error {
	if b.Version == 0 && b.ItemID > 0xffff {
		return fmt.Errorf("pitm: item ID %d needs version 1", b.ItemID)
	}
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	versionAndFlags := (uint32(b.Version) << 24) + b.Flags
	sw.WriteUint32(versionAndFlags)
	if b.Version == 0 {
		sw.WriteUint16(uint16(b.ItemID))
	} else {
		sw.WriteUint32(b.ItemID)
	}
	return sw.AccError()
}

// Info - write box-specific information
func (b *PitmBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, int(b.Version), b.Flags)
	bd.write(" - itemID: %d", b.ItemID)
	return bd.err
}
