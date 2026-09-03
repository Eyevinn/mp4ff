package mp4

import (
	"errors"
	"fmt"
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// MdatBox - Media Data Box (mdat)
// The mdat box contains media chunks/samples.
//
// The payload is the concatenation of DataParts followed by Data, so the two
// can be combined freely: DataParts gathers output data without new
// allocations by referencing the caller's buffers, while Data is an open tail
// that AddSampleData appends (and thereby copies) into. Adding a part when a
// tail is present closes the tail into a part first, so the order in which
// data was added is always the order it is written.
type MdatBox struct {
	StartPos     uint64
	Data         []byte
	DataParts    [][]byte
	lazyDataSize uint64
	LargeSize    bool
}

const maxNormalPayloadSize = (1 << 32) - 1 - 8

// DecodeMdat - box-specific decode
func DecodeMdat(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	largeSize := hdr.Hdrlen > boxHeaderSize
	return &MdatBox{startPos, data, nil, 0, largeSize}, nil
}

// DecodeMdatSR decodes an mdat box
//
// Currently no content and no error is returned if not full length available.
// If not enough content, an accumulated error is stored in sr, though
func DecodeMdatSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	largeSize := hdr.Hdrlen > boxHeaderSize
	return &MdatBox{startPos, sr.ReadBytes(hdr.payloadLen()), nil, 0, largeSize}, nil
}

// IsLazy - is the mdat data handled lazily (with separate writer/reader).
func (m *MdatBox) IsLazy() bool {
	return m.lazyDataSize > 0
}

// DecodeMdatLazily - box-specific decode but Data is not in memory
func DecodeMdatLazily(hdr BoxHeader, startPos uint64) (Box, error) {
	largeSize := hdr.Hdrlen > boxHeaderSize
	decLazyDataSize := hdr.Size - uint64(hdr.Hdrlen)
	return &MdatBox{startPos, nil, nil, decLazyDataSize, largeSize}, nil
}

// SetLazyDataSize - set size of mdat lazy data so that the data can be written separately
// Don't put any data in m.Data in this mode.
func (m *MdatBox) SetLazyDataSize(newSize uint64) {
	m.lazyDataSize = newSize
}

// GetLazyDataSize - size of the box if filled with data
func (m *MdatBox) GetLazyDataSize() uint64 {
	return m.lazyDataSize
}

// Type - return box type
func (m *MdatBox) Type() string {
	return "mdat"
}

// payloadSize - the mdat payload size, in memory or lazy
func (m *MdatBox) payloadSize() uint64 {
	if m.lazyDataSize > 0 {
		return m.lazyDataSize
	}
	return m.DataLength()
}

// Size - return calculated size, depending on largeSize set or not.
// Sets the LargeSize flag if the payload requires a large header.
func (m *MdatBox) Size() uint64 {
	if m.payloadSize() > maxNormalPayloadSize {
		m.LargeSize = true
	}
	return m.HeaderSize() + m.payloadSize()
}

// AddSampleData - add sample data to an mdat box. The bytes are copied, so
// the caller may reuse s. It is appended after any data parts already added.
func (m *MdatBox) AddSampleData(s []byte) {
	m.Data = append(m.Data, s...)
}

// SetData - set the mdat data to given slice. No copying is done.
// The payload becomes exactly data, so any data parts are dropped.
func (m *MdatBox) SetData(data []byte) {
	m.Data = data
	m.DataParts = nil
	m.lazyDataSize = 0
}

// AddSampleDataPart - add a data part (for output). The slice is referenced,
// not copied, so the caller must keep it unmodified until the box has been
// encoded. Any data added with AddSampleData is closed into a part first, so
// that this part is written after it.
func (m *MdatBox) AddSampleDataPart(s []byte) {
	if len(m.DataParts) == 0 {
		m.DataParts = make([][]byte, 0, 8) // Reasonable size
	}
	if len(m.Data) != 0 {
		m.DataParts = append(m.DataParts, m.Data)
		m.Data = nil
	}
	m.DataParts = append(m.DataParts, s)
}

// Encode - write box to w. If m.lazyDataSize > 0, the mdat data needs to be written separately
func (m *MdatBox) Encode(w io.Writer) error {
	err := EncodeHeaderWithSize("mdat", m.Size(), m.LargeSize, w)
	if err != nil {
		return err
	}
	for _, dp := range m.DataParts {
		_, err = w.Write(dp)
		if err != nil {
			return err
		}
	}
	if len(m.Data) > 0 {
		_, err = w.Write(m.Data)
	}

	return err
}

