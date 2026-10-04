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
	ownsData     bool // Data was allocated by AddSampleData, so Fragment.Reset may reuse it
}

const maxNormalPayloadSize = (1 << 32) - 1 - 8

// DecodeMdat - box-specific decode
func DecodeMdat(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	largeSize := hdr.Hdrlen > boxHeaderSize
	return &MdatBox{StartPos: startPos, Data: data, LargeSize: largeSize}, nil
}

// DecodeMdatSR decodes an mdat box
//
// Currently no content and no error is returned if not full length available.
// If not enough content, an accumulated error is stored in sr, though
func DecodeMdatSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	largeSize := hdr.Hdrlen > boxHeaderSize
	return &MdatBox{StartPos: startPos, Data: sr.ReadBytes(hdr.payloadLen()), LargeSize: largeSize}, nil
}

// IsLazy - is the mdat data handled lazily (with separate writer/reader).
func (m *MdatBox) IsLazy() bool {
	return m.lazyDataSize > 0
}

// DecodeMdatLazily - box-specific decode but Data is not in memory
func DecodeMdatLazily(hdr BoxHeader, startPos uint64) (Box, error) {
	largeSize := hdr.Hdrlen > boxHeaderSize
	decLazyDataSize := hdr.Size - uint64(hdr.Hdrlen)
	return &MdatBox{StartPos: startPos, lazyDataSize: decLazyDataSize, LargeSize: largeSize}, nil
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
	if m.Data == nil {
		m.ownsData = true
	}
	m.Data = append(m.Data, s...)
}

// SetData - set the mdat data to given slice. No copying is done.
// The payload becomes exactly data, so any data parts are dropped.
func (m *MdatBox) SetData(data []byte) {
	m.Data = data
	m.DataParts = nil
	m.lazyDataSize = 0
	m.ownsData = false
}

