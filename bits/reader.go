package bits

import (
	"fmt"
	"io"
)

// Reader is a bit reader that stops reading at first error and stores it.
// First error can be fetched usiin AccError().
type Reader struct {
	rd    io.Reader
	br    io.ByteReader // reads the bytes of rd without allocating
	data  []byte        // the bytes to read when created by NewReaderFromSlice
	off   int           // offset of the next byte in data
	slice bool          // created by NewReaderFromSlice
	err   error
	n     int  // current number of bits
	value uint // current accumulated value
	pos   int  // current position in reader (in bytes)
}

// AccError - accumulated error is first error that occurred
func (r *Reader) AccError() error {
	return r.err
}

// NewReader return a new Reader that accumulates errors.
// No byte read allocates. If rd is not an io.ByteReader, such as a bytes.Reader, the reader allocates once.
func NewReader(rd io.Reader) *Reader {
	return &Reader{
		rd:  rd,
		br:  newByteReader(rd),
		pos: -1,
	}
}

// NewReaderFromSlice returns a new Reader of data that accumulates errors. It reads without allocating.
func NewReaderFromSlice(data []byte) *Reader {
	return &Reader{
		data:  data,
		slice: true,
		pos:   -1,
	}
}

// readByte reads the next byte.
func (r *Reader) readByte() (byte, error) {
	if r.slice {
		if r.off >= len(r.data) {
			return 0, io.EOF
		}
		b := r.data[r.off]
		r.off++
		return b, nil
	}
	return r.br.ReadByte()
}

// byteReader reads the bytes of an io.Reader that is not an io.ByteReader one at a time, without reading ahead, so
// that the io.Reader can still be used and seeked directly.
type byteReader struct {
	rd  io.Reader
	buf [1]byte
}

// newByteReader returns rd as an io.ByteReader, wrapping it if it is not one.
func newByteReader(rd io.Reader) io.ByteReader {
	if br, ok := rd.(io.ByteReader); ok {
		return br
	}
	return &byteReader{rd: rd}
}

// ReadByte reads one byte. The buffer is a field, since a local one would escape and allocate for every byte.
func (b *byteReader) ReadByte() (byte, error) {
	_, err := io.ReadFull(b.rd, b.buf[:])
	return b.buf[0], err
}

// Read - read n bits. Return 0, if error now or previously
func (r *Reader) Read(n int) uint {
	if r.err != nil {
		return 0
	}

	for r.n < n {
		r.value <<= 8
		newByte, err := r.readByte()
		if err != nil {
			r.err = err
			return 0
		}
		r.pos++
		r.value |= uint(newByte)

		r.n += 8
	}
	value := r.value >> uint(r.n-n)

	r.n -= n
	r.value &= Mask(r.n)

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

// ReadExpGolomb reads one unsigned exponential Golomb code ue(v). Returns 0 if error now or previously.
func (r *Reader) ReadExpGolomb() uint {
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
	res := uint(1<<leadingZeroBits) - 1
	endBits := r.Read(leadingZeroBits)
	if r.err != nil {
		return 0
	}
	return res + endBits
}

// ReadSignedGolomb reads one signed exponential Golomb code se(v). Returns 0 if error now or previously.
func (r *Reader) ReadSignedGolomb() int {
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

// ReadRemainingBytes reads remaining bytes if byte-aligned. Returns nil if error now or previously.
func (r *Reader) ReadRemainingBytes() []byte {
	if r.err != nil {
		return nil
	}
	if r.n != 0 {
		r.err = fmt.Errorf("%d bit instead of byte alignment when reading remaining bytes", r.n)
		return nil
	}
	if r.slice {
		rest := make([]byte, len(r.data)-r.off)
		copy(rest, r.data[r.off:])
		r.off = len(r.data)
		return rest
	}
	rest, err := io.ReadAll(r.rd)
	if err != nil {
		r.err = err
		return nil
	}
	return rest
}

// NrBytesRead returns how many bytes read into parser.
func (r *Reader) NrBytesRead() int {
	return r.pos + 1 // Starts at -1
}

// NrBitsRead returns total number of bits read into parser.
func (r *Reader) NrBitsRead() int {
	nrBits := r.NrBytesRead() * 8
	if r.NrBitsReadInCurrentByte() != 8 {
		nrBits += r.NrBitsReadInCurrentByte() - 8
	}
	return nrBits
}

// NrBitsReadInCurrentByte returns number of bits read in current byte.
func (r *Reader) NrBitsReadInCurrentByte() int {
	return 8 - r.n
}

// ByteAlign aligns the reader to the next byte boundary by discarding
// any remaining bits in the current byte. This is commonly used in
// multimedia formats where data structures need to be byte-aligned.
func (r *Reader) ByteAlign() {
	if r.err != nil {
		return
	}
	if r.n > 0 {
		// Discard remaining bits in current byte to align to byte boundary
		r.n = 0
		r.value = 0
	}
}
