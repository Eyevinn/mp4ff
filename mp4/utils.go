package mp4

import "encoding/binary"

// SliceReader - read integers from a slice
type SliceReader struct {
	slice []byte
	pos   int
}

// NewSliceReader - create a new slice reader reading from data
func NewSliceReader(data []byte) *SliceReader {
	return &SliceReader{
		slice: data,
		pos:   0,
	}
}

// ReadUint32 - read uint32 from slice
func (s *SliceReader) ReadUint32() uint32 {
	res := binary.BigEndian.Uint32(s.slice[s.pos : s.pos+4])
	s.pos += 4
	return res
}

// ReadInt32 - read int32 from slice
func (s *SliceReader) ReadInt32() int32 {
	res := binary.BigEndian.Uint32(s.slice[s.pos : s.pos+4])
	s.pos += 4
	return int32(res)
}

// ReadUint64 - read uint64 from slice
func (s *SliceReader) ReadUint64() uint64 {
	res := binary.BigEndian.Uint64(s.slice[s.pos : s.pos+8])
	s.pos += 8
	return res
}

// ReadInt64 - read int64 from slice
func (s *SliceReader) ReadInt64() int64 {
	res := binary.BigEndian.Uint64(s.slice[s.pos : s.pos+8])
	s.pos += 8
	return int64(res)
}

// SetPos - set read position is slice
func (s *SliceReader) SetPos(pos int) {
	s.pos = pos
}

// GetPos - get read position is slice
func (s *SliceReader) GetPos() int {
	return s.pos
}

// Length - get length of slice
func (s *SliceReader) Length() int {
	return len(s.slice)
}

// SliceWriter - write numbers to a []byte slice
type SliceWriter struct {
	buf []byte
	pos int
}

// NewSliceWriter - create writer around slice
func NewSliceWriter(data []byte) *SliceWriter {
	return &SliceWriter{
		buf: data,
		pos: 0,
	}
}

// WriteUint32 - write uint32 to slice
func (b *SliceWriter) WriteUint32(n uint32) {
	binary.BigEndian.PutUint32(b.buf[b.pos:], n)
	b.pos += 4
}

// WriteInt32 - write int32 to slice
func (b *SliceWriter) WriteInt32(n int32) {
	binary.BigEndian.PutUint32(b.buf[b.pos:], uint32(n))
	b.pos += 4
}

// WriteUint64 - write uint64 to slice
func (b *SliceWriter) WriteUint64(n uint64) {
	binary.BigEndian.PutUint64(b.buf[b.pos:], n)
	b.pos += 8
}
