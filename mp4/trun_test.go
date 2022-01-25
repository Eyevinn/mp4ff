package mp4

import (
	"testing"

	"github.com/go-test/deep"
)

// TestTrunDump versus golden file. Can be regenerated with -update
func TestTrunInfo(t *testing.T) {
	goldenDumpPath := "testdata/golden_trun_dump.txt"
	trun := CreateTrun(0)
	trun.DataOffset = 314159
	fs := FullSample{
		Sample: Sample{
			Flags:                 SyncSampleFlags,
			Dur:                   1024,
			Size:                  4,
			CompositionTimeOffset: -512,
		},
		DecodeTime: 1024,
		Data:       []byte{0, 1, 2, 3},
	}
	trun.AddFullSample(&fs)

	err := compareOrUpdateInfo(t, trun, goldenDumpPath)
	if err != nil {
		t.Error(err)
	}
}

func TestGetSampleNrForRelativeTime(t *testing.T) {
	trun := CreateTrun(0)
	trun.AddSamples([]Sample{
		{0, 1024, 100, 0},
		{0, 1024, 100, 0},
		{0, 1024, 100, 0},
		{0, 1024, 100, 0},
		{0, 1024, 100, 0},
		{0, 1024, 100, 0},
		{0, 1024, 100, 0},
	})
	trun.Flags |= TrunSampleSizePresentFlag | TrunSampleDurationPresentFlag

	testCases := []struct {
		sampleTime     uint64
		wantedSampleNr uint32
		trunFlags      uint32
		wantedError    bool
	}{
		{0, 1, 0, false},
		{5 * 1024, 6, 0, false},
		{1023, 0, 0, true},
		{7 * 1024, 0, 0, true},
		{0, 1, TrunSampleDurationPresentFlag, false},
		{5 * 1024, 6, TrunSampleDurationPresentFlag, false},
		{1023, 0, TrunSampleDurationPresentFlag, true},
		{7 * 1024, 0, TrunSampleDurationPresentFlag, true},
	}

	const defaultSampleDuration = 1024

	for i, tc := range testCases {
		trun.Flags = tc.trunFlags
		gotSampleNr, err := trun.GetSampleNrForRelativeTime(tc.sampleTime, defaultSampleDuration)
		if tc.wantedError {
			if err == nil {
				t.Errorf("case %d: did not get an error", i)
			}
			continue
		}
		if err != nil {
			t.Error(err)
			continue
		}
		if gotSampleNr != tc.wantedSampleNr {
			t.Errorf("case %d: got sample nr %d instead of %d", i, gotSampleNr, tc.wantedSampleNr)
		}
	}
}

func TestGetSampleInterval(t *testing.T) {
	trun := CreateTrun(0)
	trun.AddSamples([]Sample{
		{0, 100, 1000, 0},
		{0, 200, 2000, 0},
		{0, 300, 3000, 0},
		{0, 400, 4000, 0},
		{0, 500, 5000, 0},
		{0, 600, 6000, 0},
		{0, 700, 7000, 0},
	})

	mdat := MdatBox{lazyDataSize: 28000}

	testCases := []struct {
		startSampleNr  uint32
		endSampleNr    uint32
		baseDecodeTime uint64
		mdat           *MdatBox
		offsetInMdat   uint32
		wantedSItvl    SampleInterval
	}{
		{
			1, 2, 10000, &mdat, 0, SampleInterval{10000, []Sample{{0, 100, 1000, 0}, {0, 200, 2000, 0}}, 0, 3000, nil},
		},
		{
			3, 4, 10000, &mdat, 0, SampleInterval{10300, []Sample{{0, 300, 3000, 0}, {0, 400, 4000, 0}}, 3000, 7000, nil},
		},
	}

	for i, tc := range testCases {
		gotSItvl, err := trun.GetSampleInterval(tc.startSampleNr, tc.endSampleNr, tc.baseDecodeTime, tc.mdat, tc.offsetInMdat)
		if err != nil {
			t.Error(err)
		}
		if diff := deep.Equal(gotSItvl, tc.wantedSItvl); diff != nil {
			t.Errorf("case %d: %s", i, diff)
		}
	}
}

func TestFirstSampleFlags(t *testing.T) {
	trun := CreateTrun(0)
	trun.DataOffset = 314159
	trun.AddSample(Sample{
		Flags:                 NonSyncSampleFlags,
		Dur:                   1000,
		Size:                  1000,
		CompositionTimeOffset: 0,
	})
	trun.AddSample(Sample{
		Flags:                 NonSyncSampleFlags,
		Dur:                   1000,
		Size:                  1000,
		CompositionTimeOffset: 0,
	})
	_, present := trun.FirstSampleFlags()
	if present {
		t.Error("firstSampleFlags present")
	}
	trun.SetFirstSampleFlags(SyncSampleFlags)
	gotFirstFlags, present := trun.FirstSampleFlags()
	if !present {
		t.Error("firstSampleFlags absent")
	}
	if gotFirstFlags != SyncSampleFlags {
		t.Errorf("got firstSampleFlags %02x instead of %02x", gotFirstFlags, SyncSampleFlags)
	}
	trun.RemoveFirstSampleFlags()
	_, present = trun.FirstSampleFlags()
	if present {
		t.Error("firstSampleFlags present after removal")
	}
}
