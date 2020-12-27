package mp4

import "testing"

func TestSdtp(t *testing.T) {
	entries := []*SdtpEntry{
		{IsLeading: 0, SampleDependsOn: 2, SampleIsDependedOn: 0, SampleHasRedundancy: 0},
		{IsLeading: 0, SampleDependsOn: 1, SampleIsDependedOn: 2, SampleHasRedundancy: 0},
		{IsLeading: 1, SampleDependsOn: 2, SampleIsDependedOn: 1, SampleHasRedundancy: 1},
	}

	boxDiffAfterEncodeAndDecode(t, CreateSdtpBox(entries))
}
