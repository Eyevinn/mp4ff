package mp4

import (
	"io"
	"time"

	"github.com/Eyevinn/mp4ff/bits"
)

// MvhdBox - Movie Header Box (mvhd - mandatory)
//
// Contained in : Movie Box (‘moov’)
//
// Contains all media information (duration, ...).
//
// Duration is measured in "time units", and timescale defines the number of time units per second.
type MvhdBox struct {
	Version          byte
	Flags            uint32
	CreationTime     uint64 // Seconds since 1904-01-01
	ModificationTime uint64 // Seconds since 1904-01-01
	Timescale        uint32
	Duration         uint64
	NextTrackID      uint32
	Rate             Fixed32
	Volume           Fixed16
	// Matrix - transformation matrix {a, b, u, c, d, v, x, y, w} (16.16 fixed point, u/v/w 2.30).
	// The zero value is encoded as the unity matrix.
	Matrix [9]int32
	// QuickTime overlays these times, in the movie timescale, on the six pre_defined fields,
	// which are 0 in ISO files.
	QuickTimePreviewTime       uint32
	QuickTimePreviewDuration   uint32
	QuickTimePosterTime        uint32
	QuickTimeSelectionTime     uint32
	QuickTimeSelectionDuration uint32
	QuickTimeCurrentTime       uint32
}

// EpochDiffS is the difference in seconds between Jan 1, 1904 and Jan 1, 1970
const EpochDiffS = int64((66*365 + 17) * 24 * 3600)

// CreateMvhd - create mvhd box with reasonable values
func CreateMvhd() *MvhdBox {
	return &MvhdBox{
		Timescale:   90000,      // Irrelevant since mdhd timescale is used
		NextTrackID: 2,          // There will typically only be one track
		Rate:        0x00010000, // This is 1.0
		Volume:      0x0100,     // Full volume
		Matrix:      UnityMatrix(),
	}
}

// DecodeMvhd - box-specific decode
func DecodeMvhd(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	sr := bits.NewFixedSliceReader(data)
	return DecodeMvhdSR(hdr, startPos, sr)
}

// DecodeMvhdSR - box-specific decode
func DecodeMvhdSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	versionAndFlags := sr.ReadUint32()
	version := byte(versionAndFlags >> 24)

	m := &MvhdBox{
		Version: version,
		Flags:   versionAndFlags & flagsMask,
	}

	if version == 1 {
		m.CreationTime = sr.ReadUint64()
		m.ModificationTime = sr.ReadUint64()
		m.Timescale = sr.ReadUint32()
		m.Duration = sr.ReadUint64()
	} else {
		m.CreationTime = uint64(sr.ReadUint32())
		m.ModificationTime = uint64(sr.ReadUint32())
		m.Timescale = sr.ReadUint32()
		m.Duration = uint64(sr.ReadUint32())
	}
	m.Rate = Fixed32(sr.ReadUint32())
	m.Volume = Fixed16(sr.ReadUint16())
	sr.SkipBytes(10) // Reserved bytes
	for i := range m.Matrix {
		m.Matrix[i] = sr.ReadInt32()
	}
	m.QuickTimePreviewTime = sr.ReadUint32()
	m.QuickTimePreviewDuration = sr.ReadUint32()
	m.QuickTimePosterTime = sr.ReadUint32()
	m.QuickTimeSelectionTime = sr.ReadUint32()
	m.QuickTimeSelectionDuration = sr.ReadUint32()
	m.QuickTimeCurrentTime = sr.ReadUint32()
	m.NextTrackID = sr.ReadUint32()
	return m, sr.AccError()
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
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - box-specific encode to slicewriter
func (b *MvhdBox) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
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
	matrix := b.Matrix
	if matrix == ([9]int32{}) {
		matrix = UnityMatrix() // for backwards compatibility, according to 8.2.2.2
	}
	for _, v := range matrix {
		sw.WriteInt32(v)
	}
	sw.WriteUint32(b.QuickTimePreviewTime)
	sw.WriteUint32(b.QuickTimePreviewDuration)
	sw.WriteUint32(b.QuickTimePosterTime)
	sw.WriteUint32(b.QuickTimeSelectionTime)
	sw.WriteUint32(b.QuickTimeSelectionDuration)
	sw.WriteUint32(b.QuickTimeCurrentTime)
	sw.WriteUint32(b.NextTrackID)

	return sw.AccError()
}

// Info - write box-specific information
func (b *MvhdBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, int(b.Version), b.Flags)
	bd.write(" - timeScale: %d", b.Timescale)
	bd.write(" - duration: %d", b.Duration)
	bd.write(" - creation time: %s", timeStr(b.CreationTime))
	bd.write(" - modification time: %s", timeStr(b.ModificationTime))
	for _, t := range []struct {
		name  string
		value uint32
	}{
		{"quickTimePreviewTime", b.QuickTimePreviewTime},
		{"quickTimePreviewDuration", b.QuickTimePreviewDuration},
		{"quickTimePosterTime", b.QuickTimePosterTime},
		{"quickTimeSelectionTime", b.QuickTimeSelectionTime},
		{"quickTimeSelectionDuration", b.QuickTimeSelectionDuration},
		{"quickTimeCurrentTime", b.QuickTimeCurrentTime},
	} {
		if t.value != 0 {
			bd.write(" - %s: %d", t.name, t.value)
		}
	}
	return bd.err
}

// CreationTimeS returns the creation time in seconds since Jan 1, 1970
func (b *MvhdBox) CreationTimeS() int64 {
	return int64(b.CreationTime) - EpochDiffS
}

// ModificationTimeS returns the modification time in seconds since Jan 1, 1970
func (b *MvhdBox) ModificationTimeS() int64 {
	return int64(b.ModificationTime) - EpochDiffS
}

// SetCreationTimeS sets the creation time from seconds since Jan 1, 1970
func (b *MvhdBox) SetCreationTimeS(unixTimeS int64) {
	b.CreationTime = uint64(unixTimeS + EpochDiffS)
}

// SetModificationTimeS sets the modification time from seconds since Jan 1, 1970
func (b *MvhdBox) SetModificationTimeS(unixTimeS int64) {
	b.ModificationTime = uint64(unixTimeS + EpochDiffS)
}

// Make time string from t which is seconds since Jan. 1 1904
func timeStr(t uint64) string {
	unixSeconds := int64(t) - EpochDiffS
	if unixSeconds < 0 {
		return "0"
	}
	ut := time.Unix(unixSeconds, 0)
	return ut.UTC().Format("2006-01-02T15:04:05Z")
}
