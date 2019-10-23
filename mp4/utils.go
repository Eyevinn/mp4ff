package mp4

import "encoding/binary"

type SliceReader struct {
	slice []byte
	pos   int
}

func NewSliceReader(data []byte) *SliceReader {
	return &SliceReader{
		slice: data,
		pos:   0,
	}
}

func (s *SliceReader) ReadUint32() uint32 {
	res := binary.BigEndian.Uint32(s.slice[s.pos : s.pos+4])
	s.pos += 4
	return res
}

func (s *SliceReader) ReadInt32() int32 {
	res := binary.BigEndian.Uint32(s.slice[s.pos : s.pos+4])
	s.pos += 4
	return int32(res)
}

func (s *SliceReader) ReadUint64() uint64 {
	res := binary.BigEndian.Uint64(s.slice[s.pos : s.pos+8])
	s.pos += 8
	return res
}

func (s *SliceReader) ReadInt64() int64 {
	res := binary.BigEndian.Uint64(s.slice[s.pos : s.pos+8])
	s.pos += 8
	return int64(res)
}

func (s *SliceReader) SetPos(pos int) {
	s.pos = pos
}

func (s *SliceReader) GetPos() int {
	return s.pos
}

func (s *SliceReader) Length() int {
	return len(s.slice)
}

// BufferWrapper adds methoeds for writing numbers to a []byte slices

type BufferWrapper struct {
	buf []byte
	pos int
}

func NewBufferWrapper(data []byte) *BufferWrapper {
	return &BufferWrapper{
		buf: data,
		pos: 0,
	}
}

func (b *BufferWrapper) WriteUint32(n uint32) {
	binary.BigEndian.PutUint32(b.buf[b.pos:], n)
	b.pos += 4
}

func (b *BufferWrapper) WriteInt32(n int32) {
	binary.BigEndian.PutUint32(b.buf[b.pos:], uint32(n))
	b.pos += 4
}

func (b *BufferWrapper) WriteUint64(n uint64) {
	binary.BigEndian.PutUint64(b.buf[b.pos:], n)
	b.pos += 8
}
