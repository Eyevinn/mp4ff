package bits

import (
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
)

// AccErrReader - bit reader that accumulates error
// First error can be fetched as reader.AccError()
type AccErrReader struct {
	rd     io.Reader
	err    error
	nrBits int  // current number of bits
	value  uint // current accumulated value
}

// AccError - accumulated error is first error that occurred
func (r *AccErrReader) AccError() error {
	return r.err
}

// NewAccErrReader - return a new Reader
func NewAccErrReader(rd io.Reader) *AccErrReader {
	return &AccErrReader{
		rd: rd,
	}
}

// Read - read n bits. Return 0, if error now or previously
func (r *AccErrReader) Read(n int) uint {
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
	r.value &= mask(r.nrBits)

	return value
}

// ReadFlag - read 1 bit into flag. Return false if error now or previously
func (r *AccErrReader) ReadFlag() bool {
	bit := r.Read(1)
	if r.err != nil {
		return false
	}
	return bit == 1
}

// ReadFlag - Read i(v) which is 2-complement of n bits
func (r *AccErrReader) ReadVInt(n int) int {
	uval := r.Read(n)
	var ival int

	if uval >= 1<<(n/2) {
		ival = int(uval) - (1 << n)
	}
	return ival
}

// ReadRemainingBytes - read remaining bytes if byte-aligned
func (r *AccErrReader) ReadRemainingBytes() []byte {
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