// AddSampleDataPart - add a data part (for output). The slice is referenced,
// not copied, so the caller must keep it unmodified until the box has been
// encoded, and EncryptFragment encrypts it in place. Any data added with
// AddSampleData is closed into a part first, so that this part is written
// after it.
func (m *MdatBox) AddSampleDataPart(s []byte) {
	if cap(m.DataParts) == 0 {
		m.DataParts = make([][]byte, 0, 8) // Reasonable size
	}
	if len(m.Data) != 0 {
		m.DataParts = append(m.DataParts, m.Data)
		m.Data = nil
		m.ownsData = false
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

// EncodeSW - write box to sw. If m.lazyDataSize > 0, the mdat data needs to be written separately
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

// Payload returns the in-memory payload, DataParts followed by Data, as one slice. When at most one of them is
// non-empty, as for a decoded mdat, one filled with AddSampleData, or one whose samples AddFullSamples added as a
// single data part, that slice is returned without copying, so changing it changes the payload. Otherwise the parts
// are joined into a new slice. A lazily decoded mdat has no payload in memory, and gives nil.
func (m *MdatBox) Payload() []byte {
	if m.IsLazy() {
		return nil
	}
	var payload []byte
	nrNonEmpty := 0
	for i := 0; i <= len(m.DataParts); i++ {
		if p := m.part(i); len(p) > 0 {
			payload = p
			nrNonEmpty++
		}
	}
	if nrNonEmpty <= 1 {
		return payload
	}
	payload = make([]byte, 0, m.DataLength())
	for i := 0; i <= len(m.DataParts); i++ {
		payload = append(payload, m.part(i)...)
	}
	return payload
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
// A nil MdatBox, as for a file without mdat box, gives an error.
func (m *MdatBox) ReadData(start, size int64, rs io.ReadSeeker) ([]byte, error) {
	if m == nil {
		return nil, errors.New("no mdat box")
	}
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
// A nil MdatBox, as for a file without mdat box, gives an error.
func (m *MdatBox) CopyData(start, size int64, rs io.ReadSeeker, w io.Writer) (nrWritten int64, err error) {
	if m == nil {
		return 0, errors.New("no mdat box")
	}
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
// file results in an error instead of a panic. The range must also be inside
// one of DataParts or Data, since it is returned as a view.
func (m *MdatBox) dataRange(start, size int64) ([]byte, error) {
	if start < 0 || size < 0 {
		return nil, fmt.Errorf("negative start %d or size %d", start, size)
	}
	payloadStart := m.PayloadAbsoluteOffset()
	if uint64(start) < payloadStart {
		return nil, fmt.Errorf("start %d is before mdat payload start %d", start, payloadStart)
	}
	off := uint64(start) - payloadStart
	end := off + uint64(size)  // cannot overflow since both are non-negative int64
	if len(m.DataParts) == 0 { // as in a decoded mdat, so there is no part to look up
		if end > uint64(len(m.Data)) {
			return nil, m.outsideDataErr(off, end)
		}
		return m.Data[off:end], nil
	}
	c := payloadCursor{mdat: m}
	rest, err := c.restAt(off, uint64(size))
	if err != nil {
		return nil, err
	}
	return rest[:size], nil
}

// errSpansDataParts is returned for a payload range that starts in one data part and ends in another, so that
// it cannot be returned as a view.
var errSpansDataParts = errors.New("spans more than one mdat data part")

// part returns part i of the in-memory payload: DataParts[i], or the tail Data for i == len(DataParts).
func (m *MdatBox) part(i int) []byte {
	if i < len(m.DataParts) {
		return m.DataParts[i]
	}
	return m.Data
}

// payloadCursor finds ranges of the in-memory mdat payload, DataParts followed by Data, in those slices.
// It continues from the part where the previous range was found, so finding the ranges of consecutive samples
// takes time linear in their number, however many parts there are.
//
// A loop over consecutive samples gets rest, the payload from a sample to the end of its part, from next, and
// slices that sample and the following ones inside rest from it, so that a part is looked up once and not for
// every sample:
//
//	c := mdat.cursorAt(offset)
//	var pos uint64 // position of sample i in rest
//	for i := 0; i < len(samples); {
//		rest, err := c.next(pos, uint64(samples[i].Size))
//		if err != nil { ... }
//		for pos = 0; i < len(samples); i++ {
//			end := pos + uint64(samples[i].Size)
//			if end > uint64(len(rest)) {
//				break
//			}
//			data := rest[pos:end]
//			pos = end
//		}
//	}
//
// The inner loop has no call, which keeps its variables in registers. Since next returns a rest that holds
// sample i, every round of the outer loop takes at least one sample. The sample data are not capped, so the
// data of adjacent samples are adjacent slices.
type payloadCursor struct {
	mdat  *MdatBox
	part  int    // index of the current part, as for MdatBox.part
	start uint64 // payload offset of the current part
	off   uint64 // payload offset of the rest that next returned last, or of the first range
}

// cursorAt returns a payloadCursor for consecutive ranges from payload offset off.
func (m *MdatBox) cursorAt(off uint64) payloadCursor {
	return payloadCursor{mdat: m, off: off}
}

// next returns the payload from the next range, of size bytes, to the end of the part that holds it.
// The next range starts consumed bytes after the start of the rest that next returned last.
func (c *payloadCursor) next(consumed, size uint64) ([]byte, error) {
	off := c.off + consumed
	rest, err := c.restAt(off, size)
	if err != nil {
		return nil, err
	}
	c.off = off
	return rest, nil
}

// restAt returns the payload from off to the end of the part that holds the range [off, off+size).
// A range that is outside the payload, or that spans more than one part (wrapping errSpansDataParts), gives an
// error, since it cannot be sliced from one part.
func (c *payloadCursor) restAt(off, size uint64) ([]byte, error) {
	m := c.mdat
	if off < c.start {
		c.part, c.start = 0, 0
	}
	end := off + size
	for ; c.part <= len(m.DataParts); c.part++ {
		p := m.part(c.part)
		partEnd := c.start + uint64(len(p))
		if off < partEnd || (off == partEnd && size == 0) {
			if end <= partEnd {
				return p[off-c.start:], nil
			}
			if end <= m.DataLength() {
				return nil, fmt.Errorf("range %d-%d %w", off, end, errSpansDataParts)
			}
			break
		}
		c.start = partEnd
	}
	return nil, m.outsideDataErr(off, end)
}

// payloadData returns the in-memory payload range [off, off+size): a view if the range is inside one part,
// and otherwise a copy gathered from the parts it spans.
func (m *MdatBox) payloadData(off, size uint64) ([]byte, error) {
	c := payloadCursor{mdat: m}
	rest, err := c.restAt(off, size)
	if err == nil {
		return rest[:size], nil
	}
	if !errors.Is(err, errSpansDataParts) {
		return nil, err
	}
	data := make([]byte, 0, size)
	end := off + size
	var partStart uint64
	for i := 0; i <= len(m.DataParts) && partStart < end; i++ {
		p := m.part(i)
		partEnd := partStart + uint64(len(p))
		if partEnd > off {
			lo, hi := uint64(0), uint64(len(p))
			if off > partStart {
				lo = off - partStart
			}
			if end < partEnd {
				hi = end - partStart
			}
			data = append(data, p[lo:hi]...)
		}
		partStart = partEnd
	}
	return data, nil
}

// outsideDataErr reports that the payload range [off, end) is not inside the in-memory payload.
func (m *MdatBox) outsideDataErr(off, end uint64) error {
	if m.IsLazy() {
		return fmt.Errorf("range %d-%d: mdat data is not in memory (lazy mdat)", off, end)
	}
	return fmt.Errorf("range %d-%d is outside mdat data (size %d)", off, end, m.DataLength())
}
