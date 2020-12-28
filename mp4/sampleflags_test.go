package mp4

import (
	"testing"

	"github.com/go-test/deep"
)

func TestSampleFlags(t *testing.T) {
	sf := SampleFlags{
		IsLeading:                 1,
		SampleDependsOn:           2,
		SampleIsDependedOn:        1,
		SampleHasRedundancy:       3,
		SamplePaddingValue:        5,
		SampleIsNonSync:           true,
		SampleDegradationPriority: 42,
	}

	sfBin := sf.Encode()
	sfDec := DecodeSampleFlags(sfBin)
	diff := deep.Equal(sfDec, sf)
	if diff != nil {
		t.Error(diff)
	}
}
