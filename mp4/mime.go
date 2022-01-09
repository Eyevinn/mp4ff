package mp4

import (
	"encoding/binary"
	"io"
)

// MimeBox - MIME Box as defined in ISO/IEC 14496-12 2020 Section 12.3.3.2
type MimeBox struct {
	Version              byte
	Flags                uint32
	ContentType          string
	LacksZeroTermination bool // Handle non-compliant case as well
}

// DecodeMime - box-specific decode
func DecodeMime(hdr boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	versionAndFlags := binary.BigEndian.Uint32(data[0:4])
	b := MimeBox{
		Version: byte(versionAndFlags >> 24),
		Flags:   versionAndFlags & flagsMask,
	}
	if data[len(data)-1] == 0 {
		b.ContentType = string(data[4 : len(data)-1])
	} else {
		b.ContentType = string(data[4:])
		b.LacksZeroTermination = true
	}
	return &b, nil
}

// Type - box type
func (b *MimeBox) Type() string {
	return "mime"
}

// Size - calculated size of box
func (b *MimeBox) Size() uint64 {
	size := uint64(boxHeaderSize + 4 + len(b.ContentType) + 1)
	if b.LacksZeroTermination {
		size--
	}
	return size
}

// Encode - write box to w
func (b *MimeBox) Encode(w io.Writer) error {
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	buf := makebuf(b)
	sw := NewSliceWriter(buf)
	versionAndFlags := (uint32(b.Version) << 24) + b.Flags
	sw.WriteUint32(versionAndFlags)
	sw.WriteString(b.ContentType, !b.LacksZeroTermination)
	_, err = w.Write(buf)
	return err
}

// Info - write specific box information
func (b *MimeBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, int(b.Version), b.Flags)
	bd.write(" - contentType: %s", b.ContentType)
	return bd.err
}
