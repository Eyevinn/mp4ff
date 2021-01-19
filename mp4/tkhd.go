package mp4

import (
	"io"
	"io/ioutil"
)

// TkhdBox - Track Header Box (tkhd - mandatory)
//
// This box describes the track. Duration is measured in time units (according to the time scale
// defined in the movie header box). Duration is 0 for fragmented files.
//
// Volume (relevant for audio tracks) is a fixed point number (8 bits + 8 bits). Full volume is 1.0.
// Width and Height (relevant for video tracks) are fixed point numbers (16 bits + 16 bits).
// Video pixels are not necessarily square.
type TkhdBox struct {
	Version          byte
	Flags            uint32
	CreationTime     uint64
	ModificationTime uint64
	TrackID          uint32
	Duration         uint64
	Layer            int16
	AlternateGroup   int16 // should be int16
	Volume           Fixed16
	Width, Height    Fixed32
}

// CreateTkhd - create tkhd box with common settings
func CreateTkhd() *TkhdBox {
	return &TkhdBox{
		Version: 0,
		Flags:   0x000007,      // Enabled, inMovie, inPreview set
		TrackID: DefaultTrakID, // Typically just have one track
	}
}

// DecodeTkhd - box-specific decode
func DecodeTkhd(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	s := NewSliceReader(data)
	versionAndFlags := s.ReadUint32()
	version := byte(versionAndFlags >> 24)
	flags := versionAndFlags & flagsMask

	t := &TkhdBox{
		Version: version,
		Flags:   flags,
	}

	if version == 1 {
		t.CreationTime = s.ReadUint64()
		t.ModificationTime = s.ReadUint64()
		t.TrackID = s.ReadUint32()
		s.SkipBytes(4) // Reserved = 0
		t.Duration = s.ReadUint64()
	} else {
		t.CreationTime = uint64(s.ReadUint32())
		t.ModificationTime = uint64(s.ReadUint32())
		t.TrackID = s.ReadUint32()
		s.SkipBytes(4) // Reserved = 0
		t.Duration = uint64(s.ReadUint32())
	}
	s.SkipBytes(8) // Reserved 8 x 0
	t.Layer = s.ReadInt16()
	t.AlternateGroup = s.ReadInt16()
	t.Volume = Fixed16(s.ReadInt16())
	s.SkipBytes(2)
	s.SkipBytes(36) // 3x3 matrixdata
	t.Width = Fixed32(s.ReadUint32())
	t.Height = Fixed32(s.ReadUint32())

	return t, nil
}

// Type - box type
func (b *TkhdBox) Type() string {
	return "tkhd"
}

// Size - calculated size of box
func (b *TkhdBox) Size() uint64 {
	if b.Version == 1 {
		return 104
	}
	return 92
}

// Encode - write box to w
func (b *TkhdBox) Encode(w io.Writer) error {
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
		sw.WriteUint32(b.TrackID)
		sw.WriteZeroBytes(4) // Reserved
		sw.WriteUint32(uint32(b.Duration))
	} else {
		sw.WriteUint64(b.CreationTime)
		sw.WriteUint64(b.ModificationTime)
		sw.WriteUint32(b.TrackID)
		sw.WriteZeroBytes(4) // Reserved
		sw.WriteUint64(b.Duration)
	}
	sw.WriteZeroBytes(8) // Reserved
	sw.WriteInt16(b.Layer)
	sw.WriteInt16(b.AlternateGroup)
	sw.WriteUint16(uint16(b.Volume))
	sw.WriteZeroBytes(2)  // Reserved
	sw.WriteUnityMatrix() // unity matrix according to 8.3.2.2
	sw.WriteUint32(uint32(b.Width))
	sw.WriteUint32(uint32(b.Height))

	_, err = w.Write(buf)

	return err
}

func (b *TkhdBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, int(b.Version), b.Flags)
	bd.write(" - trackID: %d", b.TrackID)
	if b.Width != 0 && b.Height != 0 { // These are Fixed32 values
		bd.write(" - Width: %s, Height: %s", b.Width, b.Height)
	}
	return bd.err
}
