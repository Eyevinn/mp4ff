package mp4

import (
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
)

// Edit List Box (elst - optional)
//
// Contained in : Edit Box (edts)
//
// Status: version 0 decoded. version 1 not supported
type ElstBox struct {
	Version                             byte
	Flags                               [3]byte
	SegmentDuration, MediaTime          []uint32 // should be uint32/int32 for version 0 and uint64/int32 for version 1
	MediaRateInteger, MediaRateFraction []uint16 // should be int16
}

func DecodeElst(r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	b := &ElstBox{
		Version:           data[0],
		Flags:             [3]byte{data[1], data[2], data[3]},
		SegmentDuration:   []uint32{},
		MediaTime:         []uint32{},
		MediaRateInteger:  []uint16{},
		MediaRateFraction: []uint16{},
	}
	ec := binary.BigEndian.Uint32(data[4:8])
	for i := 0; i < int(ec); i++ {
		sd := binary.BigEndian.Uint32(data[(8 + 12*i):(12 + 12*i)])
		mt := binary.BigEndian.Uint32(data[(12 + 12*i):(16 + 12*i)])
		mri := binary.BigEndian.Uint16(data[(16 + 12*i):(18 + 12*i)])
		mrf := binary.BigEndian.Uint16(data[(18 + 12*i):(20 + 12*i)])
		b.SegmentDuration = append(b.SegmentDuration, sd)
		b.MediaTime = append(b.MediaTime, mt)
		b.MediaRateInteger = append(b.MediaRateInteger, mri)
		b.MediaRateFraction = append(b.MediaRateFraction, mrf)
	}
	return b, nil
}

func (b *ElstBox) Type() string {
	return "elst"
}

func (b *ElstBox) Size() int {
	return BoxHeaderSize + 8 + len(b.SegmentDuration)*12
}

func (b *ElstBox) Dump() {
	fmt.Println("Segment Duration:")
	for i, d := range b.SegmentDuration {
		fmt.Printf(" #%d: %d units\n", i, d)
	}
}

func (b *ElstBox) Encode(w io.Writer) error {
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	buf := make([]byte, b.Size()-BoxHeaderSize)
	buf[0] = b.Version
	buf[1], buf[2], buf[3] = b.Flags[0], b.Flags[1], b.Flags[2]
	binary.BigEndian.PutUint32(buf[4:], uint32(len(b.SegmentDuration)))
	for i := range b.SegmentDuration {
		binary.BigEndian.PutUint32(buf[8+12*i:], b.SegmentDuration[i])
		binary.BigEndian.PutUint32(buf[12+12*i:], b.MediaTime[i])
		binary.BigEndian.PutUint16(buf[16+12*i:], b.MediaRateInteger[i])
		binary.BigEndian.PutUint16(buf[18+12*i:], b.MediaRateFraction[i])
	}
	_, err = w.Write(buf)
	return err
}
