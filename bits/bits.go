package bits

import (
	"encoding/binary"
	"io"
)

// Writer writes bits into underlying io.Writer. Stops writing at first error.
// Errors that have occurred can later be checked with Error().
type Writer struct {
	wr  io.Writer
	err error  // The first error caused by any write operation
	out []byte // Slice of length 1 to avoid allocation at output
	n   int    // current number of bits
	v   uint   // current accumulated value
}

// NewWriter - returns a new Writer
func NewWriter(w io.Writer) *Writer {
	return &Writer{
		wr:  w,
		out: make([]byte, 1),
	}
}

// Write - write n bits from bits and save error state
func (w *Writer) Write(bits uint, n int) {
	if w.err != nil {
		return
	}
	w.v <<= uint(n)
	w.v |= bits & mask(n)
	w.n += n
	for w.n >= 8 {
		b := (w.v >> (uint(w.n) - 8)) & mask(8)
		w.out[0] = uint8(b)
		_, err := w.wr.Write(w.out)
		if err != nil {
			w.err = err
			return
		}
		w.n -= 8
	}
	w.v &= mask(8)
}

// Flush - write remaining bits to the underlying io.Writer.
// bits will be left-shifted.
func (w *Writer) Flush() {
	if w.err != nil {
		return
	}
	if w.n != 0 {
		b := (w.v << (8 - uint(w.n))) & mask(8)
		if err := binary.Write(w.wr, binary.BigEndian, uint8(b)); err != nil {
			w.err = err
			return
		}
	}
}

// Error - error that has occurred and stopped writing
func (w *Writer) Error() error {
	return w.err
}

// Reader - read bits from the given io.Reader
type Reader struct {
	rd io.Reader
	n  int  // current number of bits
	v  uint // current accumulated value
}

// NewReader - return a new Reader
func NewReader(rd io.Reader) *Reader {
	return &Reader{
		rd: rd,
	}
}

// Read - read n bits
func (r *Reader) Read(n int) (uint, error) {
	var err error

	for r.n < n {
		r.v <<= 8
		var b uint8
		err = binary.Read(r.rd, binary.BigEndian, &b)
		if err != nil && err != io.EOF {
			return 0, err
		}
		r.v |= uint(b)

		r.n += 8
	}
	v := r.v >> uint(r.n-n)

	r.n -= n
	r.v &= mask(r.n)

	return v, err
}

// ReadFlag - read 1 bit into flag
func (r *Reader) ReadFlag() (bool, error) {
	bit, err := r.Read(1)
	if err != nil {
		return false, err
	}
	return bit == 1, nil
}

// MustRead - Read bits and panic if not possible
func (r *Reader) MustRead(n int) uint {
	var err error

	for r.n < n {
		r.v <<= 8
		var b uint8
		err = binary.Read(r.rd, binary.BigEndian, &b)
		if err != nil {
			panic("Reading error")
		}
		r.v |= uint(b)

		r.n += 8
	}
	v := r.v >> uint(r.n-n)

	r.n -= n
	r.v &= mask(r.n)

	return v
}

// MustReadFlag - read 1 bit into flag and panic if not possible
func (r *Reader) MustReadFlag() bool {
	return r.MustRead(1) == 1
}

// mask - n-bit binary mask
func mask(n int) uint {
	return (1 << uint(n)) - 1
}
