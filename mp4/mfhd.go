package mp4

import (
	"fmt"
	"io"
	"io/ioutil"
)

// Media Fragment Header Box (mfhd)
//
// Contained in : Movie Fragment box (moof))

type MfhdBox struct {
	Version        byte
	Flags          uint32
	SequenceNumber uint32
}

func DecodeMfhd(r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	s := NewSliceReader(data)
	versionAndFlags := s.ReadUint32()
	version := byte(versionAndFlags >> 24)
	flags := versionAndFlags & 0xffffff
	sequenceNumber := s.ReadUint32()
	return &MfhdBox{
		Version:        version,
		Flags:          flags,
		SequenceNumber: sequenceNumber,
	}, nil
}

func (m *MfhdBox) Type() string {
	return "mdhd"
}

func (m *MfhdBox) Size() int {
	return BoxHeaderSize + 8
}

func (m *MfhdBox) Dump() {
	fmt.Printf("Media Fragment Header:\n Sequence Number: %d\n", m.SequenceNumber)

}

func (m *MfhdBox) Encode(w io.Writer) error {
	err := EncodeHeader(m, w)
	if err != nil {
		return err
	}
	buf := makebuf(m)
	bw := NewBufferWrapper(buf)
	versionAndFlags := (uint32(m.Version) << 24) + m.Flags
	bw.WriteUint32(versionAndFlags)
	bw.WriteUint32(m.SequenceNumber)
	_, err = w.Write(buf)
	return err
}
