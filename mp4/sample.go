package mp4

import "encoding/binary"

// Sample - sample as used in trun box
type Sample struct {
	Flags uint32
	Dur   uint32
	Size  uint32
	Cto   int32
}

// NewSample - create Sample
func NewSample(flags uint32, dur uint32, size uint32, cto int32) *Sample {
	return &Sample{
		Flags: flags,
		Dur:   dur,
		Size:  size,
		Cto:   cto,
	}
}

// IsSync - check sync by masking flags including dependsOn
func (s *Sample) IsSync() bool {
	decFlags := DecodeSampleFlags(s.Flags)
	return !decFlags.SampleIsNonSync && (decFlags.SampleDependsOn == 2)
}

//SampleComplete - include times and data
type SampleComplete struct {
	Sample
	DecodeTime       uint64
	PresentationTime uint64
	Data             []byte
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
