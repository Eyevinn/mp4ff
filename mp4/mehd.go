package mp4

import (
	"io"
	"io/ioutil"
)

// MehdBox - Movie Extends Header Box
// Optional, provides overall duration of a fragmented movie
type MehdBox struct {
	Version          byte
	Flags            uint32
	FragmentDuration int64
}

// DecodeMehd - box-specific decode
func DecodeMehd(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	s := NewSliceReader(data)
	versionAndFlags := s.ReadUint32()
	version := byte(versionAndFlags >> 24)
	b := &MehdBox{
		Version: version,
		Flags:   versionAndFlags & flagsMask,
	}
	if version == 0 {
		b.FragmentDuration = int64(s.ReadInt32())
	} else {
		b.FragmentDuration = s.ReadInt64()
	}
	return b, nil
}

// Type - return box type
func (b *MehdBox) Type() string {
	return "mehd"
}

// Size - return calculated size
func (b *MehdBox) Size() uint64 {
	size := uint64(boxHeaderSize) + 4
	if b.Version == 0 {
		size += 4
	} else {
		size += 8
	}
	return size
}

// Encode - write box to w
func (b *MehdBox) Encode(w io.Writer) error {
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	buf := makebuf(b)
	sw := NewSliceWriter(buf)
	versionAndFlags := (uint32(b.Version) << 24) + b.Flags
	sw.WriteUint32(versionAndFlags)
	if b.Version == 0 {
		sw.WriteUint32(uint32(b.FragmentDuration))

	} else {
		sw.WriteUint64(uint64(b.FragmentDuration))
	}
	_, err = w.Write(buf)
	return err
}

// Dump - write MehBox details.
func (b *MehdBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) (err error) {
	bd := newInfoDumper(w, indent, b, int(b.Version), b.Flags)
	bd.write(" - fragmentDuration: %d", b.FragmentDuration)
	return bd.err
}
