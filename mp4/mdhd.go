package mp4

import (
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"time"
)

// Media Header Box (mdhd - mandatory)
//
// Contained in : Media Box (mdia)
//
// Status : only version 0 is decoded. version 1 is not supported
//
// Timescale defines the timescale used for tracks.
// Language is a ISO-639-2/T language code stored as 1bit padding + [3]int5
type MdhdBox struct {
	Version          byte
	Flags            [3]byte
	CreationTime     uint32
	ModificationTime uint32
	Timescale        uint32
	Duration         uint32
	Language         uint16
}

func DecodeMdhd(r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return &MdhdBox{
		Version:          data[0],
		Flags:            [3]byte{data[1], data[2], data[3]},
		CreationTime:     binary.BigEndian.Uint32(data[4:8]),
		ModificationTime: binary.BigEndian.Uint32(data[8:12]),
		Timescale:        binary.BigEndian.Uint32(data[12:16]),
		Duration:         binary.BigEndian.Uint32(data[16:20]),
		Language:         binary.BigEndian.Uint16(data[20:22]),
	}, nil
}

func (b *MdhdBox) Type() string {
	return "mdhd"
}

func (b *MdhdBox) Size() int {
	return BoxHeaderSize + 24
}

func (b *MdhdBox) Dump() {
	fmt.Printf("Media Header:\n Timescale: %d units/sec\n Duration: %d units (%s)\n", b.Timescale, b.Duration, time.Duration(b.Duration/b.Timescale)*time.Second)

}

func (b *MdhdBox) Encode(w io.Writer) error {
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
	binary.BigEndian.PutUint16(buf[20:], b.Language)
	_, err = w.Write(buf)
	return err
}
