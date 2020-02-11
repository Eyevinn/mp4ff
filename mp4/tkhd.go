package mp4

import (
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
)

// TkhdBox - Track Header Box (tkhd - mandatory)
//
// Status : only version 0 is decoded. version 1 is not supported
//
// This box describes the track. Duration is measured in time units (according to the time scale
// defined in the movie header box).
//
// Volume (relevant for audio tracks) is a fixed point number (8 bits + 8 bits). Full volume is 1.0.
// Width and Height (relevant for video tracks) are fixed point numbers (16 bits + 16 bits).
// Video pixels are not necessarily square.
type TkhdBox struct {
	Version          byte
	Flags            uint32
	CreationTime     uint32
	ModificationTime uint32
	TrackID          uint32
	Duration         uint32
	Layer            uint16
	AlternateGroup   uint16 // should be int16
	Volume           Fixed16
	Matrix           []byte
	Width, Height    Fixed32
}

// DecodeTkhd - box-specific decode
func DecodeTkhd(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	versionAndFlags := binary.BigEndian.Uint32(data[0:4])
	version := byte(versionAndFlags >> 24)
	return &TkhdBox{
		Version:          version,
		Flags:            versionAndFlags & flagsMask,
		CreationTime:     binary.BigEndian.Uint32(data[4:8]),
		ModificationTime: binary.BigEndian.Uint32(data[8:12]),
		TrackID:          binary.BigEndian.Uint32(data[12:16]),
		Volume:           fixed16(data[36:38]),
		Duration:         binary.BigEndian.Uint32(data[20:24]),
		Layer:            binary.BigEndian.Uint16(data[32:34]),
		AlternateGroup:   binary.BigEndian.Uint16(data[34:36]),
		Matrix:           data[40:76],
		Width:            fixed32(data[76:80]),
		Height:           fixed32(data[80:84]),
	}, nil
}

// Type - box type
func (b *TkhdBox) Type() string {
	return "tkhd"
}

// Size - calculated size of box
func (b *TkhdBox) Size() uint64 {
	return uint64(boxHeaderSize + 84)
}

// Encode - write box to w
func (b *TkhdBox) Encode(w io.Writer) error {
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	buf := makebuf(b)
	versionAndFlags := (uint32(b.Version) << 24) + b.Flags
	binary.BigEndian.PutUint32(buf[0:], versionAndFlags)
	binary.BigEndian.PutUint32(buf[4:], b.CreationTime)
	binary.BigEndian.PutUint32(buf[8:], b.ModificationTime)
	binary.BigEndian.PutUint32(buf[12:], b.TrackID)
	binary.BigEndian.PutUint32(buf[20:], b.Duration)
	binary.BigEndian.PutUint16(buf[32:], b.Layer)
	binary.BigEndian.PutUint16(buf[34:], b.AlternateGroup)
	putFixed16(buf[36:], b.Volume)
	copy(buf[40:], b.Matrix)
	putFixed32(buf[76:], b.Width)
	putFixed32(buf[80:], b.Height)
	_, err = w.Write(buf)
	return err
}

// Dump - print box info
func (b *TkhdBox) Dump() {
	fmt.Println("Track Header:")
	fmt.Printf(" Duration: %d units\n WxH: %sx%s\n", b.Duration, b.Width, b.Height)
}
