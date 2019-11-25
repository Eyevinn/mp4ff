package mp4

import (
	"fmt"
	"io"
	"io/ioutil"
)

// MfhdBox - Media Fragment Header Box (mfhd)
//
// Contained in : Movie Fragment box (moof))
type MfhdBox struct {
	Version        byte
	Flags          uint32
	SequenceNumber uint32
}

// DecodeMfhd - box-specific decode
func DecodeMfhd(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	s := NewSliceReader(data)
	versionAndFlags := s.ReadUint32()
	version := byte(versionAndFlags >> 24)
	flags := versionAndFlags & flagsMask
	sequenceNumber := s.ReadUint32()
	return &MfhdBox{
		Version:        version,
		Flags:          flags,
		SequenceNumber: sequenceNumber,
	}, nil
}

// CreateMfhd - create an MfhdBox
func CreateMfhd(sequenceNumber uint32) *MfhdBox {
	return &MfhdBox{
		Version:        0,
		Flags:          0,
		SequenceNumber: sequenceNumber,
	}
}

// Type - box type
func (m *MfhdBox) Type() string {
	return "mfhd"
}

// Size - calculated size of box
func (m *MfhdBox) Size() uint64 {
	return boxHeaderSize + 8
}

// Dump - print box info
func (m *MfhdBox) Dump() {
	fmt.Printf("Media Fragment Header:\n Sequence Number: %d\n", m.SequenceNumber)
}

// Encode - write box to w
func (m *MfhdBox) Encode(w io.Writer) error {
	err := EncodeHeader(m, w)
	if err != nil {
		return err
	}
	buf := makebuf(m)
	sw := NewSliceWriter(buf)
	versionAndFlags := (uint32(m.Version) << 24) + m.Flags
	sw.WriteUint32(versionAndFlags)
	sw.WriteUint32(m.SequenceNumber)
	_, err = w.Write(buf)
	return err
}
