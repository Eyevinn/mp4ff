package bits

import (
	"encoding/binary"
	"io"
)

// ByteWriter - writer that wraps an io.Writer and accumulates error.
// Only the first error is saved, but any later calls will not panic.
type ByteWriter struct {
	w   io.Writer
	err error
	buf [8]byte // for encoding values without allocating
}

// NewByteWriter creates accumulated error writer around io.Writer.
func NewByteWriter(w io.Writer) *ByteWriter {
	return &ByteWriter{
		w: w,
	}
}

// AccError - return accumulated error
func (a *ByteWriter) AccError() error {
	return a.err
}

// WriteUint8 - write a byte
func (a *ByteWriter) WriteUint8(b byte) {
	if a.err != nil {
		return
	}
	a.buf[0] = b
	a.write(1)
}

// WriteUint16 - write uint16
func (a *ByteWriter) WriteUint16(u uint16) {
	if a.err != nil {
		return
	}
	binary.BigEndian.PutUint16(a.buf[:], u)
	a.write(2)
}

// WriteUint32 - write uint32
func (a *ByteWriter) WriteUint32(u uint32) {
	if a.err != nil {
		return
	}
	binary.BigEndian.PutUint32(a.buf[:], u)
	a.write(4)
}

// WriteUint48 - write uint48
func (a *ByteWriter) WriteUint48(u uint64) {
	if a.err != nil {
		return
	}
	binary.BigEndian.PutUint16(a.buf[:], uint16(u>>32))
	binary.BigEndian.PutUint32(a.buf[2:], uint32(u))
	a.write(6)
}

// WriteUint64 - write uint64
func (a *ByteWriter) WriteUint64(u uint64) {
	if a.err != nil {
		return
	}
	binary.BigEndian.PutUint64(a.buf[:], u)
	a.write(8)
}

// write writes the first n bytes of a.buf.
func (a *ByteWriter) write(n int) {
	_, a.err = a.w.Write(a.buf[:n])
}

// WriteSlice - write a slice
func (a *ByteWriter) WriteSlice(s []byte) {
	if a.err != nil {
		return
	}
	_, a.err = a.w.Write(s)
}
