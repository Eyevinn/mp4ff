package mp4

import (
	"testing"
)

func TestStss(t *testing.T) {

	// The following stss box has two sync samples
	stss := &StssBox{
		SampleNumber: []uint32{1, 26},
	}

	tests := []struct {
		sample uint32
		isSync bool
	}{
		{
			sample: 1,
			isSync: true,
		},
		{
			sample: 2,
			isSync: false,
		},
		{
			sample: 26,
			isSync: true,
		},
	}

	for _, test := range tests {
		isSync := stss.IsSyncSample(test.sample)
		if isSync != test.isSync {
			t.Errorf("Sample %d has not write sync state", test.sample)
		}
	}
}
