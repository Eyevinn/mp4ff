package mp4

import (
	"io"
	"io/ioutil"
)

// MvhdBox - Movie Header Box (mvhd - mandatory)
//
// Contained in : Movie Box (‘moov’)
//
// Contains all media information (duration, ...).
//
// Duration is measured in "time units", and timescale defines the number of time units per second.
//
type MvhdBox struct {
	Version          byte
	Flags            uint32
	CreationTime     uint64
	ModificationTime uint64
	Timescale        uint32
	Duration         uint64
	NextTrackID      uint32
	Rate             Fixed32
	Volume           Fixed16
}

// CreateMvhd - create mvhd box with reasonable values
func CreateMvhd() *MvhdBox {
	return &MvhdBox{
		Timescale:   90000,      // Irrelevant since mdhd timescale is used
		NextTrackID: 2,          // There will typically only be one track
		Rate:        0x00010000, // This is 1.0
		Volume:      0x0100,     // Full volume
	}
}

// DecodeMvhd - box-specific decode
func DecodeMvhd(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	s := NewSliceReader(data)
	versionAndFlags := s.ReadUint32()
	version := byte(versionAndFlags >> 24)

	m := &MvhdBox{
		Version: version,
		Flags:   versionAndFlags & flagsMask,
	}

	if version == 1 {
		m.CreationTime = s.ReadUint64()
		m.ModificationTime = s.ReadUint64()
		m.Timescale = s.ReadUint32()
		m.Duration = s.ReadUint64()
	} else {
		m.CreationTime = uint64(s.ReadUint32())
		m.ModificationTime = uint64(s.ReadUint32())
		m.Timescale = s.ReadUint32()
		m.Duration = uint64(s.ReadUint32())
	}
	m.Rate = Fixed32(s.ReadUint32())
	m.Volume = Fixed16(s.ReadUint16())
	s.SkipBytes(10) // Reserved bytes
	s.SkipBytes(36) // Matrix patterndata
	s.SkipBytes(24) // Predefined 0
	m.NextTrackID = s.ReadUint32()
	return m, nil
}

// Type - return box type
func (b *MvhdBox) Type() string {
	return "mvhd"
}

// Size - return calculated size
func (b *MvhdBox) Size() uint64 {
	if b.Version == 1 {
		return 12 + 80 + 28 // Full header + variable part + fixed part
	}
	return 12 + 80 + 16 // Full header + variable part + fixed part
}

// Encode - write box to w
func (b *MvhdBox) Encode(w io.Writer) error {
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	buf := makebuf(b)
	sw := NewSliceWriter(buf)
	versionAndFlags := (uint32(b.Version) << 24) + b.Flags
	sw.WriteUint32(versionAndFlags)
	if b.Version == 0 {
		sw.WriteUint32(uint32(b.CreationTime))
		sw.WriteUint32(uint32(b.ModificationTime))
		sw.WriteUint32(b.Timescale)
		sw.WriteUint32(uint32(b.Duration))
	} else {
		sw.WriteUint64(b.CreationTime)
		sw.WriteUint64(b.ModificationTime)
		sw.WriteUint32(b.Timescale)
		sw.WriteUint64(b.Duration)
	}

	sw.WriteUint32(uint32(b.Rate))
	sw.WriteUint16(uint16(b.Volume))
	sw.WriteZeroBytes(10) // Reserved bytes
	sw.WriteUnityMatrix() // unity matrix according to 8.2.2.2
	sw.WriteZeroBytes(24) // Predefined 0
	sw.WriteUint32(b.NextTrackID)

	_, err = w.Write(buf)
	return err
}

func (b *MvhdBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, int(b.Version), b.Flags)
	bd.write(" - timeScale: %d", b.Timescale)
	bd.write(" - duration: %d", b.Duration)
	return bd.err
}
