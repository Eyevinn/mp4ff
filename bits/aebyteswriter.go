package bits

import (
	"encoding/binary"
	"io"
)

// AccErrByteWriter - writer that wraps an io.Writer and accumulates error
type AccErrByteWriter struct {
	w   io.Writer
	err error
}

// NewAccErrByteWriter - create accumulated error writer around io.Writer
func NewAccErrByteWriter(w io.Writer) *AccErrByteWriter {
	return &AccErrByteWriter{
		w: w,
	}
}

// AccError - return accumulated error
func (a *AccErrByteWriter) AccError() error {
	return a.err
}

// WriteUint8 - write a byte
func (a *AccErrByteWriter) WriteUint8(b byte) {
	if a.err != nil {
		return
	}
	a.err = binary.Write(a.w, binary.BigEndian, b)
}

// WriteUint16 - write uint16
func (a *AccErrByteWriter) WriteUint16(u uint16) {
	if a.err != nil {
		return
	}
	a.err = binary.Write(a.w, binary.BigEndian, u)
}

// WriteUint32 - write uint32
func (a *AccErrByteWriter) WriteUint32(u uint32) {
	if a.err != nil {
		return
	}
	a.err = binary.Write(a.w, binary.BigEndian, u)
}

// WriteUint48 - write uint48
func (a *AccErrByteWriter) WriteUint48(u uint64) {
	if a.err != nil {
		return
	}
	msb := uint16(u >> 32)
	a.err = binary.Write(a.w, binary.BigEndian, msb)
	if a.err != nil {
		return
	}
	lsb := uint32(u & 0xffffffff)
	a.err = binary.Write(a.w, binary.BigEndian, lsb)
}

// WriteUint64 - write uint64
func (a *AccErrByteWriter) WriteUint64(u uint64) {
	if a.err != nil {
		return
	}
	a.err = binary.Write(a.w, binary.BigEndian, u)
}

// WriteSlice - write a slice
func (a *AccErrByteWriter) WriteSlice(s []byte) {
	if a.err != nil {
		return
	}
	_, a.err = a.w.Write(s)
}
