package iamf

import (
	"errors"

	"github.com/Eyevinn/mp4ff/bits"
)

var (
	ErrInvalidLeb = errors.New("Invalid Leb128")
)

// ReadLeb128 reads an unsigned Leb128 (Little Endian Base 128) encoded integer
func ReadLeb128(sr bits.SliceReader) (uint64, error) {
	var result uint64
	var shift uint
	for {
		if shift > 63 {
			return 0, ErrInvalidLeb
		}
		b := sr.ReadUint8()
		if sr.AccError() != nil {
			return 0, sr.AccError()
		}
		result |= uint64(b&0x7f) << shift
		if (b & 0x80) == 0 {
			break
		}
		shift += 7
	}
	return result, nil
}

// WriteLeb128 writes an unsigned Leb128 (Little Endian Base 128) encoded integer
func WriteLeb128(sw bits.SliceWriter, value uint64) {
	for {
		b := byte(value & 0x7f)
		value >>= 7
		if value != 0 {
			b |= 0x80
		}
		sw.WriteUint8(b)
		if value == 0 {
			break
		}
	}
}

// Leb128Size calculates the size in bytes of a Leb128 (Little Endian Base 128) encoded integer
func Leb128Size(value uint64) int {
	if value == 0 {
		return 1
	}
	size := 0
	for value > 0 {
		size++
		value >>= 7
	}
	return size
}

func GetCodecString(descriptorData []byte) (string, error) {
	return "", nil
}
