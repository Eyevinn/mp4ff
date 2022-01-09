package mp4

import (
	"encoding/binary"
	"errors"
)

var SliceWriterError = errors.New("overflow in SliceWriter")

// SliceWriter - write numbers to a fixed []byte slice
type SliceWriter struct {
	buf      []byte
	off      int
	accError error
}

// NewSliceWriter - create writer around slice.
// The slice will not grow, but stay the same size.
// If too much data is written, there will be
// an accumuluated error. Can be retrieved with AccError()
func NewSliceWriter(data []byte) *SliceWriter {
	return &SliceWriter{
		buf:      data,
		off:      0,
		accError: nil,
	}
}

// NewSliceWriter - create slice writer with fixed size.
func NewSliceWriterWithSize(size int) *SliceWriter {
	return &SliceWriter{
		buf:      make([]byte, size),
		off:      0,
		accError: nil,
	}
}

// Len - length of SliceWriter buffer
func (b *SliceWriter) Len() int {
	return len(b.buf)
}

// Offset - offset for writing in SliceWriter buffer
func (b *SliceWriter) Offset() int {
	return b.off
}

// AccError - return accumulated erro
func (b *SliceWriter) AccError() error {
	return b.accError
}

// WriteUint8 - write byte to slice
func (b *SliceWriter) WriteUint8(n byte) {
	if b.off+1 > len(b.buf) {
		b.accError = SliceWriterError
		return
	}
	b.buf[b.off] = n
	b.off++
}

// WriteUint16 - write uint16 to slice
func (b *SliceWriter) WriteUint16(n uint16) {
	if b.off+2 > len(b.buf) {
		b.accError = SliceWriterError
		return
	}
	binary.BigEndian.PutUint16(b.buf[b.off:], n)
	b.off += 2
}

// WriteInt16 - write int16 to slice
func (b *SliceWriter) WriteInt16(n int16) {
	if b.off+2 > len(b.buf) {
		b.accError = SliceWriterError
		return
	}
	binary.BigEndian.PutUint16(b.buf[b.off:], uint16(n))
	b.off += 2
}

// WriteUint24 - write uint24 to slice
func (b *SliceWriter) WriteUint24(n uint32) {
	if b.off+3 > len(b.buf) {
		b.accError = SliceWriterError
		return
	}
	b.WriteUint8(byte(n >> 16))
	b.WriteUint16(uint16(n & 0xffff))
}

// WriteUint32 - write uint32 to slice
func (b *SliceWriter) WriteUint32(n uint32) {
	if b.off+4 > len(b.buf) {
		b.accError = SliceWriterError
		return
	}
	binary.BigEndian.PutUint32(b.buf[b.off:], n)
	b.off += 4
}

// WriteInt32 - write int32 to slice
func (b *SliceWriter) WriteInt32(n int32) {
	if b.off+4 > len(b.buf) {
		b.accError = SliceWriterError
		return
	}
	binary.BigEndian.PutUint32(b.buf[b.off:], uint32(n))
	b.off += 4
}

// WriteUint64 - write uint64 to slice
func (b *SliceWriter) WriteUint64(n uint64) {
	if b.off+8 > len(b.buf) {
		b.accError = SliceWriterError
		return
	}
	binary.BigEndian.PutUint64(b.buf[b.off:], n)
	b.off += 8
}

// WriteInt64 - write int64 to slice
func (b *SliceWriter) WriteInt64(n int64) {
	if b.off+8 > len(b.buf) {
		b.accError = SliceWriterError
		return
	}
	binary.BigEndian.PutUint64(b.buf[b.off:], uint64(n))
	b.off += 8
}

// WriteString - write string to slice with or without zero end
func (b *SliceWriter) WriteString(s string, addZeroEnd bool) {
	nrNew := len(s)
	if addZeroEnd {
		nrNew++
	}
	if b.off+nrNew > len(b.buf) {
		b.accError = SliceWriterError
		return
	}
	copy(b.buf[b.off:b.off+len(s)], s)
	b.off += len(s)
	if addZeroEnd {
		b.buf[b.off] = 0
		b.off++
	}
}

// WriteZeroBytes - write n byte of zeroes
func (b *SliceWriter) WriteZeroBytes(n int) {
	if b.off+n > len(b.buf) {
		b.accError = SliceWriterError
		return
	}
	for i := 0; i < n; i++ {
		b.buf[b.off] = 0
		b.off++
	}
}

// WriteBytes - write []byte
func (b *SliceWriter) WriteBytes(byteSlice []byte) {
	if b.off+len(byteSlice) > len(b.buf) {
		b.accError = SliceWriterError
		return
	}
	copy(b.buf[b.off:b.off+len(byteSlice)], byteSlice)
	b.off += len(byteSlice)
}

// WriteUnityMatrix - write a unity matrix for mvhd or tkhd
func (b *SliceWriter) WriteUnityMatrix() {
	if b.off+36 > len(b.buf) {
		b.accError = SliceWriterError
		return
	}
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

// Bytes - return buf
func (b *SliceWriter) Bytes() []byte {
	return b.buf
}
