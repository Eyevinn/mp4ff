package bits

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const (
	startCodeEmulationPreventionByte = 0x03
)

// ESBPReader errors
var (
	ErrNotReedSeeker = errors.New("Reader does not support Seek")
)

// NewEBSPReader - return a new Reader.
func NewEBSPReader(rd io.Reader) *EBSPReader {
	return &EBSPReader{
		rd:  rd,
		pos: -1,
	}
}

// EBSPReader - Reader that drops start code emulation 0x03 after two bytes of 0x00
type EBSPReader struct {
	rd        io.Reader
	n         int  // current number of bits
	v         uint // current accumulated value
	pos       int
	zeroCount int // Count number of zero bytes read
}

// MustRead - read n bits and panic if not possible
func (r *EBSPReader) MustRead(n int) uint {
	var err error

	for r.n < n {
		r.v <<= 8
		var b uint8
		err = binary.Read(r.rd, binary.BigEndian, &b)
		if err != nil {
			panic("Reading error")
		}
		r.pos++
		if r.zeroCount == 2 && b == startCodeEmulationPreventionByte {
			err = binary.Read(r.rd, binary.BigEndian, &b)
			if err != nil {
				panic("Reading error")
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

// MustReadFlag - read 1 bit into flag. Panic if not possible
func (r *EBSPReader) MustReadFlag() bool {
	return r.MustRead(1) == 1
}

// MustReadExpGolomb - Read one unsigned exponential golomb code. Panic if not possible
func (r *EBSPReader) MustReadExpGolomb() uint {
	leadingZeroBits := 0

	for {
		b := r.MustRead(1)
		if b == 1 {
			break
		}
		leadingZeroBits++
	}

	var res uint = (1 << leadingZeroBits) - 1
	endBits := r.MustRead(leadingZeroBits)

	return res + endBits
}

// MustReadSignedGolomb - Read one signed exponential golomb code. Panic if not possible
func (r *EBSPReader) MustReadSignedGolomb() int {
	unsignedGolomb := r.MustReadExpGolomb()
	if unsignedGolomb%2 == 1 {
		return int((unsignedGolomb + 1) / 2)
	}
	return -int(unsignedGolomb / 2)
}

// NrBytesRead - how many bytes read into parser
func (r *EBSPReader) NrBytesRead() int {
	return r.pos + 1 // Starts at -1
}

// NrBitsReadInCurrentByte - how many bits have been read
func (r *EBSPReader) NrBitsReadInCurrentByte() int {
	return 8 - r.n
}

// EBSP2rbsp - convert from EBSP to RBSP by removing start code emulation prevention bytes
func EBSP2rbsp(ebsp []byte) []byte {
	zeroCount := 0
	output := make([]byte, 0, len(ebsp))
	for i := 0; i < len(ebsp); i++ {
		b := ebsp[i]
		if zeroCount == 2 && b == startCodeEmulationPreventionByte {
			zeroCount = 0
		} else {
			if b != 0 {
				zeroCount = 0
			} else {
				zeroCount++
			}
			output = append(output, ebsp[i])
		}
	}
	return output
}

// Read - read n bits and return error if not possible
func (r *EBSPReader) Read(n int) (uint, error) {
	var err error

	for r.n < n {
		r.v <<= 8
		var b uint8
		err = binary.Read(r.rd, binary.BigEndian, &b)
		if err != nil {
			return 0, err
		}
		r.pos++
		if r.zeroCount == 2 && b == startCodeEmulationPreventionByte {
			err = binary.Read(r.rd, binary.BigEndian, &b)
			if err != nil {
				return 0, err
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

	return v, nil
}

// ReadFlag - read 1 bit into flag. Return error if not possible
func (r *EBSPReader) ReadFlag() (bool, error) {
	bit, err := r.Read(1)
	if err != nil {
		return false, err
	}
	return bit == 1, nil
}

// ReadExpGolomb - Read one unsigned exponential golomb code
func (r *EBSPReader) ReadExpGolomb() (uint, error) {
	leadingZeroBits := 0

	for {
		b, err := r.Read(1)
		if err != nil {
			return 0, err
		}
		if b == 1 {
			break
		}
		leadingZeroBits++
	}

	var res uint = (1 << leadingZeroBits) - 1

	endBits, err := r.Read(leadingZeroBits)
	if err != nil {
		return 0, err
	}

	return res + endBits, nil
}

// ReadSignedGolomb - Read one signed exponential golomb code
func (r *EBSPReader) ReadSignedGolomb() (int, error) {
	unsignedGolomb, err := r.ReadExpGolomb()
	if err != nil {
		return 0, err
	}
	if unsignedGolomb%2 == 1 {
		return int((unsignedGolomb + 1) / 2), nil
	}
	return -int(unsignedGolomb / 2), nil
}

// IsSeeker - does reader support Seek
func (r *EBSPReader) IsSeeker() bool {
	_, ok := r.rd.(io.ReadSeeker)
	return ok
}

// MoreRbspData - false if next bit is 1 and last 1 in fullSlice
// Underlying reader must support ReadSeeker interface to reset after check
func (r *EBSPReader) MoreRbspData() (bool, error) {
	if !r.IsSeeker() {
		return false, ErrNotReedSeeker
	}
	// Find out if next position is the last 1
	stateCopy := *r

	firstBit, err := r.Read(1)
	if err != nil {
		return false, err
	}
	if firstBit != 1 {
		err = r.reset(stateCopy)
		if err != nil {
			return false, err
		}
		return true, nil
	}
	// If all remainging bits are zero, there is no more rbsp data
	more := false
	for {
		b, err := r.Read(1)
		if err == io.EOF {
			break
		}
		if err != nil {
			return false, err
		}
		if b == 1 {
			more = true
			break
		}
	}
	err = r.reset(stateCopy)
	if err != nil {
		return false, nil
	}
	return more, nil
}

// reset EBSPReader based on copy of previous state
func (r *EBSPReader) reset(prevState EBSPReader) error {
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

// ReadRbspTrailingBits - read rbsp_traling_bits. Return false if wrong pattern
func (r *EBSPReader) ReadRbspTrailingBits() error {
	firstBit, err := r.Read(1)
	if err != nil {
		return err
	}
	if firstBit != 1 {
		return fmt.Errorf("RbspTrailingBits don't start with 1")
	}
	for {
		b, err := r.Read(1)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if b == 1 {
			return fmt.Errorf("Another 1 in RbspTrailingBits")
		}
	}
}
