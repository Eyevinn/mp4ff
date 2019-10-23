package mp4

type Sample struct {
	flags uint32
	dur   uint32
	size  uint32
	cto   int32
}

func NewSample(flags uint32, dur uint32, size uint32, cto int32) *Sample {
	return &Sample{
		flags: flags,
		dur:   dur,
		size:  size,
		cto:   cto,
	}
}
