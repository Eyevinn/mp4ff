package bits

import (
	"encoding/binary"
	"fmt"
	"io"
)

// AccErrEBSPReader - Reader that drops start code emulation 0x03 after two bytes of 0x00
type AccErrEBSPReader struct {
	rd        io.Reader
	err       error
	n         int  // current number of bits
	v         uint // current accumulated value
	pos       int
	zeroCount int // Count number of zero bytes read
}

// NewAccErrEBSPReader - return a new reader accumulating errors.
func NewAccErrEBSPReader(rd io.Reader) *AccErrEBSPReader {
	return &AccErrEBSPReader{
		rd:  rd,
		pos: -1,
	}
}

// AccError - accumulated error
func (r *AccErrEBSPReader) AccError() error {
	return r.err
}

// NrBytesRead - how many bytes read into parser
func (r *AccErrEBSPReader) NrBytesRead() int {
	return r.pos + 1 // Starts at -1
}

// NrBitsReadInCurrentByte - how many bits have been read
func (r *AccErrEBSPReader) NrBitsReadInCurrentByte() int {
	return 8 - r.n
}

// Read - read n bits and return 0 if (previous) error
func (r *AccErrEBSPReader) Read(n int) uint {
	if r.err != nil {
		return 0
	}
	var err error
	for r.n < n {
		r.v <<= 8
		var b uint8
		err = binary.Read(r.rd, binary.BigEndian, &b)
		if err != nil {
			r.err = err
			return 0
		}
		r.pos++
		if r.zeroCount == 2 && b == startCodeEmulationPreventionByte {
			err = binary.Read(r.rd, binary.BigEndian, &b)
			if err != nil {
				r.err = err
				return 0
			}
			r.pos++
			r.zeroCount = 0
		}
		if b != 0 {
			r.zeroCount = 0
		} else {
			r.zeroCount++
		}
		r.v |= uint(b)

		r.n += 8
	}
	v := r.v >> uint(r.n-n)

	r.n -= n
	r.v &= mask(r.n)

	return v
}

// Read - read n bytes and return nil if (previous) error or if n bytes not available
func (r *AccErrEBSPReader) ReadBytes(n int) []byte {
	if r.err != nil {
		return nil
	}
	payload := make([]byte, n)
	for i := 0; i < n; i++ {
		b := byte(r.Read(8))
		payload[i] = b
	}
	if r.err != nil {
		return nil
	}
	return payload
}

// ReadFlag - read 1 bit into bool. Return false if not possible
func (r *AccErrEBSPReader) ReadFlag() bool {
	return r.Read(1) == 1
}

// ReadExpGolomb - Read one unsigned exponential golomb code. Return 0 if error
func (r *AccErrEBSPReader) ReadExpGolomb() uint {
	if r.err != nil {
		return 0
	}
	leadingZeroBits := 0

	for {
		b := r.Read(1)
		if r.err != nil {
			return 0
		}
		if b == 1 {
			break
		}
		leadingZeroBits++
	}

	var res uint = (1 << leadingZeroBits) - 1

	endBits := r.Read(leadingZeroBits)
	if r.err != nil {
		return 0
	}

	return res + endBits
}

// ReadSignedGolomb - Read one signed exponential golomb code. Return 0 if error
func (r *AccErrEBSPReader) ReadSignedGolomb() int {
	if r.err != nil {
		return 0
	}
	unsignedGolomb := r.ReadExpGolomb()
	if r.err != nil {
		return 0
	}
	if unsignedGolomb%2 == 1 {
		return int((unsignedGolomb + 1) / 2)
	}
	return -int(unsignedGolomb / 2)
}

// IsSeeker - does reader support Seek
func (r *AccErrEBSPReader) IsSeeker() bool {
	_, ok := r.rd.(io.ReadSeeker)
	return ok
}

// MoreRbspData - false if next bit is 1 and last 1 in fullSlice.
// Underlying reader must support ReadSeeker interface to reset after check
// Return false, nil if underlying error
func (r *AccErrEBSPReader) MoreRbspData() (bool, error) {
	if !r.IsSeeker() {
		return false, ErrNotReedSeeker
	}
	// Find out if next position is the last 1
	stateCopy := *r

	firstBit := r.Read(1)
	if r.err != nil {
		return false, nil
	}
	if firstBit != 1 {
		err := r.reset(stateCopy)
		if err != nil {
			return false, err
		}
		return true, nil
	}
	// If all remainging bits are zero, there is no more rbsp data
	more := false
	for {
		b := r.Read(1)
		if r.err == io.EOF {
			r.err = nil // Reset
			break
		}
		if r.err != nil {
			return false, nil
		}
		if b == 1 {
			more = true
			break
		}
	}
	err := r.reset(stateCopy)
	if err != nil {
		return false, err
	}
	return more, nil
}

// reset EBSPReader based on copy of previous state
func (r *AccErrEBSPReader) reset(prevState AccErrEBSPReader) error {
	rdSeek, ok := r.rd.(io.ReadSeeker)

	if !ok {
		return ErrNotReedSeeker
	}

	_, err := rdSeek.Seek(int64(prevState.pos+1), 0)
	if err != nil {
		return err
	}
	r.n = prevState.n
	r.v = prevState.v
	r.pos = prevState.pos
	r.zeroCount = prevState.zeroCount
	return nil
}

// ReadRbspTrailingBits - read rbsp_traling_bits. Return error if wrong pattern
// If other error, return nil and let AccError provide that error
func (r *AccErrEBSPReader) ReadRbspTrailingBits() error {
	if r.err != nil {
		return nil
	}
	firstBit := r.Read(1)
	if r.err != nil {
		return nil
	}
	if firstBit != 1 {
		return fmt.Errorf("RbspTrailingBits don't start with 1")
	}
	for {
		b := r.Read(1)
		if r.err == io.EOF {
			r.err = nil // Reset
			return nil
		}
		if r.err != nil {
			return nil
		}
		if b == 1 {
			return fmt.Errorf("Another 1 in RbspTrailingBits")
		}
	}
}

// SetError - set an error if not already set.
func (r *AccErrEBSPReader) SetError(err error) {
	if r.err != nil {
		r.err = err
	}
}
