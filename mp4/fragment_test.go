package mp4

import (
	"testing"
)

func TestCreateMultiTrackFragment(t *testing.T) {

	trackIDs := []uint32{1, 2, 3}
	mFrag, err := CreateMultiTrackFragment(1, trackIDs)
	if err != nil {
		t.Error("Error creating MultiTrackFragment")
	}
	if len(mFrag.Moof.Trafs) != 3 {
		t.Error("Not 3 tracks in MultiTrackFragment")
	}
}

func TestFragmentSampleIntervals(t *testing.T) {
	frag, err := CreateFragment(12, 1)
	if err != nil {
		t.Error("Error creating Fragment")
	}
	s := NewSample(0, 100, 1, 0)
	frag.AddSample(s, 1230)
	samples := []Sample{NewSample(0, 100, 2, 0), NewSample(0, 100, 3, 0), NewSample(0, 100, 4, 0)}
	frag.AddSamples(samples, 1330)

	sampleNr, err := frag.GetSampleNrFromTime(nil, 1430)
	if err != nil {
		t.Error("Error getting sample number from time")
	}
	if sampleNr != 3 {
		t.Error("Wrong sample number from time")
	}

	sIntv, err := frag.GetSampleInterval(nil, 2, 3)
	if err != nil {
		t.Error("Error getting sample interval")
	}
	if sIntv.FirstDecodeTime != 1330 {
		t.Error("Wrong first decode time")
	}

	// Check common sample duration from trex
	_, err = frag.CommonSampleDuration(nil)
	if err == nil {
		t.Error("Should have gotten error from CommonSampleDuration")
	}

	sampleItvl := SampleInterval{
		FirstDecodeTime: 1630,
		Samples:         []Sample{{0, 100, 2, 0}},
		OffsetInMdat:    0,
		Data:            []byte{},
	}
	err = frag.AddSampleInterval(sampleItvl)
	if err != nil {
		t.Error("Error adding sample interval")
	}
	sampleItvl.Reset()
}
