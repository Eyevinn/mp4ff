package mp4

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
)

const charOffset = 0x60 // According to Section 8.4.2.3 of 14496-12

// MdhdBox - Media Header Box (mdhd - mandatory)
//
// Contained in : Media Box (mdia)
//
// Timescale defines the timescale used for this track.
// Language is a ISO-639-2/T language code stored as 1bit padding + [3]int5
type MdhdBox struct {
	Version          byte // Only version 0
	Flags            uint32
	CreationTime     uint64 // Typically not set
	ModificationTime uint64 // Typically not set
	Timescale        uint32 // Media timescale for this track
	Duration         uint64 // Trak duration, 0 for fragmented files
	Language         uint16 // Three-letter ISO-639-2/T language code
}

// DecodeMdhd - Decode box
func DecodeMdhd(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	versionAndFlags := binary.BigEndian.Uint32(data[0:4])
	version := byte(versionAndFlags >> 24)
	if version == 1 {
		return &MdhdBox{
			Version:          1,
			Flags:            versionAndFlags & flagsMask,
			CreationTime:     binary.BigEndian.Uint64(data[4:12]),
			ModificationTime: binary.BigEndian.Uint64(data[12:20]),
			Timescale:        binary.BigEndian.Uint32(data[20:24]),
			Duration:         binary.BigEndian.Uint64(data[24:32]),
			Language:         binary.BigEndian.Uint16(data[32:34]),
		}, nil
	} else if version == 0 {
		return &MdhdBox{
			Version:          0,
			Flags:            versionAndFlags & flagsMask,
			CreationTime:     uint64(binary.BigEndian.Uint32(data[4:8])),
			ModificationTime: uint64(binary.BigEndian.Uint32(data[8:12])),
			Timescale:        binary.BigEndian.Uint32(data[12:16]),
			Duration:         uint64(binary.BigEndian.Uint32(data[16:20])),
			Language:         binary.BigEndian.Uint16(data[20:22]),
		}, nil
	} else {
		return nil, errors.New("Unknown mdhd version")
	}
}

// GetLanguage - Get three-byte language string
func (m *MdhdBox) GetLanguage() string {
	a := (m.Language >> 10) & 0x1f
	b := (m.Language >> 5) & 0x1f
	c := m.Language & 0x1f
	return fmt.Sprintf("%c%c%c", a+charOffset, b+charOffset, c+charOffset)
}

// SetLanguage - Set three-byte language string
func (m *MdhdBox) SetLanguage(lang string) {
	var l uint16 = 0
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
	if m.Version == 1 {
		return 44
	}
	return 32 // m.Version = 0
}

// Encode - write box to w
func (m *MdhdBox) Encode(w io.Writer) error {
	err := EncodeHeader(m, w)
	if err != nil {
		return err
	}
	buf := makebuf(m)

	sw := NewSliceWriter(buf)
	versionAndFlags := (uint32(m.Version) << 24) + m.Flags
	sw.WriteUint32(versionAndFlags)
	if m.Version == 1 {
		sw.WriteUint64(m.CreationTime)
		sw.WriteUint64(m.ModificationTime)
		sw.WriteUint32(m.Timescale)
		sw.WriteUint64(m.Duration)
	} else {
		sw.WriteUint32(uint32(m.CreationTime))
		sw.WriteUint32(uint32(m.ModificationTime))
		sw.WriteUint32(m.Timescale)
		sw.WriteUint32(uint32(m.Duration))
	}
	sw.WriteUint16(m.Language)
	sw.WriteUint16(0)
	_, err = w.Write(buf)
	return err
}

func (m *MdhdBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, m, int(m.Version), m.Flags)
	bd.write(" - timeScale: %d", m.Timescale)
	bd.write(" - language: %s", m.GetLanguage())
	return bd.err
}
