package mp4

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

//SampleComplete - include times and data
type SampleComplete struct {
	Sample
	DecodeTime       uint64
	PresentationTime uint64
	Data             []byte
}
