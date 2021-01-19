package mp4

import (
	"encoding/binary"
	"io"
	"io/ioutil"
)

// StszBox - Sample Size Box (stsz - mandatory)
//
// Contained in : Sample Table box (stbl)
//
// For each track, either stsz of the more compact stz2 must be present. stz2 variant is not supported.
//
// This table lists the size of each sample. If all samples have the same size, it can be defined in the
// SampleUniformSize attribute.
type StszBox struct {
	Version           byte
	Flags             uint32
	SampleUniformSize uint32
	SampleNumber      uint32
	SampleSize        []uint32
}

// DecodeStsz - box-specific decode
func DecodeStsz(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	versionAndFlags := binary.BigEndian.Uint32(data[0:4])

	b := &StszBox{
		Version:           byte(versionAndFlags >> 24),
		Flags:             versionAndFlags & flagsMask,
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

// Type - box-specific type
func (b *StszBox) Type() string {
	return "stsz"
}

// Size - box-specific size
func (b *StszBox) Size() uint64 {
	return uint64(boxHeaderSize + 12 + len(b.SampleSize)*4)
}

// GetSampleSize returns the size (in bytes) of a sample
func (b *StszBox) GetSampleSize(i int) uint32 {
	if i > len(b.SampleSize) {
		return b.SampleUniformSize
	}
	return b.SampleSize[i-1]
}

// Encode - write box to w
func (b *StszBox) Encode(w io.Writer) error {
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	buf := makebuf(b)
	sw := NewSliceWriter(buf)
	versionAndFlags := (uint32(b.Version) << 24) + b.Flags
	sw.WriteUint32(versionAndFlags)
	sw.WriteUint32(b.SampleUniformSize)
	if len(b.SampleSize) == 0 {
		sw.WriteUint32(b.SampleNumber)
	} else {
		sw.WriteUint32(uint32(len(b.SampleSize)))
		for i := range b.SampleSize {
			sw.WriteUint32(b.SampleSize[i])
		}
	}
	_, err = w.Write(buf)
	return err
}

func (b *StszBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, int(b.Version), b.Flags)
	if b.SampleNumber == 0 { // No samples
		return bd.err
	}
	if len(b.SampleSize) == 0 {
		bd.write(" - sampleSize: %d", b.SampleUniformSize)
		bd.write(" - sampleCount: %d", b.SampleNumber)
	} else {
		bd.write(" - sampleCount: %d", b.SampleNumber)
	}
	level := getInfoLevel(b, specificBoxLevels)
	if level >= 1 {
		for i := range b.SampleSize {
			bd.write(" - sample[%d] size=%d", i+1, b.SampleSize[i])
		}
	}
	return bd.err
}
