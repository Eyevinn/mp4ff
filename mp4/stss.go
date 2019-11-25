package mp4

import (
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
)

// StssBox - Sync Sample Box (stss - optional)
//
// Contained in : Sample Table box (stbl)
//
// Status: decoded
//
// This lists all sync samples (key frames for video tracks) in the data. If absent, all samples are sync samples.
type StssBox struct {
	Version      byte
	Flags        [3]byte
	SampleNumber []uint32
}

// DecodeStss - box-specific decode
func DecodeStss(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	b := &StssBox{
		Version:      data[0],
		Flags:        [3]byte{data[1], data[2], data[3]},
		SampleNumber: []uint32{},
	}
	ec := binary.BigEndian.Uint32(data[4:8])
	for i := 0; i < int(ec); i++ {
		sample := binary.BigEndian.Uint32(data[(8 + 4*i):(12 + 4*i)])
		b.SampleNumber = append(b.SampleNumber, sample)
	}
	return b, nil
}

// Type - box-specfic type
func (b *StssBox) Type() string {
	return "stss"
}

// Size - box-specfic size
func (b *StssBox) Size() uint64 {
	return uint64(boxHeaderSize + 8 + len(b.SampleNumber)*4)
}

// Dump - box-specific dump
func (b *StssBox) Dump() {
	fmt.Println("Key frames:")
	for i, n := range b.SampleNumber {
		fmt.Printf(" #%d : sample #%d\n", i, n)
	}
}

// Encode - box-specific encode
func (b *StssBox) Encode(w io.Writer) error {
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	buf := makebuf(b)
	buf[0] = b.Version
	buf[1], buf[2], buf[3] = b.Flags[0], b.Flags[1], b.Flags[2]
	binary.BigEndian.PutUint32(buf[4:], uint32(len(b.SampleNumber)))
	for i := range b.SampleNumber {
		binary.BigEndian.PutUint32(buf[8+4*i:], b.SampleNumber[i])
	}
	_, err = w.Write(buf)
	return err
}
