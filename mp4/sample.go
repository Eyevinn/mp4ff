package mp4

import "encoding/binary"

// Sample - sample as used in trun box (mdhd timescale)
type Sample struct {
	Flags                 uint32 // interpreted as SampleFlags
	Dur                   uint32 // Sample duration in mdhd timescale
	Size                  uint32 // Size of sample data
	CompositionTimeOffset int32  // Signed composition time offset
}

// NewSample - create Sample with trun data
func NewSample(flags uint32, dur uint32, size uint32, compositionTimeOffset int32) Sample {
	return Sample{
		Flags:                 flags,
		Dur:                   dur,
		Size:                  size,
		CompositionTimeOffset: compositionTimeOffset,
	}
}

// IsSync - check sync by masking flags including dependsOn
func (s *Sample) IsSync() bool {
	decFlags := DecodeSampleFlags(s.Flags)
	return !decFlags.SampleIsNonSync && (decFlags.SampleDependsOn == 2)
}

// FullSample - include accumulated time and data. Times in mdhd timescale
type FullSample struct {
	Sample
	DecodeTime uint64 // Absolute decode time (offset + accumulated sample Dur)
	Data       []byte // Sample data
}

// PresentationTime - DecodeTime displaced by composition time offset (possibly negative)
func (s *FullSample) PresentationTime() uint64 {
	p := int64(s.DecodeTime) + int64(s.CompositionTimeOffset)
	if p < 0 {
		p = 0 // Extraordinary case. Clip it to 0.
	}
	return uint64(p)
}

func toAnnexB(videoSample []byte) {
	length := uint64(len(videoSample))
	var pos uint64 = 0
	for pos < length-4 {
		lenSlice := videoSample[pos : pos+4]
		nalLen := binary.BigEndian.Uint32(lenSlice)
		videoSample[pos] = 0
		videoSample[pos+1] = 0
		videoSample[pos+2] = 0
		videoSample[pos+3] = 1
		pos += uint64(nalLen + 4)
	}
}
