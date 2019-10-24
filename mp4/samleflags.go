package mp4

// SampleFlags according to 14496-12 Sec. 8.8.3.1
type SampleFlags struct {
	IsLeading                 uint32
	SampleDependsOn           uint32
	SampleIsDependedOn        uint32
	SampleHasRedundancy       uint32
	SampleIsNonSync           uint32
	SampleDegradationPriority uint32
}

// DecodeSampleFlags - decode a uint32 flags field
func DecodeSampleFlags(u uint32) *SampleFlags {
	return &SampleFlags{
		IsLeading:                 (u >> 26) & 0x3,
		SampleDependsOn:           (u >> 24) & 0x3,
		SampleIsDependedOn:        (u >> 22) & 0x3,
		SampleHasRedundancy:       (u >> 20) & 0x3,
		SampleIsNonSync:           (u >> 16) & 0x1,
		SampleDegradationPriority: u & 0xffff,
	}
}
