package mp4

import (
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"time"
)

// MvhdBox - Movie Header Box (mvhd - mandatory)
//
// Contained in : Movie Box (‘moov’)
//
// Status: version 0 is partially decoded. version 1 is not supported
//
// Contains all media information (duration, ...).
//
// Duration is measured in "time units", and timescale defines the number of time units per second.
//
// Only version 0 is decoded.
type MvhdBox struct {
	Version          byte
	Flags            [3]byte
	CreationTime     uint32
	ModificationTime uint32
	Timescale        uint32
	Duration         uint32
	NextTrackID      uint32
	Rate             Fixed32
	Volume           Fixed16
	notDecoded       []byte
}

// DecodeMvhd - box-specific decode
func DecodeMvhd(size uint64, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return &MvhdBox{
		Version:          data[0],
		Flags:            [3]byte{data[1], data[2], data[3]},
		CreationTime:     binary.BigEndian.Uint32(data[4:8]),
		ModificationTime: binary.BigEndian.Uint32(data[8:12]),
		Timescale:        binary.BigEndian.Uint32(data[12:16]),
		Duration:         binary.BigEndian.Uint32(data[16:20]),
		Rate:             fixed32(data[20:24]),
		Volume:           fixed16(data[24:26]),
		notDecoded:       data[26:],
	}, nil
}

// Type - return box type
func (b *MvhdBox) Type() string {
	return "mvhd"
}

// Size - return calculated size
func (b *MvhdBox) Size() uint64 {
	return uint64(boxHeaderSize + 26 + len(b.notDecoded))
}

// Dump - write box details
func (b *MvhdBox) Dump() {
	fmt.Printf("Movie Header:\n Timescale: %d units/sec\n Duration: %d units (%s)\n Rate: %s\n Volume: %s\n", b.Timescale, b.Duration, time.Duration(b.Duration/b.Timescale)*time.Second, b.Rate, b.Volume)
}

// Encode - write box to w
func (b *MvhdBox) Encode(w io.Writer) error {
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	buf := makebuf(b)
	buf[0] = b.Version
	buf[1], buf[2], buf[3] = b.Flags[0], b.Flags[1], b.Flags[2]
	binary.BigEndian.PutUint32(buf[4:], b.CreationTime)
	binary.BigEndian.PutUint32(buf[8:], b.ModificationTime)
	binary.BigEndian.PutUint32(buf[12:], b.Timescale)
	binary.BigEndian.PutUint32(buf[16:], b.Duration)
	binary.BigEndian.PutUint32(buf[20:], uint32(b.Rate))
	binary.BigEndian.PutUint16(buf[24:], uint16(b.Volume))
	copy(buf[26:], b.notDecoded)
	_, err = w.Write(buf)
	return err
}
