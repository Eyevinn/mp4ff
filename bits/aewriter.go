package bits

import (
	"encoding/binary"
	"io"
)

// AccErrWriter - writer that wraps an io.Writer and accumulater error
type AccErrWriter struct {
	w   io.Writer
	err error
}

// NewAccErrWriter - create accumulated error writer around io.Writer
func NewAccErrWriter(w io.Writer) *AccErrWriter {
	return &AccErrWriter{
		w: w,
	}
}

// AccError - return accumulated error
func (a *AccErrWriter) AccError() error {
	return a.err
}

func (a *AccErrWriter) WriteUint8(b byte) {
	if a.err != nil {
		return
	}
	a.err = binary.Write(a.w, binary.BigEndian, b)
}

func (a *AccErrWriter) WriteUint16(u uint16) {
	if a.err != nil {
		return
	}
	a.err = binary.Write(a.w, binary.BigEndian, u)
}

func (a *AccErrWriter) WriteUint32(u uint32) {
	if a.err != nil {
		return
	}
	a.err = binary.Write(a.w, binary.BigEndian, u)
}

func (a *AccErrWriter) WriteUint48(u uint64) {
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

func (a *AccErrWriter) WriteUint64(u uint64) {
	if a.err != nil {
		return
	}
	a.err = binary.Write(a.w, binary.BigEndian, u)
}

func (a *AccErrWriter) WriteSlice(s []byte) {
	if a.err != nil {
		return
	}
	_, a.err = a.w.Write(s)
}
