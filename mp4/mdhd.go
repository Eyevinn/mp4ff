package mp4

import (
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"time"

	log "github.com/sirupsen/logrus"
)

const charOffset = 0x60 // According to Setion 8.4.2.3 of 14496-12

// MdhdBox - Media Header Box (mdhd - mandatory)
//
// Contained in : Media Box (mdia)
//
// Status : only version 0 is decoded. version 1 is not supported
//
// Timescale defines the timescale used for tracks.
// Language is a ISO-639-2/T language code stored as 1bit padding + [3]int5
type MdhdBox struct {
	Version          byte
	Flags            uint32
	CreationTime     uint32
	ModificationTime uint32
	Timescale        uint32
	Duration         uint32
	Language         uint16
}

// DecodeMdhd - Decode box
func DecodeMdhd(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	versionAndFlags := binary.BigEndian.Uint32(data[0:4])
	version := byte(versionAndFlags >> 24)
	if version != 0 {
		log.Fatalf("Only version 0 of mdhd supported")
	}
	return &MdhdBox{
		Version:          version,
		Flags:            versionAndFlags & flagsMask,
		CreationTime:     binary.BigEndian.Uint32(data[4:8]),
		ModificationTime: binary.BigEndian.Uint32(data[8:12]),
		Timescale:        binary.BigEndian.Uint32(data[12:16]),
		Duration:         binary.BigEndian.Uint32(data[16:20]),
		Language:         binary.BigEndian.Uint16(data[20:22]),
	}, nil
}

// GetLanguage - Get thee-byte language string
func (m *MdhdBox) GetLanguage() string {
	a := (m.Language >> 10) & 0x1f
	b := (m.Language >> 5) & 0x1f
	c := m.Language & 0x1f
	return fmt.Sprintf("%c%c%c", a+charOffset, b+charOffset, c+charOffset)
}

// SetLanguage - Set three-byte language string
func (m *MdhdBox) SetLanguage(lang string) {
	var l uint16 = 0 //TODO. Fix this
	for i, c := range lang {
		l += uint16(((c - charOffset) & 0x1f) << (5 * (2 - i)))
	}
	m.Language = l
}

// Type - box type
func (m *MdhdBox) Type() string {
	return "mdhd"
}

// Size - calculated size of box
func (m *MdhdBox) Size() uint64 {
	return 32 // For version 0
}

// Dump - print box info
func (m *MdhdBox) Dump() {
	fmt.Printf("Media Header:\n Timescale: %d units/sec\n Duration: %d units (%s)\n",
		m.Timescale, m.Duration, time.Duration(m.Duration/m.Timescale)*time.Second)
}

// Encode - write box to w
func (m *MdhdBox) Encode(w io.Writer) error {
	err := EncodeHeader(m, w)
	if err != nil {
		return err
	}
	buf := makebuf(m)
	versionAndFlags := (uint32(m.Version) << 24) + m.Flags
	binary.BigEndian.PutUint32(buf[0:], versionAndFlags)
	binary.BigEndian.PutUint32(buf[4:], m.CreationTime)
	binary.BigEndian.PutUint32(buf[8:], m.ModificationTime)
	binary.BigEndian.PutUint32(buf[12:], m.Timescale)
	binary.BigEndian.PutUint32(buf[16:], m.Duration)
	binary.BigEndian.PutUint16(buf[20:], m.Language)
	binary.BigEndian.PutUint16(buf[22:], 0)
	_, err = w.Write(buf)
	return err
}
