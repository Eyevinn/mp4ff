package bits

import (
	"io"
)

// EbspWriter writes bits into underlying io.Writer and inserts
// start-code emulation prevention bytes as necessary
// Stops writing at first error.
// Errors that have occured can later be checked with Error().
type EBSPWriter struct {
	n   int    // current number of bits
	v   uint   // current accumulated value
	err error  // The first error caused by any write operation
	nr0 int    // Number preceeding zero bytes
	out []byte // Slice of length 1 to avoid allocation at output

	wr io.Writer
}

// NewWriter - returns a new Writer
func NewEBSPWriter(w io.Writer) *EBSPWriter {
	return &EBSPWriter{
		wr:  w,
		out: make([]byte, 1),
	}
}

// Write - write n bits from bits and save error state
func (w *EBSPWriter) Write(bits uint, n int) {
	if w.err != nil {
		return
	}
	w.v <<= uint(n)
	w.v |= bits & mask(n)
	w.n += n
	for w.n >= 8 {
		b := (w.v >> (uint(w.n) - 8)) & mask(8)
		if w.nr0 == 2 && b <= 3 {
			w.out[0] = 0x3
			_, err := w.wr.Write(w.out)
			if err != nil {
				w.err = err
				return
			}
			w.nr0 = 0
		}
		w.out[0] = uint8(b)
		_, err := w.wr.Write(w.out)
		if err != nil {
			w.err = err
			return
		}
		if b == 0 {
			w.nr0++
		}
		w.n -= 8
	}
	w.v &= mask(8)
}

// Write - write rbsp trailing bits (a 1 followed by zeros to a byte boundary)
func (w *EBSPWriter) WriteRbspTrailingBits() {
	w.Write(1, 1)
	w.StuffByteWithZeros()
}

// StuffByteWithZeros - write zero bits until byte boundary (0-7bits)
func (w *EBSPWriter) StuffByteWithZeros() {
	if w.n > 0 {
		w.Write(0, 8-w.n)
	}
}
