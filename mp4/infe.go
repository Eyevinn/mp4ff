package mp4

import (
	"fmt"
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// InfeFlagHidden is the InfeBox flag of an item that is not intended to be part of the presentation.
const InfeFlagHidden = 0x000001

// InfeBox - Item Info Entry (infe), ISO/IEC 14496-12:2026 Sec. 8.11.6
//
// Contained in: Item Information Box (iinf)
//
// Version 2 and 3 have ItemType, with ItemID being 16 bits in version 2 and 32 bits in version 3.
// ContentType and ContentEncoding are then present for item type "mime", and ItemURIType for "uri ".
//
// Version 0 and 1 have ItemName, ContentType and ContentEncoding. Version 1 can end with an extension,
// with ExtensionType as its four-character type, such as "fdel", and ExtensionData as its bytes.
//
// ContentEncoding is optional. A decoded box without it is encoded without it, as long as
// ContentEncoding and ExtensionType are empty.
type InfeBox struct {
	Version             byte
	Flags               uint32
	ItemID              uint32
	ItemProtectionIndex uint16
	ItemType            string
	ItemName            string
	ContentType         string
	ContentEncoding     string
	ItemURIType         string
	ExtensionType       string
	ExtensionData       []byte
	noContentEncoding   bool // decoded without the optional content encoding
}

// DecodeInfe - box-specific decode
func DecodeInfe(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	sr := bits.NewFixedSliceReader(data)
	return DecodeInfeSR(hdr, startPos, sr)
}

// DecodeInfeSR - box-specific decode
func DecodeInfeSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	initPos := sr.GetPos()
	remaining := func() int {
		return hdr.payloadLen() - (sr.GetPos() - initPos)
	}
	versionAndFlags := sr.ReadUint32()
	b := InfeBox{
		Version: byte(versionAndFlags >> 24),
		Flags:   versionAndFlags & flagsMask,
	}
	if b.Version > 3 {
		return nil, fmt.Errorf("infe: version %d is not supported", b.Version)
	}
	if b.Version == 3 {
		b.ItemID = sr.ReadUint32()
	} else {
		b.ItemID = uint32(sr.ReadUint16())
	}
	b.ItemProtectionIndex = sr.ReadUint16()
	if b.Version >= 2 {
		b.ItemType = sr.ReadFixedLengthString(4)
	}
	b.ItemName = sr.ReadZeroTerminatedString(remaining())
	switch {
	case b.hasContentType():
		b.ContentType = sr.ReadZeroTerminatedString(remaining())
		if remaining() > 0 {
			b.ContentEncoding = sr.ReadZeroTerminatedString(remaining())
		} else {
			b.noContentEncoding = true
		}
	case b.ItemType == "uri ":
		b.ItemURIType = sr.ReadZeroTerminatedString(remaining())
	}
	if n := remaining(); b.Version == 1 && n > 0 {
		if n < 4 {
			return nil, fmt.Errorf("infe: %d bytes are too few for an extension type", n)
		}
		b.ExtensionType = sr.ReadFixedLengthString(4)
		b.ExtensionData = sr.ReadBytes(n - 4)
	}
	if err := sr.AccError(); err != nil {
		return nil, fmt.Errorf("infe: %w", err)
	}
	if n := remaining(); n != 0 {
		return nil, fmt.Errorf("infe: %d bytes after the last field", n)
	}
	return &b, nil
}

// hasContentType tells if the box has ContentType and the optional ContentEncoding.
func (b *InfeBox) hasContentType() bool {
	return b.Version < 2 || b.ItemType == "mime"
}

// hasContentEncoding tells if the optional content encoding is written.
func (b *InfeBox) hasContentEncoding() bool {
	return !b.noContentEncoding || b.ContentEncoding != "" || b.hasExtension()
}

// hasExtension tells if the box ends with an extension.
func (b *InfeBox) hasExtension() bool {
	return b.Version == 1 && b.ExtensionType != ""
}

// IsHidden tells if the item is not intended to be part of the presentation.
func (b *InfeBox) IsHidden() bool {
	return b.Version >= 2 && b.Flags&InfeFlagHidden != 0
}

// Type - box type
func (b *InfeBox) Type() string {
	return "infe"
}

// Size - calculated size of box
func (b *InfeBox) Size() uint64 {
	size := boxHeaderSize + 4 + 2 + 2 + len(b.ItemName) + 1
	if b.Version == 3 {
		size += 2 // 32-bit item ID
	}
	if b.Version >= 2 {
		size += 4
	}
	switch {
	case b.hasContentType():
		size += len(b.ContentType) + 1
		if b.hasContentEncoding() {
			size += len(b.ContentEncoding) + 1
		}
	case b.ItemType == "uri ":
		size += len(b.ItemURIType) + 1
	}
	if b.hasExtension() {
		size += 4 + len(b.ExtensionData)
	}
	return uint64(size)
}

// Encode - write box to w
func (b *InfeBox) Encode(w io.Writer) error {
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
// It returns an error for an unsupported version, an item ID that does not fit in the version,
// and an item type or extension type that is not four bytes.
func (b *InfeBox) EncodeSW(sw bits.SliceWriter) error {
	switch {
	case b.Version > 3:
		return fmt.Errorf("infe: version %d is not supported", b.Version)
	case b.Version != 3 && b.ItemID > 0xffff:
		return fmt.Errorf("infe: item ID %d needs version 3", b.ItemID)
	case b.Version >= 2 && len(b.ItemType) != 4:
		return fmt.Errorf("infe: item type %q is not four bytes", b.ItemType)
	case b.hasExtension() && len(b.ExtensionType) != 4:
		return fmt.Errorf("infe: extension type %q is not four bytes", b.ExtensionType)
	}
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	versionAndFlags := (uint32(b.Version) << 24) + b.Flags
	sw.WriteUint32(versionAndFlags)
	if b.Version == 3 {
		sw.WriteUint32(b.ItemID)
	} else {
		sw.WriteUint16(uint16(b.ItemID))
	}
	sw.WriteUint16(b.ItemProtectionIndex)
	if b.Version >= 2 {
		sw.WriteString(b.ItemType, false)
	}
	sw.WriteString(b.ItemName, true)
	switch {
	case b.hasContentType():
		sw.WriteString(b.ContentType, true)
		if b.hasContentEncoding() {
			sw.WriteString(b.ContentEncoding, true)
		}
	case b.ItemType == "uri ":
		sw.WriteString(b.ItemURIType, true)
	}
	if b.hasExtension() {
		sw.WriteString(b.ExtensionType, false)
		sw.WriteBytes(b.ExtensionData)
	}
	return sw.AccError()
}

// Info - write box-specific information
func (b *InfeBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, int(b.Version), b.Flags)
	bd.write(" - itemID: %d", b.ItemID)
	bd.write(" - itemProtectionIndex: %d", b.ItemProtectionIndex)
	if b.Version >= 2 {
		bd.write(" - itemType: %s", displayBoxType(b.ItemType))
	}
	bd.write(" - itemName: %q", b.ItemName)
	switch {
	case b.hasContentType():
		bd.write(" - contentType: %q", b.ContentType)
		if b.hasContentEncoding() {
			bd.write(" - contentEncoding: %q", b.ContentEncoding)
		}
	case b.ItemType == "uri ":
		bd.write(" - itemURIType: %q", b.ItemURIType)
	}
	if b.hasExtension() {
		bd.write(" - extensionType: %s", displayBoxType(b.ExtensionType))
		bd.write(" - extensionDataSize: %d", len(b.ExtensionData))
	}
	if b.IsHidden() {
		bd.write(" - hidden: true")
	}
	return bd.err
}
