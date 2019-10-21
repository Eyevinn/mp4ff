package mp4

import (
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
)

// Sample To Chunk Box (stsc - mandatory)
//
// Contained in : Sample Table box (stbl)
//
// Status: decoded
//
// A chunk contains samples. This table defines to which chunk a sample is associated.
// Each entry is defined by :
//
//   * first chunk : all chunks starting at this index up to the next first chunk have the same sample count/description
//   * samples per chunk : number of samples in the chunk
//   * description id : description (see the sample description box - stsd)
type StscBox struct {
	Version             byte
	Flags               [3]byte
	FirstChunk          []uint32
	SamplesPerChunk     []uint32
	SampleDescriptionID []uint32
}

func DecodeStsc(r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	b := &StscBox{
		Version:             data[0],
		Flags:               [3]byte{data[1], data[2], data[3]},
		FirstChunk:          []uint32{},
		SamplesPerChunk:     []uint32{},
		SampleDescriptionID: []uint32{},
	}
	ec := binary.BigEndian.Uint32(data[4:8])
	for i := 0; i < int(ec); i++ {
		fc := binary.BigEndian.Uint32(data[(8 + 12*i):(12 + 12*i)])
		spc := binary.BigEndian.Uint32(data[(12 + 12*i):(16 + 12*i)])
		sdi := binary.BigEndian.Uint32(data[(16 + 12*i):(20 + 12*i)])
		b.FirstChunk = append(b.FirstChunk, fc)
		b.SamplesPerChunk = append(b.SamplesPerChunk, spc)
		b.SampleDescriptionID = append(b.SampleDescriptionID, sdi)
	}
	return b, nil
}

func (b *StscBox) Type() string {
	return "stsc"
}

func (b *StscBox) Size() int {
	return BoxHeaderSize + 8 + len(b.FirstChunk)*12
}

func (b *StscBox) Dump() {
	fmt.Println("Sample to Chunk:")
	for i := range b.SamplesPerChunk {
		fmt.Printf(" #%d : %d samples per chunk starting @chunk #%d \n", i, b.SamplesPerChunk[i], b.FirstChunk[i])
	}
}

func (b *StscBox) Encode(w io.Writer) error {
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	buf := makebuf(b)
	buf[0] = b.Version
	buf[1], buf[2], buf[3] = b.Flags[0], b.Flags[1], b.Flags[2]
	binary.BigEndian.PutUint32(buf[4:], uint32(len(b.FirstChunk)))
	for i := range b.FirstChunk {
		binary.BigEndian.PutUint32(buf[8+12*i:], b.FirstChunk[i])
		binary.BigEndian.PutUint32(buf[12+12*i:], b.SamplesPerChunk[i])
		binary.BigEndian.PutUint32(buf[16+12*i:], b.SampleDescriptionID[i])
	}
	_, err = w.Write(buf)
	return err
}
