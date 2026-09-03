package mp4_test

import (
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
	"github.com/go-test/deep"
)

func TestSampleFlags(t *testing.T) {
	sf := mp4.SampleFlags{
		IsLeading:                 1,
		SampleDependsOn:           2,
		SampleIsDependedOn:        1,
		SampleHasRedundancy:       3,
		SamplePaddingValue:        5,
		SampleIsNonSync:           true,
		SampleDegradationPriority: 42,
	}

	sfBin := sf.Encode()
	sfDec := mp4.DecodeSampleFlags(sfBin)
	diff := deep.Equal(sfDec, sf)
	if diff != nil {
		t.Error(diff)
	}
}

func TestSetSyncAndNonSyncFlags(t *testing.T) {
	zeroFlags := uint32(0)

	// Test SetSyncSampleFlags on zero flags
	syncResult := mp4.SetSyncSampleFlags(zeroFlags)
	expectedSync := uint32(0x02000000) // Only the sync sample bit should be set
	if syncResult != expectedSync {
		t.Errorf("SetSyncSampleFlags(0) = 0x%08x, expected 0x%08x", syncResult, expectedSync)
	}

	// Test SetNonSyncSampleFlags on zero flags
	nonSyncResult := mp4.SetNonSyncSampleFlags(zeroFlags)
	expectedNonSync := uint32(0x01010000) // NonSyncSampleFlags (0x00010000) | SampleDependsOn1 (0x01000000)
	if nonSyncResult != expectedNonSync {
		t.Errorf("SetNonSyncSampleFlags(0) = 0x%08x, expected 0x%08x", nonSyncResult, expectedNonSync)
	}

	// Test with SampleHasRedundancy = 2 (bits 21-20 = 10 binary = 0x00200000)
	redundancyFlags := uint32(2 << 20) // 0x00200000

	// Test SetSyncSampleFlags preserves redundancy bits
	syncWithRedundancy := mp4.SetSyncSampleFlags(redundancyFlags)
	expectedSyncWithRedundancy := uint32(0x02200000) // Sync bit + redundancy bits
	if syncWithRedundancy != expectedSyncWithRedundancy {
		t.Errorf("SetSyncSampleFlags(0x%08x) = 0x%08x, expected 0x%08x", redundancyFlags, syncWithRedundancy, expectedSyncWithRedundancy)
	}

	// Test SetNonSyncSampleFlags preserves redundancy bits
	nonSyncWithRedundancy := mp4.SetNonSyncSampleFlags(redundancyFlags)
	expectedNonSyncWithRedundancy := uint32(0x01210000) // NonSync + SampleDependsOn1 + redundancy bits
	if nonSyncWithRedundancy != expectedNonSyncWithRedundancy {
		t.Errorf("SetNonSyncSampleFlags(0x%08x) = 0x%08x, expected 0x%08x",
			redundancyFlags, nonSyncWithRedundancy, expectedNonSyncWithRedundancy)
	}
}

// TestIsSyncSampleFlags checks that the sync predicate requires both
// indications to agree: sample_depends_on == 2 and sample_is_non_sync_sample
// == 0. Every other bit of sample_flags must be ignored, which is asserted by
// repeating each case with all of them set.
func TestIsSyncSampleFlags(t *testing.T) {
	// Every bit outside the sample_depends_on field and the non-sync bit.
	const otherBits = ^uint32(0x03010000)

	for dependsOn := uint32(0); dependsOn < 4; dependsOn++ {
		for nonSync := uint32(0); nonSync < 2; nonSync++ {
			flags := dependsOn<<24 | nonSync<<16
			want := dependsOn == 2 && nonSync == 0
			for _, f := range []uint32{flags, flags | otherBits} {
				if got := mp4.IsSyncSampleFlags(f); got != want {
					t.Errorf("IsSyncSampleFlags(0x%08x) = %v, want %v (dependsOn=%d nonSync=%d)",
						f, got, want, dependsOn, nonSync)
				}
				s := mp4.Sample{Flags: f}
				if got := s.IsSync(); got != want {
					t.Errorf("Sample{0x%08x}.IsSync() = %v, want %v", f, got, want)
				}
			}
		}
	}

	// The exported patterns must agree with the predicate.
	if !mp4.IsSyncSampleFlags(mp4.SyncSampleFlags) {
		t.Error("SyncSampleFlags is not recognized as a sync sample")
	}
	if mp4.IsSyncSampleFlags(mp4.SetNonSyncSampleFlags(0)) {
		t.Error("SetNonSyncSampleFlags(0) is recognized as a sync sample")
	}
}
