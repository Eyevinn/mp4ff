package bits

import (
	"encoding/binary"
	"errors"
	"fmt"
)

var SliceReadError = fmt.Errorf("Read too far in SliceReader")

// SliceReader - read integers and other data from a slice.
// Accumulates error, and the first error can be retrived.
// If err != nil, 0 or empty string is returned
type SliceReader struct {
	slice []byte
	pos   int
	len   int
	err   error
}

// NewSliceReader - create a new slice reader reading from data
func NewSliceReader(data []byte) *SliceReader {
	return &SliceReader{
		slice: data,
		pos:   0,
		len:   len(data),
		err:   nil,
	}
}

// AccError - get accumulated error after read operations
func (s *SliceReader) AccError() error {
	return s.err
}

// ReadUint8 - read uint8 from slice
func (s *SliceReader) ReadUint8() byte {
	if s.err != nil {
		return 0
	}
	if s.pos > s.len-1 {
		s.err = SliceReadError
		return 0
	}
	res := s.slice[s.pos]
	s.pos++
	return res
}

// ReadUint16 - read uint16 from slice
func (s *SliceReader) ReadUint16() uint16 {
	if s.err != nil {
		return 0
	}
	if s.pos > s.len-2 {
		s.err = SliceReadError
		return 0
	}
	res := binary.BigEndian.Uint16(s.slice[s.pos : s.pos+2])
	s.pos += 2
	return res
}

// ReadInt16 - read int16 from slice
func (s *SliceReader) ReadInt16() int16 {
	if s.err != nil {
		return 0
	}
	if s.pos > s.len-2 {
		s.err = SliceReadError
		return 0
	}
	res := binary.BigEndian.Uint16(s.slice[s.pos : s.pos+2])
	s.pos += 2
	return int16(res)
}

// ReadUint32 - read uint32 from slice
func (s *SliceReader) ReadUint32() uint32 {
	if s.err != nil {
		return 0
	}
	if s.pos > s.len-4 {
		s.err = SliceReadError
		return 0
	}
	res := binary.BigEndian.Uint32(s.slice[s.pos : s.pos+4])
	s.pos += 4
	return res
}

// ReadInt32 - read int32 from slice
func (s *SliceReader) ReadInt32() int32 {
	if s.err != nil {
		return 0
	}
	if s.pos > s.len-4 {
		s.err = SliceReadError
		return 0
	}
	res := binary.BigEndian.Uint32(s.slice[s.pos : s.pos+4])
	s.pos += 4
	return int32(res)
}

// ReadUint64 - read uint64 from slice
func (s *SliceReader) ReadUint64() uint64 {
	if s.err != nil {
		return 0
	}
	if s.pos > s.len-8 {
		s.err = SliceReadError
		return 0
	}
	res := binary.BigEndian.Uint64(s.slice[s.pos : s.pos+8])
	s.pos += 8
	return res
}

// ReadInt64 - read int64 from slice
func (s *SliceReader) ReadInt64() int64 {
	if s.err != nil {
		return 0
	}
	if s.pos > s.len-8 {
		s.err = SliceReadError
		return 0
	}
	res := binary.BigEndian.Uint64(s.slice[s.pos : s.pos+8])
	s.pos += 8
	return int64(res)
}

// ReadFixedLengthString - read string of specified length n.
// Sets err and returns empty string if full length not available
func (s *SliceReader) ReadFixedLengthString(n int) string {
	if s.err != nil {
		return ""
	}
	if s.pos > s.len-n {
		s.err = SliceReadError
		return ""
	}
	res := string(s.slice[s.pos : s.pos+n])
	s.pos += n
	return res
}

// ReadZeroTerminatedString - read string until zero byte
// Set err and return empty string if no zero byte found
func (s *SliceReader) ReadZeroTerminatedString() string {
	if s.err != nil {
		return ""
	}
	startPos := s.pos
	for {
		c := s.slice[s.pos]
		if c == 0 {
			str := string(s.slice[startPos:s.pos])
			s.pos++ // Next position to read
			return str
		}
		s.pos++
		if s.pos >= len(s.slice) {
			s.err = errors.New("Did not find terminating zero")
			return ""
		}
	}
}

// ReadBytes - read a slice of n bytes
// Return empty slice if n bytes not available
func (s *SliceReader) ReadBytes(n int) []byte {
	if s.err != nil {
		return []byte{}
	}
	if s.pos > s.len-n {
		s.err = SliceReadError
		return []byte{}
	}
	res := s.slice[s.pos : s.pos+n]
	s.pos += n
	return res
}

// RemainingBytes - return remaining bytes of this slice
func (s *SliceReader) RemainingBytes() []byte {
	if s.err != nil {
		return []byte{}
	}
	res := s.slice[s.pos:]
	s.pos = s.Length()
	return res
}

// NrRemaingingByts - return number of bytes remaining
func (s *SliceReader) NrRemainingBytes() int {
	if s.err != nil {
		return 0
	}
	return s.Length() - s.GetPos()
}

// SkipBytes - skip passed n bytes
func (s *SliceReader) SkipBytes(n int) {
	if s.err != nil {
		return
	}
	if s.pos+n > s.Length() {
		s.err = fmt.Errorf("Attempt to skip bytes to pos %d beyond slice len %d", s.pos+n, s.len)
		return
	}
	s.pos += n
}

// SetPos - set read position is slice
func (s *SliceReader) SetPos(pos int) {
	if pos > s.len {
		s.err = fmt.Errorf("Attempt to set pos %d beyond slice len %d", pos, s.len)
		return
	}
	s.pos = pos
}

// GetPos - get read position is slice
func (s *SliceReader) GetPos() int {
	return s.pos
}

// Length - get length of slice
func (s *SliceReader) Length() int {
	return s.len
}
