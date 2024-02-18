package bits

import (
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
)

// Reader is a bit reader that stops reading at first error and stores it.
// First error can be fetched usiin AccError().
type Reader struct {
	rd     io.Reader
	err    error
	nrBits int  // current number of bits
	value  uint // current accumulated value
}

// AccError - accumulated error is first error that occurred
func (r *Reader) AccError() error {
	return r.err
}

// NewReader return a new Reader that accumulates errors.
func NewReader(rd io.Reader) *Reader {
	return &Reader{
		rd: rd,
	}
}

// Read - read n bits. Return 0, if error now or previously
func (r *Reader) Read(n int) uint {
	if r.err != nil {
		return 0
	}

	for r.nrBits < n {
		r.value <<= 8
		var newByte uint8
		err := binary.Read(r.rd, binary.BigEndian, &newByte)
		if err != nil {
			r.err = err
			return 0
		}
		r.value |= uint(newByte)

		r.nrBits += 8
	}
	value := r.value >> uint(r.nrBits-n)

	r.nrBits -= n
	r.value &= Mask(r.nrBits)

	return value
}

// ReadSigned reads a 2-complemented signed int with n bits.
func (r *Reader) ReadSigned(n int) int {
	nr := int(r.Read(n))
	firstBit := nr >> (n - 1)
	if firstBit == 1 {
		nr |= -1 << n
	}
	return nr
}

// ReadFlag reads 1 bit and interprets as a boolean flag. Returns false if error now or previously.
func (r *Reader) ReadFlag() bool {
	bit := r.Read(1)
	if r.err != nil {
		return false
	}
	return bit == 1
}

// ReadRemainingBytes reads remaining bytes if byte-aligned. Returns nil if error now or previously.
func (r *Reader) ReadRemainingBytes() []byte {
	if r.err != nil {
		return nil
	}
	if r.nrBits != 0 {
		r.err = fmt.Errorf("%d bit instead of byte alignment when reading remaining bytes", r.nrBits)
		return nil
	}
	rest, err := ioutil.ReadAll(r.rd)
	if err != nil {
		r.err = err
		return nil
	}
	return rest
}
