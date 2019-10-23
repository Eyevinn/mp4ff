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
	Flags            uint32
	sampleCount      uint32
	dataOffset       int32
	firstSampleFlags uint32
	samples          []*Sample
}

const dataOffsetPresentFlag = 0x01
const firstSamplePresentFlag = 0x02
const sampleDurationPresentFlag = 0x100
const sampleSizePresentFlag = 0x200
const sampleFlagsPresentFlag = 0x400
const sampleCTOPresentFlag = 0x800

// DecodeTrun - box-specific decode
func DecodeTrun(r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	s := NewSliceReader(data)
	versionAndFlags := s.ReadUint32()
	t := &TrunBox{
		Version:     byte(versionAndFlags >> 24),
		Flags:       versionAndFlags & 0xffffff,
		sampleCount: s.ReadUint32(),
	}

	if t.HasDataOffset() {
		t.dataOffset = s.ReadInt32()
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
			t.samples[i].dur = defaultSampleDuration
		}
		if !t.HasSampleSize() {
			t.samples[i].size = defaultSampleSize
		}
		if !t.HasSampleFlags() {
			t.samples[i].flags = defaultSampleFlags
		}
	}
}

// SampleCount - return how many samples are defined
func (t *TrunBox) SampleCount() uint32 {
	return t.sampleCount
}

// HasDataOffset - interpted dataOffsetPresent flag
func (t *TrunBox) HasDataOffset() bool {
	return t.Flags&0x01 != 0
}

// HasFirstSampleFlags - interpreted firstSampleFlagsPresent flag
func (t *TrunBox) HasFirstSampleFlags() bool {
	return t.Flags&firstSamplePresentFlag != 0
}

// HasSampleDuration - interpreted sampleDurationPresent flag
func (t *TrunBox) HasSampleDuration() bool {
	return t.Flags&sampleDurationPresentFlag != 0
}

// HasSampleFlags - interpreted sampleFlagsPresent flag
func (t *TrunBox) HasSampleFlags() bool {
	return t.Flags&sampleFlagsPresentFlag != 0
}

// HasSampleSize - interpreted sampleSizePresent flag
func (t *TrunBox) HasSampleSize() bool {
	return t.Flags&sampleSizePresentFlag != 0
}

// HasSampleCTO - interpreted sampleCompositionTimeOffset flag
func (t *TrunBox) HasSampleCTO() bool {
	return t.Flags&sampleCTOPresentFlag != 0
}

// Type - return box type
func (t *TrunBox) Type() string {
	return "trun"
}

// Size - return calculated size
func (t *TrunBox) Size() int {
	sz := BoxHeaderSize + 8
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
	return sz
}

// Encode - write box to w
func (t *TrunBox) Encode(w io.Writer) error {
	err := EncodeHeader(t, w)
	if err != nil {
		return err
	}
	buf := makebuf(t)
	bb := NewBufferWrapper(buf)
	versionAndFlags := (uint32(t.Version) << 24) + t.Flags
	bb.WriteUint32(versionAndFlags)
	bb.WriteUint32(t.sampleCount)
	if t.HasDataOffset() {
		bb.WriteInt32(t.dataOffset)
	}
	if t.HasFirstSampleFlags() {
		bb.WriteUint32(t.firstSampleFlags)
	}
	var i uint32
	for i = 0; i < t.sampleCount; i++ {
		if t.HasSampleDuration() {
			bb.WriteUint32(t.samples[i].dur)
		}
		if t.HasSampleSize() {
			bb.WriteUint32(t.samples[i].size)
		}
		if t.HasSampleFlags() {
			bb.WriteUint32(t.samples[i].flags)
		}
		if t.HasSampleCTO() {
			bb.WriteInt32(t.samples[i].cto)
		}

	}
	_, err = w.Write(buf)
	return err
}
