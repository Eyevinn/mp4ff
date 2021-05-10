package mp4

import (
	"errors"
	"fmt"
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
	return &MdatBox{startPos, data, 0, largeSize}, nil
}

// DecodeMdatLazily - box-specific decode but Data is not in memory
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

// ReadData reads Mdat data specified by the start and size.
// Input argument start is the postion relative to the start of a file.
// The ReadSeeker is used for lazily loaded mdat case.
func (m *MdatBox) ReadData(start, size int64, rs io.ReadSeeker) ([]byte, error) {
	// The Mdat box was decoded lazily
	if m.decLazyDataSize > 0 {
		if rs == nil {
			return nil, errors.New("lazy mdat mode - expects non-nil readseeker to read data")
		}

		_, err := rs.Seek(start, io.SeekStart)
		if err != nil {
			return nil, fmt.Errorf("lazy mdat mode - unable to seek to %d", start)
		}

		buf := make([]byte, size)
		n, err := rs.Read(buf)
		if err != nil {
			return nil, err
		}
		if int64(n) != size {
			return nil, fmt.Errorf("lazy mdat mode - expects to read %d bytes, but only read %d bytes", size, n)
		}
		return buf, nil
	}

	// Otherwise, all Mdat data is in memory
	mdatPayloadStart := m.PayloadAbsoluteOffset()
	offsetInMdatData := uint64(start) - mdatPayloadStart
	endIndexInMdatData := offsetInMdatData + uint64(size)

	// validate if indexes are valid to avoid panics
	if offsetInMdatData >= uint64(len(m.Data)) || endIndexInMdatData >= uint64(len(m.Data)) {
		return nil, fmt.Errorf("normal mdat mode - invalid range provided")
	}
	return m.Data[offsetInMdatData : offsetInMdatData+uint64(size)], nil

}

// CopyData - copy data range from mdat to w.
// The ReadSeeker is used for lazily loaded mdat case.
func (m *MdatBox) CopyData(start, size int64, rs io.ReadSeeker, w io.Writer) (nrWritten int64, err error) {
	// The Mdat box was decoded lazily
	if m.decLazyDataSize > 0 {
		if rs == nil {
			return 0, errors.New("lazy mdat mode - expects non-nil readseeker to read data")
		}

		_, err := rs.Seek(start, io.SeekStart)
		if err != nil {
			return 0, fmt.Errorf("lazy mdat mode - unable to seek to %d", start)
		}
		return io.CopyN(w, rs, size)
	}

	// Otherwise, all Mdat data is in memory
	mdatPayloadStart := m.PayloadAbsoluteOffset()
	offsetInMdatData := uint64(start) - mdatPayloadStart
	endIndexInMdatData := offsetInMdatData + uint64(size)

	// validate if indexes are valid to avoid panics
	if uint64(start) < mdatPayloadStart || endIndexInMdatData >= uint64(len(m.Data)) {
		return 0, fmt.Errorf("normal mdat mode - invalid range provided")
	}
	var n int
	n, err = w.Write(m.Data[offsetInMdatData : offsetInMdatData+uint64(size)])
	return int64(n), err

}
