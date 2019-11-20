package mp4

import (
	"io"
	"io/ioutil"
)

// TrunBox - Track Fragment Run Box (trun)
//
// Contained in :  Track Fragmnet Box (traf)
//
type TrunBox struct {
	Version          byte
	flags            uint32
	sampleCount      uint32
	DataOffset       int32
	firstSampleFlags uint32
	samples          []*Sample
}

const dataOffsetPresentFlag = 0x01
const firstSamplePresentFlag = 0x02
const sampleDurationPresentFlag = 0x100
const sampleSizePresentFlag = 0x200
const sampleFlagsPresentFlag = 0x400
const sampleCTOPresentFlag = 0x800

/*
// NewTrunBox - Create a new TrunBox
func NewTrunBox(baseMediaDecodeTime uint64) *TrunBox {
	var version byte = 0
	if baseMediaDecodeTime >= 4294967296 {
		version = 1
	}
	return &TrunBox{
		Version:             version,
		flags:               0,
		BaseMediaDecodeTime: baseMediaDecodeTime,
	}
} */

// DecodeTrun - box-specific decode
func DecodeTrun(size uint64, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	s := NewSliceReader(data)
	versionAndFlags := s.ReadUint32()
	t := &TrunBox{
		Version:     byte(versionAndFlags >> 24),
		flags:       versionAndFlags & 0xffffff,
		sampleCount: s.ReadUint32(),
	}

	if t.HasDataOffset() {
		t.DataOffset = s.ReadInt32()
	}

	if t.HasFirstSampleFlags() {
		t.firstSampleFlags = s.ReadUint32()
	}

	var i uint32
	for i = 0; i < t.sampleCount; i++ {
		var dur, size, flags uint32
		var cto int32
		if t.HasSampleDuration() {
			dur = s.ReadUint32()
		}
		if t.HasSampleSize() {
			size = s.ReadUint32()
		}
		if t.HasSampleFlags() {
			flags = s.ReadUint32()
		} else if t.HasFirstSampleFlags() && i == 0 {
			flags = t.firstSampleFlags
		}
		if t.HasSampleCTO() {
			cto = s.ReadInt32()
		}
		sample := NewSample(flags, dur, size, cto)
		t.samples = append(t.samples, sample)
	}

	return t, nil
}

// AddSampleDefaultValues - add values from tfhd box if needed
func (t *TrunBox) AddSampleDefaultValues(tfhd *TfhdBox) {
	// Here we will decode samles including default values

	var defaultSampleDuration uint32
	var defaultSampleSize uint32
	var defaultSampleFlags uint32

	if tfhd.HasDefaultSampleDuration() {
		defaultSampleDuration = tfhd.DefaultSampleDuration
	}
	if tfhd.HasDefaultSampleSize() {
		defaultSampleSize = tfhd.DefaultSampleSize
	}
	if tfhd.HasDefaultSampleFlags() {
		defaultSampleFlags = tfhd.DefaultSampleFlags
	}
	var i uint32
	for i = 0; i < t.sampleCount; i++ {
		if !t.HasSampleDuration() {
			t.samples[i].Dur = defaultSampleDuration
		}
		if !t.HasSampleSize() {
			t.samples[i].Size = defaultSampleSize
		}
		if !t.HasSampleFlags() {
			t.samples[i].Flags = defaultSampleFlags
		}
	}
}

// SampleCount - return how many samples are defined
func (t *TrunBox) SampleCount() uint32 {
	return t.sampleCount
}

// HasDataOffset - interpted dataOffsetPresent flag
func (t *TrunBox) HasDataOffset() bool {
	return t.flags&0x01 != 0
}

// HasFirstSampleFlags - interpreted firstSampleFlagsPresent flag
func (t *TrunBox) HasFirstSampleFlags() bool {
	return t.flags&firstSamplePresentFlag != 0
}

// HasSampleDuration - interpreted sampleDurationPresent flag
func (t *TrunBox) HasSampleDuration() bool {
	return t.flags&sampleDurationPresentFlag != 0
}

// HasSampleFlags - interpreted sampleFlagsPresent flag
func (t *TrunBox) HasSampleFlags() bool {
	return t.flags&sampleFlagsPresentFlag != 0
}

// HasSampleSize - interpreted sampleSizePresent flag
func (t *TrunBox) HasSampleSize() bool {
	return t.flags&sampleSizePresentFlag != 0
}

// HasSampleCTO - interpreted sampleCompositionTimeOffset flag
func (t *TrunBox) HasSampleCTO() bool {
	return t.flags&sampleCTOPresentFlag != 0
}

// Type - return box type
func (t *TrunBox) Type() string {
	return "trun"
}

// Size - return calculated size
func (t *TrunBox) Size() uint64 {
	sz := boxHeaderSize + 8
	if t.HasDataOffset() {
		sz += 4
	}
	if t.HasFirstSampleFlags() {
		sz += 4
	}
	bytesPerSample := 0
	if t.HasSampleDuration() {
		bytesPerSample += 4
	}
	if t.HasSampleSize() {
		bytesPerSample += 4
	}
	if t.HasSampleFlags() {
		bytesPerSample += 4
	}
	if t.HasSampleCTO() {
		bytesPerSample += 4
	}
	sz += int(t.sampleCount) * bytesPerSample
	return uint64(sz)
}

// Encode - write box to w
func (t *TrunBox) Encode(w io.Writer) error {
	err := EncodeHeader(t, w)
	if err != nil {
		return err
	}
	buf := makebuf(t)
	sw := NewSliceWriter(buf)
	versionAndFlags := (uint32(t.Version) << 24) + t.flags
	sw.WriteUint32(versionAndFlags)
	sw.WriteUint32(t.sampleCount)
	if t.HasDataOffset() {
		sw.WriteInt32(t.DataOffset)
	}
	if t.HasFirstSampleFlags() {
		sw.WriteUint32(t.firstSampleFlags)
	}
	var i uint32
	for i = 0; i < t.sampleCount; i++ {
		if t.HasSampleDuration() {
			sw.WriteUint32(t.samples[i].Dur)
		}
		if t.HasSampleSize() {
			sw.WriteUint32(t.samples[i].Size)
		}
		if t.HasSampleFlags() {
			sw.WriteUint32(t.samples[i].Flags)
		}
		if t.HasSampleCTO() {
			sw.WriteInt32(t.samples[i].Cto)
		}

	}
	_, err = w.Write(buf)
	return err
}

// GetSampleData - return list of Samples. baseOffset is offset in mdat
func (t *TrunBox) GetSampleData(baseOffset uint32, baseTime uint64, mdat *MdatBox) []*SampleComplete {
	samples := make([]*SampleComplete, 0, t.SampleCount())
	var accDur uint64 = 0
	offset := baseOffset
	for _, s := range t.samples {
		dTime := baseTime + accDur
		pTime := uint64(int64(dTime) + int64(s.Cto))

		newSample := &SampleComplete{
			Sample:           *s,
			DecodeTime:       dTime,
			PresentationTime: pTime,
			Data:             mdat.Data[offset : offset+s.Size],
		}
		samples = append(samples, newSample)
		accDur += uint64(s.Dur)
		offset += s.Size
	}
	return samples
}
