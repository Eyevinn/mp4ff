package mp4

import (
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
)

// Sample Size Box (stsz - mandatory)
//
// Contained in : Sample Table box (stbl)
//
// Status : decoded
//
// For each track, either stsz of the more compact stz2 must be present. stz2 variant is not supported.
//
// This table lists the size of each sample. If all samples have the same size, it can be defined in the
// SampleUniformSize attribute.
type StszBox struct {
	Version           byte
	Flags             [3]byte
	SampleUniformSize uint32
	SampleNumber      uint32
	SampleSize        []uint32
}

func DecodeStsz(r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	b := &StszBox{
		Version:           data[0],
		Flags:             [3]byte{data[1], data[2], data[3]},
		SampleUniformSize: binary.BigEndian.Uint32(data[4:8]),
		SampleNumber:      binary.BigEndian.Uint32(data[8:12]),
		SampleSize:        []uint32{},
	}
	if len(data) > 12 {
		for i := 0; i < int(b.SampleNumber); i++ {
			sz := binary.BigEndian.Uint32(data[(12 + 4*i):(16 + 4*i)])
			b.SampleSize = append(b.SampleSize, sz)
		}
	}
	return b, nil
}

func (b *StszBox) Type() string {
	return "stsz"
}

func (b *StszBox) Size() int {
	return BoxHeaderSize + 12 + len(b.SampleSize)*4
}

func (b *StszBox) Dump() {
	if len(b.SampleSize) == 0 {
		fmt.Printf("Samples : %d total samples\n", b.SampleNumber)
	} else {
		fmt.Printf("Samples : %d total samples\n", len(b.SampleSize))
	}
}

// GetSampleSize returns the size (in bytes) of a sample
func (b *StszBox) GetSampleSize(i int) uint32 {
	if i > len(b.SampleSize) {
		return b.SampleUniformSize
	}
	return b.SampleSize[i-1]
}

func (b *StszBox) Encode(w io.Writer) error {
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	buf := makebuf(b)
	buf[0] = b.Version
	buf[1], buf[2], buf[3] = b.Flags[0], b.Flags[1], b.Flags[2]
	binary.BigEndian.PutUint32(buf[4:], b.SampleUniformSize)
	if len(b.SampleSize) == 0 {
		binary.BigEndian.PutUint32(buf[8:], b.SampleNumber)
	} else {
		binary.BigEndian.PutUint32(buf[8:], uint32(len(b.SampleSize)))
		for i := range b.SampleSize {
			binary.BigEndian.PutUint32(buf[12+4*i:], b.SampleSize[i])
		}
	}
	_, err = w.Write(buf)
	return err
}
