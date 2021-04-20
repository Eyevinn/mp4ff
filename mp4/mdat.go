package mp4

import (
	"io"
	"io/ioutil"
)

// MdatBox - Media Data Box (mdat)
// The mdat box contains media chunks/samples.
type MdatBox struct {
	StartPos        uint64
	Data            []byte
	decLazyDataSize uint64
	LargeSize       bool
}

const maxNormalPayloadSize = (1 << 32) - 1 - 8

// DecodeMdat - box-specific decode
func DecodeMdat(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	largeSize := hdr.hdrlen > boxHeaderSize
	return &MdatBox{startPos, data, uint64(len(data)), largeSize}, nil
}

func DecodeMdatLazily(hdr *boxHeader, startPos uint64) (Box, error) {
	largeSize := hdr.hdrlen > boxHeaderSize
	decLazyDataSize := hdr.size - uint64(hdr.hdrlen)
	return &MdatBox{startPos, nil, decLazyDataSize, largeSize}, nil
}

// Type - return box type
func (m *MdatBox) Type() string {
	return "mdat"
}

// Size - return calculated size, depending on largeSize set or not
func (m *MdatBox) Size() uint64 {
	if m.decLazyDataSize > 0 {
		return uint64(boxHeaderSize + m.decLazyDataSize)
	}
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
	bd := newInfoDumper(w, indent, m, -1, 0)
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
