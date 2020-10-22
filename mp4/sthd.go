package mp4

import (
	"fmt"
	"io"
	"io/ioutil"
)

// SthdBox - Subtitle Media Header Box (sthd - for subtitle tracks)
type SthdBox struct {
	Version byte
	Flags   uint32
}

// DecodeSthd - box-specific decode
func DecodeSthd(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	s := NewSliceReader(data)
	versionAndFlags := s.ReadUint32()
	sb := &SthdBox{
		Version: byte(versionAndFlags >> 24),
		Flags:   versionAndFlags & flagsMask,
	}
	return sb, nil
}

// Type - box-specific type
func (b *SthdBox) Type() string {
	return "sthd"
}

// Size - calculated size of box
func (b *SthdBox) Size() uint64 {
	return boxHeaderSize + 4 // FullBox
}

// Encode - write box to w
func (b *SthdBox) Encode(w io.Writer) error {
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	buf := makebuf(b)
	sw := NewSliceWriter(buf)
	versionAndFlags := (uint32(b.Version) << 24) + b.Flags
	sw.WriteUint32(versionAndFlags)
	_, err = w.Write(buf)
	return err
}

func (b *SthdBox) Dump(w io.Writer, indent, indentStep string) error {
	_, err := fmt.Fprintf(w, "%s%s size=%d\n", indent, b.Type(), b.Size())
	return err
}
