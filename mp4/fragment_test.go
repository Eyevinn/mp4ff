package mp4_test

import (
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

func TestCreateMultiTrackFragment(t *testing.T) {

	trackIDs := []uint32{1, 2, 3}
	mFrag, err := mp4.CreateMultiTrackFragment(1, trackIDs)
	if err != nil {
		t.Error("Error creating MultiTrackFragment")
	}
	if len(mFrag.Moof.Trafs) != 3 {
		t.Error("Not 3 tracks in MultiTrackFragment")
	}
}

func TestFragmentSampleIntervals(t *testing.T) {
	frag, err := mp4.CreateFragment(12, 1)
	if err != nil {
		t.Error("Error creating Fragment")
	}
	s := mp4.NewSample(0, 100, 1, 0)
	frag.AddSample(s, 1230)
	samples := []mp4.Sample{mp4.NewSample(0, 100, 2, 0), mp4.NewSample(0, 100, 3, 0), mp4.NewSample(0, 100, 4, 0)}
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

	sampleItvl := mp4.SampleInterval{
		FirstDecodeTime: 1630,
		Samples:         []mp4.Sample{{0, 100, 2, 0}},
		OffsetInMdat:    0,
		Data:            []byte{},
	}
	err = frag.AddSampleInterval(sampleItvl)
	if err != nil {
		t.Error("Error adding sample interval")
	}
	sampleItvl.Reset()
}

func TestFragmentGetFullSamplesTruncatedMdat(t *testing.T) {
	frag, err := mp4.CreateFragment(1, 1)
	if err != nil {
		t.Fatal(err)
	}
	fs := mp4.FullSample{
		Sample: mp4.Sample{
			Flags: mp4.SyncSampleFlags,
			Dur:   1024,
			Size:  4,
		},
		DecodeTime: 0,
		Data:       []byte{0, 1, 2, 3},
	}
	frag.AddFullSample(fs)
	// Simulate a truncated file: the trun declares more sample bytes
	// than the mdat carries.
	frag.Mdat.Data = frag.Mdat.Data[:2]

	if _, err := frag.GetFullSamples(nil); err == nil {
		t.Error("expected error when mdat is truncated, got nil")
	}
}
