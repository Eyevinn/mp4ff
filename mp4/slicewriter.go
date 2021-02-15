package mp4

import (
	"encoding/binary"
)

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

// WriteUint8 - write byte to slice
func (b *SliceWriter) WriteUint8(n byte) {
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

// WriteUint24 - write uint24 to slice
func (b *SliceWriter) WriteUint24(n uint32) {
	b.WriteUint8(byte(n >> 16))
	b.WriteUint16(uint16(n & 0xffff))
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

// WriteInt64 - write int64 to slice
func (b *SliceWriter) WriteInt64(n int64) {
	binary.BigEndian.PutUint64(b.buf[b.pos:], uint64(n))
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

// WriteBytes - write []byte
func (b *SliceWriter) WriteBytes(byteSlice []byte) {
	for _, c := range byteSlice {
		b.buf[b.pos] = c
		b.pos++
	}
}

// WriteUnityMatrix - write a unity matrix for mvhd or tkhd
func (b *SliceWriter) WriteUnityMatrix() {
	b.WriteUint32(0x00010000) // = 1 fixed 16.16
	b.WriteUint32(0)
	b.WriteUint32(0)
	b.WriteUint32(0)
	b.WriteUint32(0x00010000) // = 1 fixed 16.16
	b.WriteUint32(0)
	b.WriteUint32(0)
	b.WriteUint32(0)
	b.WriteUint32(0x40000000) // = 1 fixed 2.30
}
