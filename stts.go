package mp4

import (
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"time"
)

// Decoding Time to Sample Box (stts - mandatory)
//
// Contained in : Sample Table box (stbl)
//
// Status: decoded
//
// This table contains the duration in time units for each sample.
//
//   * sample count : the number of consecutive samples having the same duration
//   * time delta : duration in time units
type SttsBox struct {
	Version         byte
	Flags           [3]byte
	SampleCount     []uint32
	SampleTimeDelta []uint32
}

func DecodeStts(r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	b := &SttsBox{
		Version:         data[0],
		Flags:           [3]byte{data[1], data[2], data[3]},
		SampleCount:     []uint32{},
		SampleTimeDelta: []uint32{},
	}
	ec := binary.BigEndian.Uint32(data[4:8])
	for i := 0; i < int(ec); i++ {
		s_count := binary.BigEndian.Uint32(data[(8 + 8*i):(12 + 8*i)])
		s_delta := binary.BigEndian.Uint32(data[(12 + 8*i):(16 + 8*i)])
		b.SampleCount = append(b.SampleCount, s_count)
		b.SampleTimeDelta = append(b.SampleTimeDelta, s_delta)
	}
	return b, nil
}

func (b *SttsBox) Type() string {
	return "stts"
}

func (b *SttsBox) Size() int {
	return BoxHeaderSize + 8 + len(b.SampleCount)*8
}

// GetTimeCode returns the timecode (duration since the beginning of the media)
// of the beginning of a sample
func (b *SttsBox) GetTimeCode(sample, timescale uint32) time.Duration {
	sample--
	var units uint32
	i := 0
	for sample > 0 && i < len(b.SampleCount) {
		if sample >= b.SampleCount[i] {
			units += b.SampleCount[i] * b.SampleTimeDelta[i]
			sample -= b.SampleCount[i]
		} else {
			units += sample * b.SampleTimeDelta[i]
			sample = 0
		}
		i++
	}
	return time.Second * time.Duration(units) / time.Duration(timescale)
}

func (b *SttsBox) Dump() {
	fmt.Println("Time to sample:")
	for i := range b.SampleCount {
		fmt.Printf(" #%d : %d samples with duration %d units\n", i, b.SampleCount[i], b.SampleTimeDelta[i])
	}
}

func (b *SttsBox) Encode(w io.Writer) error {
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	buf := makebuf(b)
	buf[0] = b.Version
	buf[1], buf[2], buf[3] = b.Flags[0], b.Flags[1], b.Flags[2]
	binary.BigEndian.PutUint32(buf[4:], uint32(len(b.SampleCount)))
	for i := range b.SampleCount {
		binary.BigEndian.PutUint32(buf[8+8*i:], b.SampleCount[i])
		binary.BigEndian.PutUint32(buf[12+8*i:], b.SampleTimeDelta[i])
	}
	_, err = w.Write(buf)
	return err
}
