package avc

import (
	"bytes"
	"errors"

	"github.com/edgeware/mp4ff/bits"
)

var ErrNoSliceHeader = errors.New("No slice header")
var ErrInvalidSliceType = errors.New("Invalid slice type")
var ErrTooFewBytesToParse = errors.New("Too few bytes to parse symbol")

type SliceType uint

func (s SliceType) String() string {
	switch s {
	case SLICE_I:
		return "I"
	case SLICE_P:
		return "P"
	case SLICE_B:
		return "B"
	default:
		return ""
	}
}

const (
	SLICE_P  = SliceType(0)
	SLICE_B  = SliceType(1)
	SLICE_I  = SliceType(2)
	SLICE_SP = SliceType(3)
	SLICE_SI = SliceType(4)
)

// GetSliceTypeFromNAL - parse slice header to get slice type in interval 0 to 4
func GetSliceTypeFromNAL(data []byte) (sliceType SliceType, err error) {

	if len(data) <= 1 {
		err = ErrTooFewBytesToParse
		return
	}

	nalType := GetNalType(data[0])
	switch nalType {
	case 1, 2, 5, 19:
		// slice_layer_without_partitioning_rbsp
		// slice_data_partition_a_layer_rbsp

	default:
		err = ErrNoSliceHeader
		return
	}
	r := bits.NewEBSPReader(bytes.NewReader((data[1:])))

	// first_mb_in_slice
	if _, err = r.ReadExpGolomb(); err != nil {
		return
	}

	// slice_type
	var st uint
	if st, err = r.ReadExpGolomb(); err != nil {
		return
	}
	sliceType = SliceType(st)
	if sliceType > 9 {
		err = ErrInvalidSliceType
		return
	}

	if sliceType >= 5 {
		sliceType -= 5 // The same type is repeated twice to tell if all slices in picture are the same
	}
	return
}
