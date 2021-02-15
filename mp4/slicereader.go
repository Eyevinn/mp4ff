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

// ReadUint8 - read uint8 from slice
func (s *SliceReader) ReadUint8() byte {
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

// ReadUint24 - read uint24 from slice
func (s *SliceReader) ReadUint24() uint32 {
	p1 := s.ReadUint8()
	p2 := s.ReadUint16()
	return (uint32(p1) << 16) + uint32(p2)
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

// ReadBytes - read a slice of bytes
func (s *SliceReader) ReadBytes(n int) []byte {
	res := s.slice[s.pos : s.pos+n]
	s.pos += n
	return res
}

// RemainingBytes - return remaining bytes of this slice
func (s *SliceReader) RemainingBytes() []byte {
	res := s.slice[s.pos:]
	s.pos = s.Length()
	return res
}

// NrRemaingingByts - return number of bytes remaining
func (s *SliceReader) NrRemainingBytes() int {
	return s.Length() - s.GetPos()
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
