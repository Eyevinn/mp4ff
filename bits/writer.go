package bits

import (
	"encoding/binary"
	"io"
	"math"
	mathbits "math/bits"
)

// Writer writes bits into underlying io.Writer. Stops writing at first error.
// That first error is stored and can later be checked with AccError().
type Writer struct {
	wr  io.Writer
	err error  // The first error caused by any write operation
	out []byte // Slice of length 1 to avoid allocation at output
	n   int    // current number of bits
	v   uint64 // current accumulated value, 64 bits so that up to 32 bits can be written at any bit position
}

// NewWriter returns a new Writer
func NewWriter(w io.Writer) *Writer {
	return &Writer{
		wr:  w,
		out: make([]byte, 1),
	}
}

// Write writes n bits from bits and saves error state.
func (w *Writer) Write(bits uint, n int) {
	if w.err != nil {
		return
	}
	w.v <<= uint(n)
	w.v |= uint64(bits & Mask(n))
	w.n += n
	for w.n >= 8 {
		b := (w.v >> (uint(w.n) - 8)) & uint64(Mask(8))
		w.out[0] = uint8(b)
		_, err := w.wr.Write(w.out)
		if err != nil {
			w.err = err
			return
		}
		w.n -= 8
	}
	w.v &= uint64(Mask(8))
}

// Flush writes remaining bits to the underlying io.Writer by adding zeros to the right.
func (w *Writer) Flush() {
	if w.err != nil {
		return
	}
	if w.n != 0 {
		b := (w.v << (8 - uint(w.n))) & uint64(Mask(8))
		if err := binary.Write(w.wr, binary.BigEndian, uint8(b)); err != nil {
			w.err = err
			return
		}
	}
}

// AccError returns the first error that occurred and stopped writing.
func (w *Writer) AccError() error {
	return w.err
}

// Mask returns a binary mask for the n least significant bits.
func Mask(n int) uint {
	return (1 << uint(n)) - 1
}

// expGolombSplit returns the number of leading zero bits of the ue(v) code of n
// and the value of the bits that follow its first 1 bit.
// The code is n+1 written with 2*leadingZeroBits+1 bits, but n+1 does not fit in a uint if n is math.MaxUint.
func expGolombSplit(n uint) (leadingZeroBits int, suffix uint) {
	if n == math.MaxUint {
		leadingZeroBits = mathbits.UintSize
	} else {
		leadingZeroBits = mathbits.Len(n+1) - 1
	}
	return leadingZeroBits, n + 1 - 1<<leadingZeroBits
}
