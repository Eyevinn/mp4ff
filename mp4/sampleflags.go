package mp4

// SampleFlags according to 14496-12 Sec. 8.8.3.1
type SampleFlags struct {
	IsLeading                 uint32
	SampleDependsOn           uint32
	SampleIsDependedOn        uint32
	SampleHasRedundancy       uint32
	SampleDegradationPriority uint16
	SampleIsNonSync           bool
}

// SyncSampleFlags - flags for I-frame or other sync sample
const SyncSampleFlags uint32 = 0x02010000

// NonSyncSampleFlags - flags for non-sync sample
const NonSyncSampleFlags uint32 = 0x01000000

// IsSyncSampleFlags - flags is set correctly for sync sample
func IsSyncSampleFlags(flags uint32) bool {
	return flags&SyncSampleFlags == SyncSampleFlags
}

// SetSyncSampleFlags - return flags with syncsample pattern
func SetSyncSampleFlags(flags uint32) uint32 {
	return flags & SyncSampleFlags & ^NonSyncSampleFlags
}

// SetNonSyncSampleFlags - return flags with nonsyncsample pattern
func SetNonSyncSampleFlags(flags uint32) uint32 {
	return flags & ^SyncSampleFlags & NonSyncSampleFlags
}

// DecodeSampleFlags - decode a uint32 flags field
func DecodeSampleFlags(u uint32) *SampleFlags {
	sf := &SampleFlags{
		IsLeading:                 (u >> 26) & 0x3,
		SampleDependsOn:           (u >> 24) & 0x3,
		SampleIsDependedOn:        (u >> 22) & 0x3,
		SampleHasRedundancy:       (u >> 20) & 0x3,
		SampleIsNonSync:           (u>>16)&0x1 == 1,
		SampleDegradationPriority: uint16(u & 0xffff),
	}
	return sf
}
