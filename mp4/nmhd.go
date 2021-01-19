package mp4

import (
	"io"
	"io/ioutil"
)

// NmhdBox - Null Media Header Box (nmhd - often used instead ofsthd for subtitle tracks)
type NmhdBox struct {
	Version byte
	Flags   uint32
}

// DecodeNmhd - box-specific decode
func DecodeNmhd(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	s := NewSliceReader(data)
	versionAndFlags := s.ReadUint32()
	sb := &NmhdBox{
		Version: byte(versionAndFlags >> 24),
		Flags:   versionAndFlags & flagsMask,
	}
	return sb, nil
}

// Type - box-specific type
func (b *NmhdBox) Type() string {
	return "nmhd"
}

// Size - calculated size of box
func (b *NmhdBox) Size() uint64 {
	return boxHeaderSize + 4 // FullBox
}

// Encode - write box to w
func (b *NmhdBox) Encode(w io.Writer) error {
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

func (b *NmhdBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, int(b.Version), b.Flags)
	return bd.err
}