// Encode - write box to sw. If m.lazyDataSize > 0, the mdat data needs to be written separately
func (m *MdatBox) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderWithSizeSW("mdat", m.Size(), m.LargeSize, sw)
	if err != nil {
		return err
	}
	for _, dp := range m.DataParts {
		sw.WriteBytes(dp)
	}
	if len(m.Data) > 0 {
		sw.WriteBytes(m.Data)
	}

	return sw.AccError()
}

// DataLength - length of data stored in box, as parts and as a monolithic tail
func (m *MdatBox) DataLength() uint64 {
	dataLength := len(m.Data)
	for i := range m.DataParts {
		dataLength += len(m.DataParts[i])
	}
	return uint64(dataLength)
}

// Info - write box-specific information
func (m *MdatBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, m, -1, 0)
	return bd.err
}

// HeaderSize - 8 or 16 (bytes) depending on whether largeSize is used.
// A large header is reported whenever the (possibly lazy) payload needs one,
// so the value is right even before Size() or Encode() has flipped the
// LargeSize flag. The flag is not mutated. This matters for
// Fragment.SetTrunDataOffsets, which reads the header size before encoding.
func (m *MdatBox) HeaderSize() uint64 {
	if m.LargeSize || m.payloadSize() > maxNormalPayloadSize {
		return boxHeaderSize + largeSizeLen
	}
	return boxHeaderSize
}

// PayloadAbsoluteOffset - position of mdat payload start (works after header)
func (m *MdatBox) PayloadAbsoluteOffset() uint64 {
	return m.StartPos + m.HeaderSize()
}

// ReadData reads Mdat data specified by the start and size.
// Input argument start is the position relative to the start of a file.
// The ReadSeeker is used for lazily loaded mdat case.
func (m *MdatBox) ReadData(start, size int64, rs io.ReadSeeker) ([]byte, error) {
	// The Mdat box was decoded lazily
	if m.lazyDataSize > 0 {
		if rs == nil {
			return nil, errors.New("lazy mdat mode - expects non-nil readseeker to read data")
		}

		_, err := rs.Seek(start, io.SeekStart)
		if err != nil {
			return nil, fmt.Errorf("lazy mdat mode - unable to seek to %d", start)
		}

		buf := make([]byte, size)
		_, err = io.ReadFull(rs, buf)
		if err != nil {
			return nil, err
		}
		return buf, nil
	}

	// Otherwise, all Mdat data is in memory, either as parts or as one big slice
	return m.dataRange(start, size)
}

// CopyData - copy data range from mdat to w.
// The ReadSeeker is used for lazily loaded mdat case.
func (m *MdatBox) CopyData(start, size int64, rs io.ReadSeeker, w io.Writer) (nrWritten int64, err error) {
	// The Mdat box was decoded lazily
	if m.lazyDataSize > 0 {
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
	data, err := m.dataRange(start, size)
	if err != nil {
		return 0, err
	}
	n, err := w.Write(data)
	return int64(n), err
}

// dataRange returns the in-memory mdat payload for the absolute file range
// [start, start+size). An error is returned if the range is not fully inside
// the mdat payload, so that a bad chunk offset or sample size in a corrupt
// file results in an error instead of a panic.
func (m *MdatBox) dataRange(start, size int64) ([]byte, error) {
	if start < 0 || size < 0 {
		return nil, fmt.Errorf("negative start %d or size %d", start, size)
	}
	if len(m.DataParts) > 0 {
		return nil, fmt.Errorf("extraction of range from dataParts not yet implemented")
	}
	payloadStart := m.PayloadAbsoluteOffset()
	if uint64(start) < payloadStart {
		return nil, fmt.Errorf("start %d is before mdat payload start %d", start, payloadStart)
	}
	offset := uint64(start) - payloadStart
	end := offset + uint64(size) // cannot overflow since both are non-negative int64
	if dataLen := m.DataLength(); end > dataLen {
		return nil, fmt.Errorf("range %d-%d is outside mdat data (size %d)", offset, end, dataLen)
	}
	return m.Data[offset:end], nil
}
