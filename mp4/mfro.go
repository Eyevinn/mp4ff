package mp4

import (
	"io"
	"io/ioutil"
)

// MfroBox - Movie Fragment Random Access Offset Box (mfro)
// Contained in : MfraBox (mfra)
type MfroBox struct {
	Version    byte
	Flags      uint32
	ParentSize uint32
}

// DecodeMfro - box-specific decode
func DecodeMfro(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	s := NewSliceReader(data)
	versionAndFlags := s.ReadUint32()

	b := &MfroBox{
		Version:    byte(versionAndFlags >> 24),
		Flags:      versionAndFlags & flagsMask,
		ParentSize: s.ReadUint32(),
	}
	return b, nil
}

// Type - return box type
func (b *MfroBox) Type() string {
	return "mfro"
}

// Size - return calculated size
func (b *MfroBox) Size() uint64 {
	return uint64(boxHeaderSize + 4 + 4)
}

// Encode - write box to w
func (b *MfroBox) Encode(w io.Writer) error {
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	buf := makebuf(b)
	sw := NewSliceWriter(buf)
	versionAndFlags := (uint32(b.Version) << 24) + b.Flags
	sw.WriteUint32(versionAndFlags)
	sw.WriteUint32(b.ParentSize)
	_, err = w.Write(buf)
	return err
}

func (t *MfroBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, t, int(t.Version), t.Flags)
	bd.write(" - parentSize: %d", t.ParentSize)
	return bd.err
}
