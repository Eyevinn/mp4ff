package mp4

import (
	"fmt"
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
	firstSampleFlags uint32 // interpreted as SampleFlags
	Samples          []*Sample
	writeOrderNr     uint32 // Used for multi trun offsets
}

const dataOffsetPresentFlag uint32 = 0x01
const firstSampleFlagsPresentFlag uint32 = 0x04
const sampleDurationPresentFlag uint32 = 0x100
const sampleSizePresentFlag uint32 = 0x200
const sampleFlagsPresentFlag uint32 = 0x400
const sampleCTOPresentFlag uint32 = 0x800

// DecodeTrun - box-specific decode
func DecodeTrun(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	s := NewSliceReader(data)
	versionAndFlags := s.ReadUint32()
	t := &TrunBox{
		Version:     byte(versionAndFlags >> 24),
		flags:       versionAndFlags & flagsMask,
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
		t.Samples = append(t.Samples, sample)
	}

	return t, nil
}

// CreateTrun - create a TrunBox for filling up with samples
func CreateTrun(writeOrderNr uint32) *TrunBox {
	trun := &TrunBox{
		Version:          1,     // Signed composition_time_offset
		flags:            0xf01, // Data offset and all sample data present
		sampleCount:      0,
		DataOffset:       0,
		firstSampleFlags: 0,
		Samples:          nil,
		writeOrderNr:     writeOrderNr,
	}
	return trun
}

// AddSampleDefaultValues - add values from tfhd and trex boxes if needed
// Return total duration
func (t *TrunBox) AddSampleDefaultValues(tfhd *TfhdBox, trex *TrexBox) (totalDur uint64) {

	var defaultSampleDuration uint32
	var defaultSampleSize uint32
	var defaultSampleFlags uint32

	if tfhd.HasDefaultSampleDuration() {
		defaultSampleDuration = tfhd.DefaultSampleDuration
	} else if trex != nil {
		defaultSampleDuration = trex.DefaultSampleDuration
	}
	if tfhd.HasDefaultSampleSize() {
		defaultSampleSize = tfhd.DefaultSampleSize
	} else if trex != nil {
		defaultSampleSize = trex.DefaultSampleSize
	}
	if tfhd.HasDefaultSampleFlags() {
		defaultSampleFlags = tfhd.DefaultSampleFlags
	} else if trex != nil {
		defaultSampleFlags = trex.DefaultSampleFlags
	}
	var i uint32
	totalDur = 0
	for i = 0; i < t.sampleCount; i++ {
		if !t.HasSampleDuration() {
			t.Samples[i].Dur = defaultSampleDuration
		}
		totalDur += uint64(t.Samples[i].Dur)
		if !t.HasSampleSize() {
			t.Samples[i].Size = defaultSampleSize
		}
		if !t.HasSampleFlags() {
			if i > 0 || !t.HasFirstSampleFlags() {
				t.Samples[i].Flags = defaultSampleFlags
			}
		}
	}
	return totalDur
}

// SampleCount - return how many samples are defined
func (t *TrunBox) SampleCount() uint32 {
	return t.sampleCount
}

// HasDataOffset - interpreted dataOffsetPresent flag
func (t *TrunBox) HasDataOffset() bool {
	return t.flags&dataOffsetPresentFlag != 0
}

// HasFirstSampleFlags - interpreted firstSampleFlagsPresent flag
func (t *TrunBox) HasFirstSampleFlags() bool {
	return t.flags&firstSampleFlagsPresentFlag != 0
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
	sz := boxHeaderSize + 8 // flags + entrycCount
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
		if t.DataOffset == 0 {
			panic("trun data offset not set")
		}
		sw.WriteInt32(t.DataOffset)
	}
	if t.HasFirstSampleFlags() {
		sw.WriteUint32(t.firstSampleFlags)
	}
	var i uint32
	for i = 0; i < t.sampleCount; i++ {
		if t.HasSampleDuration() {
			sw.WriteUint32(t.Samples[i].Dur)
		}
		if t.HasSampleSize() {
			sw.WriteUint32(t.Samples[i].Size)
		}
		if t.HasSampleFlags() {
			sw.WriteUint32(t.Samples[i].Flags)
		}
		if t.HasSampleCTO() {
			sw.WriteInt32(t.Samples[i].Cto)
		}

	}
	_, err = w.Write(buf)
	return err
}

