package mp4

import (
	"encoding/binary"
	"errors"
)

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

// ReadByte - read uint8 from slice
func (s *SliceReader) ReadByte() byte {
	res := s.slice[s.pos]
	s.pos++
	return res
}

// ReadUint16 - read uint16 from slice
func (s *SliceReader) ReadUint16() uint16 {
	res := binary.BigEndian.Uint16(s.slice[s.pos : s.pos+2])
	s.pos += 2
	return res
}

// ReadInt16 - read int16 from slice
func (s *SliceReader) ReadInt16() int16 {
	res := binary.BigEndian.Uint16(s.slice[s.pos : s.pos+2])
	s.pos += 2
	return int16(res)
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

// ReadFixedLengthString - read string of specified length
func (s *SliceReader) ReadFixedLengthString(length int) string {
	res := string(s.slice[s.pos : s.pos+length])
	s.pos += length
	return res
}

// ReadZeroTerminatedString - read string until zero
func (s *SliceReader) ReadZeroTerminatedString() (string, error) {
	startPos := s.pos
	for {
		c := s.slice[s.pos]
		if c == 0 {
			str := string(s.slice[startPos:s.pos])
			s.pos++ // Next position to read
			return str, nil
		}
		s.pos++
		if s.pos >= len(s.slice) {
			return "", errors.New("Did not find terminating zero")
		}
	}
}

// RemainingBytes - return remaining bytes of this slice
func (s *SliceReader) RemainingBytes() []byte {
	res := s.slice[s.pos:]
	s.pos = s.Length()
	return res
}

// SkipBytes - skip passed n bytes
func (s *SliceReader) SkipBytes(n int) {
	if s.pos+n > s.Length() {
		panic("Skipping past end of box")
	}
	s.pos += n
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

// WriteByte - write byte to slice
func (b *SliceWriter) WriteByte(n byte) {
	b.buf[b.pos] = n
	b.pos++
}

// WriteUint16 - write uint16 to slice
func (b *SliceWriter) WriteUint16(n uint16) {
	binary.BigEndian.PutUint16(b.buf[b.pos:], n)
	b.pos += 2
}

// WriteInt16 - write int16 to slice
func (b *SliceWriter) WriteInt16(n int16) {
	binary.BigEndian.PutUint16(b.buf[b.pos:], uint16(n))
	b.pos += 2
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

// WriteString - write string to slice with or without zero end
func (b *SliceWriter) WriteString(s string, addZeroEnd bool) {
	for _, c := range s {
		b.buf[b.pos] = byte(c)
		b.pos++
	}
	if addZeroEnd {
		b.buf[b.pos] = 0
		b.pos++
	}
}

// WriteZeroBytes - write n byte of zeroes
func (b *SliceWriter) WriteZeroBytes(n int) {
	for i := 0; i < n; i++ {
		b.buf[b.pos] = 0
		b.pos++
	}
}
