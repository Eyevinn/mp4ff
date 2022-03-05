package bits

import "encoding/binary"

// FixedSliceWriter - write numbers to a fixed []byte slice
type FixedSliceWriter struct {
	buf      []byte
	off      int
	n        int  // current number of bits
	v        uint // current accumulated value for bits
	accError error
}

// NewFixedSliceWriter - create writer around slice.
// The slice will not grow, but stay the same size.
// If too much data is written, there will be
// an accumuluated error. Can be retrieved with AccError()
func NewFixedSliceWriterFromSlice(data []byte) *FixedSliceWriter {
	return &FixedSliceWriter{
		buf:      data,
		off:      0,
		n:        0,
		v:        0,
		accError: nil,
	}
}

// NewSliceWriter - create slice writer with fixed size.
func NewFixedSliceWriter(size int) *FixedSliceWriter {
	return &FixedSliceWriter{
		buf:      make([]byte, size),
		off:      0,
		n:        0,
		v:        0,
		accError: nil,
	}
}

// Len - length of FixedSliceWriter buffer written. Same as Offset()
func (b *FixedSliceWriter) Len() int {
	return b.off
}

// Capacity - max length of FixedSliceWriter buffer
func (b *FixedSliceWriter) Capacity() int {
	return len(b.buf)
}

// Offset - offset for writing in FixedSliceWriter buffer
func (b *FixedSliceWriter) Offset() int {
	return b.off
}

// Bytes - return buf up to what's written
func (b *FixedSliceWriter) Bytes() []byte {
	return b.buf[:b.off]
}

// AccError - return accumulated erro
func (b *FixedSliceWriter) AccError() error {
	return b.accError
}

// WriteUint8 - write byte to slice
func (b *FixedSliceWriter) WriteUint8(n byte) {
	if b.off+1 > len(b.buf) {
		b.accError = SliceWriterError
		return
	}
	b.buf[b.off] = n
	b.off++
}

// WriteUint16 - write uint16 to slice
func (b *FixedSliceWriter) WriteUint16(n uint16) {
	if b.off+2 > len(b.buf) {
		b.accError = SliceWriterError
		return
	}
	binary.BigEndian.PutUint16(b.buf[b.off:], n)
	b.off += 2
}

// WriteInt16 - write int16 to slice
func (b *FixedSliceWriter) WriteInt16(n int16) {
	if b.off+2 > len(b.buf) {
		b.accError = SliceWriterError
		return
	}
	binary.BigEndian.PutUint16(b.buf[b.off:], uint16(n))
	b.off += 2
}

// WriteUint24 - write uint24 to slice
func (b *FixedSliceWriter) WriteUint24(n uint32) {
	if b.off+3 > len(b.buf) {
		b.accError = SliceWriterError
		return
	}
	b.WriteUint8(byte(n >> 16))
	b.WriteUint16(uint16(n & 0xffff))
}

// WriteUint32 - write uint32 to slice
func (b *FixedSliceWriter) WriteUint32(n uint32) {
	if b.off+4 > len(b.buf) {
		b.accError = SliceWriterError
		return
	}
	binary.BigEndian.PutUint32(b.buf[b.off:], n)
	b.off += 4
}

// WriteInt32 - write int32 to slice
func (b *FixedSliceWriter) WriteInt32(n int32) {
	if b.off+4 > len(b.buf) {
		b.accError = SliceWriterError
		return
	}
	binary.BigEndian.PutUint32(b.buf[b.off:], uint32(n))
	b.off += 4
}

// WriteUint48 - write uint48
func (b *FixedSliceWriter) WriteUint48(u uint64) {
	if b.accError != nil {
		return
	}
	msb := uint16(u >> 32)
	binary.BigEndian.PutUint16(b.buf[b.off:], msb)
	b.off += 2

	lsb := uint32(u & 0xffffffff)
	binary.BigEndian.PutUint32(b.buf[b.off:], lsb)
	b.off += 4
}

// WriteUint64 - write uint64 to slice
func (b *FixedSliceWriter) WriteUint64(n uint64) {
	if b.off+8 > len(b.buf) {
		b.accError = SliceWriterError
		return
	}
	binary.BigEndian.PutUint64(b.buf[b.off:], n)
	b.off += 8
}

// WriteInt64 - write int64 to slice
func (b *FixedSliceWriter) WriteInt64(n int64) {
	if b.off+8 > len(b.buf) {
		b.accError = SliceWriterError
		return
	}
	binary.BigEndian.PutUint64(b.buf[b.off:], uint64(n))
	b.off += 8
}

// WriteString - write string to slice with or without zero end
func (b *FixedSliceWriter) WriteString(s string, addZeroEnd bool) {
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
func (b *FixedSliceWriter) WriteZeroBytes(n int) {
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
func (b *FixedSliceWriter) WriteBytes(byteSlice []byte) {
	if b.off+len(byteSlice) > len(b.buf) {
		b.accError = SliceWriterError
		return
	}
	copy(b.buf[b.off:b.off+len(byteSlice)], byteSlice)
	b.off += len(byteSlice)
}

// WriteUnityMatrix - write a unity matrix for mvhd or tkhd
func (b *FixedSliceWriter) WriteUnityMatrix() {
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

func (sw *FixedSliceWriter) WriteBits(bits uint, n int) {
	if sw.accError != nil {
		return
	}
	sw.v <<= uint(n)
	sw.v |= bits & mask(n)
	sw.n += n
	for sw.n >= 8 {
		b := byte((sw.v >> (uint(sw.n) - 8)) & mask(8))
		sw.WriteUint8(b)
		sw.n -= 8
	}
	sw.v &= mask(8)
}

// FlushBits - write remaining bits to the underlying .Writer.
// bits will be left-shifted.
func (sw *FixedSliceWriter) FlushBits() {
	if sw.accError != nil {
		return
	}
	if sw.n != 0 {
		b := byte((sw.v << (8 - uint(sw.n))) & mask(8))
		sw.WriteUint8(b)
	}
}
