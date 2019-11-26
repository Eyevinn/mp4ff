package mp4

import (
	"fmt"
	"io"
	"io/ioutil"
)

const baseDataOffsetPresent = 0x000001
const sampleDescriptionIndexPresent = 0x000002
const defaultSampleDurationPresent = 0x000008
const defaultSampleSizePresent = 0x000010
const defaultSampleFlagsPresent = 0x000020
const durationIsEmpty = 0x010000
const defaultBaseIsMoof = 0x020000

// TfhdBox - Track Fragment Header Box (tfhd)
//
// Contained in : Track Fragment box (traf))
type TfhdBox struct {
	Version                byte
	Flags                  uint32
	TrackID                uint32
	BaseDataOffset         uint64
	SampleDescriptionIndex uint32
	DefaultSampleDuration  uint32
	DefaultSampleSize      uint32
	DefaultSampleFlags     uint32
}

// DecodeTfhd - box-specific decode
func DecodeTfhd(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	s := NewSliceReader(data)
	versionAndFlags := s.ReadUint32()
	version := byte(versionAndFlags >> 24)
	flags := versionAndFlags & flagsMask

	t := &TfhdBox{
		Version: version,
		Flags:   flags,
		TrackID: s.ReadUint32(),
	}

	if t.HasBaseDataOffset() {
		t.BaseDataOffset = s.ReadUint64()
	}
	if t.HasSampleDescriptionIndex() {
		t.SampleDescriptionIndex = s.ReadUint32()
	}
	if t.HasDefaultSampleDuration() {
		t.DefaultSampleDuration = s.ReadUint32()
	}
	if t.HasDefaultSampleSize() {
		t.DefaultSampleFlags = s.ReadUint32()
	}
	if t.HasDefaultSampleFlags() {
		t.DefaultSampleDuration = s.ReadUint32()
	}

	return t, nil
}

// CreateTfhd - Create a new TfdtBox with baseMediaDecodeTime
func CreateTfhd(trackID uint32) *TfhdBox {
	// We only set defaultBaseIsMoof flag
	tfhd := &TfhdBox{
		Version:                0,
		Flags:                  0x020000,
		TrackID:                trackID,
		BaseDataOffset:         0,
		SampleDescriptionIndex: 0,
		DefaultSampleDuration:  0,
		DefaultSampleSize:      0,
		DefaultSampleFlags:     0,
	}
	return tfhd
}

// HasBaseDataOffset - interpreted flags value
func (t *TfhdBox) HasBaseDataOffset() bool {
	return t.Flags&baseDataOffsetPresent != 0
}

// HasSampleDescriptionIndex - interpreted flags value
func (t *TfhdBox) HasSampleDescriptionIndex() bool {
	return t.Flags&sampleDescriptionIndexPresent != 0
}

// HasDefaultSampleDuration - interpreted flags value
func (t *TfhdBox) HasDefaultSampleDuration() bool {
	return t.Flags&defaultSampleDurationPresent != 0
}

// HasDefaultSampleSize - interpreted flags value
func (t *TfhdBox) HasDefaultSampleSize() bool {
	return t.Flags&defaultSampleSizePresent != 0
}

// HasDefaultSampleFlags - interpreted flags value
func (t *TfhdBox) HasDefaultSampleFlags() bool {
	return t.Flags&defaultSampleFlagsPresent != 0
}

// DurationIsEmpty - interpreted flags value
func (t *TfhdBox) DurationIsEmpty() bool {
	return t.Flags&durationIsEmpty != 0
}

// DefaultBaseIfMoof - interpreted flags value
func (t *TfhdBox) DefaultBaseIfMoof() bool {
	return t.Flags&defaultBaseIsMoof != 0
}

// Type - returns box type
func (t *TfhdBox) Type() string {
	return "tfhd"
}

// Size - returns calculated size
func (t *TfhdBox) Size() uint64 {
	sz := boxHeaderSize + 8
	if t.HasBaseDataOffset() {
		sz += 8
	}
	if t.HasSampleDescriptionIndex() {
		sz += 4
	}
	if t.HasDefaultSampleDuration() {
		sz += 4
	}
	if t.HasDefaultSampleSize() {
		sz += 4
	}
	if t.HasDefaultSampleFlags() {
		sz += 4
	}
	return uint64(sz)
}

// Dump - print box specific data
func (t *TfhdBox) Dump() {
	fmt.Printf("Track Fragment Header:\n Track ID: %d\n", t.TrackID)
}

// Encode - write box to w
func (t *TfhdBox) Encode(w io.Writer) error {
	err := EncodeHeader(t, w)
	if err != nil {
		return err
	}
	buf := makebuf(t)
	sw := NewSliceWriter(buf)
	versionAndFlags := (uint32(t.Version) << 24) + t.Flags
	sw.WriteUint32(versionAndFlags)
	sw.WriteUint32(t.TrackID)
	if t.HasBaseDataOffset() {
		sw.WriteUint64(t.BaseDataOffset)
	}
	if t.HasSampleDescriptionIndex() {
		sw.WriteUint32(t.SampleDescriptionIndex)
	}
	if t.HasDefaultSampleDuration() {
		sw.WriteUint32(t.DefaultSampleDuration)
	}
	if t.HasDefaultSampleSize() {
		sw.WriteUint32(t.DefaultSampleSize)
	}
	if t.HasDefaultSampleFlags() {
		sw.WriteUint32(t.DefaultSampleFlags)
	}

	_, err = w.Write(buf)
	return err
}
