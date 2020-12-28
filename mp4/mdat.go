package mp4

import (
	"io"
	"io/ioutil"
)

// MdatBox - Media Data Box (mdat)
// The mdat box contains media chunks/samples.
type MdatBox struct {
	StartPos  uint64
	Data      []byte
	LargeSize bool
}

const maxNormalPayloadSize = (1 << 32) - 1 - 8

// DecodeMdat - box-specific decode
func DecodeMdat(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	largeSize := hdr.hdrlen > boxHeaderSize
	return &MdatBox{startPos, data, largeSize}, nil
}

// Type - return box type
func (m *MdatBox) Type() string {
	return "mdat"
}

// Size - return calculated size, depending on largeSize set or not
func (m *MdatBox) Size() uint64 {
	if len(m.Data) > maxNormalPayloadSize {
		m.LargeSize = true
	}
	size := boxHeaderSize + len(m.Data)
	if m.LargeSize {
		size += 8
	}
	return uint64(size)
}

// AddSampleData -  a sample data to an mdat box
func (m *MdatBox) AddSampleData(s []byte) {
	m.Data = append(m.Data, s...)
}

// Encode - write box to w
func (m *MdatBox) Encode(w io.Writer) error {
	err := EncodeHeaderWithSize("mdat", m.Size(), m.LargeSize, w)
	if err != nil {
		return err
	}
	_, err = w.Write(m.Data)
	return err
}

func (m *MdatBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, m, -1)
	return bd.err
}

func (m *MdatBox) HeaderSize() uint64 {
	hSize := boxHeaderSize
	if m.LargeSize {
		hSize += largeSizeLen
	}
	return uint64(hSize)
}

// PayloadAbsoluteOffset - position of mdat payload start (works after header)
func (m *MdatBox) PayloadAbsoluteOffset() uint64 {
	return m.StartPos + m.HeaderSize()
}
