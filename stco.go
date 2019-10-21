package mp4

import (
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
)

// Chunk Offset Box (stco - mandatory)
//
// Contained in : Sample Table box (stbl)
//
// Status: decoded
//
// This is the 32bits version of the box, the 64bits version (co64) is not decoded.
//
// The table contains the offsets (starting at the beginning of the file) for each chunk of data for the current track.
// A chunk contains samples, the table defining the allocation of samples to each chunk is stsc.
type StcoBox struct {
	Version     byte
	Flags       [3]byte
	ChunkOffset []uint32
}

func DecodeStco(r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	b := &StcoBox{
		Version:     data[0],
		Flags:       [3]byte{data[1], data[2], data[3]},
		ChunkOffset: []uint32{},
	}
	ec := binary.BigEndian.Uint32(data[4:8])
	for i := 0; i < int(ec); i++ {
		chunk := binary.BigEndian.Uint32(data[(8 + 4*i):(12 + 4*i)])
		b.ChunkOffset = append(b.ChunkOffset, chunk)
	}
	return b, nil
}

func (b *StcoBox) Type() string {
	return "stco"
}

func (b *StcoBox) Size() int {
	return BoxHeaderSize + 8 + len(b.ChunkOffset)*4
}

func (b *StcoBox) Dump() {
	fmt.Println("Chunk byte offsets:")
	for i, o := range b.ChunkOffset {
		fmt.Printf(" #%d : starts at %d\n", i, o)
	}
}

func (b *StcoBox) Encode(w io.Writer) error {
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	buf := makebuf(b)
	buf[0] = b.Version
	buf[1], buf[2], buf[3] = b.Flags[0], b.Flags[1], b.Flags[2]
	binary.BigEndian.PutUint32(buf[4:], uint32(len(b.ChunkOffset)))
	for i := range b.ChunkOffset {
		binary.BigEndian.PutUint32(buf[8+4*i:], b.ChunkOffset[i])
	}
	_, err = w.Write(buf)
	return err
}
