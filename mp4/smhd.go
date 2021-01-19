package mp4

import (
	"io"
	"io/ioutil"
)

// SmhdBox - Sound Media Header Box (smhd - mandatory for sound tracks)
//
// Contained in : Media Information Box (minf)
//
type SmhdBox struct {
	Version byte
	Flags   uint32
	Balance uint16 // should be int16
}

// CreateSmhd - Create Sound Media Header Box (all is zero)
func CreateSmhd() *SmhdBox {
	return &SmhdBox{}
}

// DecodeSmhd - box-specific decode
func DecodeSmhd(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	s := NewSliceReader(data)
	versionAndFlags := s.ReadUint32()
	return &SmhdBox{
		Version: byte(versionAndFlags >> 24),
		Flags:   versionAndFlags & flagsMask,
		Balance: s.ReadUint16(),
	}, nil
}

// Type - box type
func (b *SmhdBox) Type() string {
	return "smhd"
}

// Size - calculated size of box
func (b *SmhdBox) Size() uint64 {
	return boxHeaderSize + 8
}

// Encode - write box to w
func (b *SmhdBox) Encode(w io.Writer) error {
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	buf := makebuf(b)
	sw := NewSliceWriter(buf)
	versionAndFlags := (uint32(b.Version) << 24) + b.Flags
	sw.WriteUint32(versionAndFlags)
	sw.WriteUint16(b.Balance)
	sw.WriteUint16(0) // Reserved
	_, err = w.Write(buf)
	return err
}

func (b *SmhdBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, int(b.Version), b.Flags)
	return bd.err
}