// Info - specificBoxLevels trun:1 gives details
func (t *TrunBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, t, int(t.Version), t.flags)
	bd.write(" - sampleCount: %d", t.sampleCount)
	level := getInfoLevel(t, specificBoxLevels)
	if level > 0 {
		if t.HasDataOffset() {
			bd.write(" - DataOffset: %d", t.DataOffset)
		}
		if t.HasFirstSampleFlags() {
			bd.write(" - firstSampleFlags: %08x (%s)", t.firstSampleFlags, DecodeSampleFlags(t.firstSampleFlags))
		}
		for i := 0; i < int(t.sampleCount); i++ {
			msg := fmt.Sprintf(" - sample[%d]:", i+1)
			if t.HasSampleDuration() {
				msg += fmt.Sprintf(" dur=%d", t.Samples[i].Dur)
			}
			if t.HasSampleSize() {
				msg += fmt.Sprintf(" size=%d", t.Samples[i].Size)
			}
			if t.HasSampleFlags() {
				sampleFlags := t.Samples[i].Flags
				msg += fmt.Sprintf(" flags=%08x (%s)", sampleFlags, DecodeSampleFlags(sampleFlags))
			}
			if t.HasSampleCTO() {
				msg += fmt.Sprintf(" cto=%d", t.Samples[i].Cto)
			}
			bd.write(msg)
		}
	}
	return bd.err
}

// GetFullSamples - get all sample data including accumulated time and binary media
// baseOffset is offset in mdat (normally 8)
// baseTime is offset in track timescale (from mfhd)
// To fill missing individual values from thd and trex defaults, call AddSampleDefaultValues() before this call
func (t *TrunBox) GetFullSamples(baseOffset uint32, baseTime uint64, mdat *MdatBox) []*FullSample {
	samples := make([]*FullSample, 0, t.SampleCount())
	var accDur uint64 = 0
	offset := baseOffset
	for _, s := range t.Samples {
		dTime := baseTime + accDur

		newSample := &FullSample{
			Sample:     *s,
			DecodeTime: dTime,
			Data:       mdat.Data[offset : offset+s.Size],
		}
		samples = append(samples, newSample)
		accDur += uint64(s.Dur)
		offset += s.Size
	}
	return samples
}

// GetSamples - get all trun sample data
// To fill missing individual values from thd and trex defaults, call AddSampleDefaultValues() before this call
func (t *TrunBox) GetSamples() []*Sample {
	return t.Samples
}

// AddFullSample - add Sample part of FullSample
func (t *TrunBox) AddFullSample(s *FullSample) {
	t.Samples = append(t.Samples, &s.Sample)
	t.sampleCount++
}

// AddSample - add a Sample
func (t *TrunBox) AddSample(s *Sample) {
	t.Samples = append(t.Samples, s)
	t.sampleCount++
}

// Duration - calculated duration given defaultSampleDuration
func (t *TrunBox) Duration(defaultSampleDuration uint32) uint64 {
	if !t.HasSampleDuration() {
		return uint64(defaultSampleDuration) * uint64(t.SampleCount())
	}
	var total uint64 = 0
	for _, s := range t.Samples {
		total += uint64(s.Dur)
	}
	return total
}

// SizeOfData - size of mediasamples in bytes
func (t *TrunBox) SizeOfData() (totalSize uint64) {
	for _, sample := range t.Samples {
		totalSize += uint64(sample.Size)
	}
	return totalSize
}
