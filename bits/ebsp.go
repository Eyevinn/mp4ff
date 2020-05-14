package bits

import (
	"encoding/binary"
	"io"
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
	n   int  // current number of bits
	v   uint // current accumulated value
	pos int

	rd        io.Reader
	zeroCount int // Count number of zero bytes read
}

// MustRead - read n bits and panic if not possible
func (r *EBSPReader) MustRead(n int) uint {
	var err error

	for r.n <= n {
		r.v <<= 8
		var b uint8
		err = binary.Read(r.rd, binary.BigEndian, &b)
		if err != nil {
			panic("Reading error")
		}
		r.pos++
		if r.zeroCount == 2 {
			err = binary.Read(r.rd, binary.BigEndian, &b)
			if err != nil {
				panic("Reading error")
			}
			r.pos++
			r.zeroCount = 0
		} else {
			if b != 0 {
				r.zeroCount = 0
			} else {
				r.zeroCount++
			}
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

// MustReadExpGolomb - Read one unsigned exponential golomb code
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

// NrBytesRead - how many bytes read into parser
func (r *EBSPReader) NrBytesRead() int {
	return r.pos
}

// EBSP2rbsp - convert from EBSP to RBSP by removing escape 0x03 after two 0x00
func EBSP2rbsp(ebsp []byte) []byte {
	zeroCount := 0
	output := make([]byte, 0, len(ebsp))
	for i := 0; i < len(ebsp); i++ {
		b := ebsp[i]
		if zeroCount == 2 && b == 3 {
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
