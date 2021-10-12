package mp4

import (
	"bytes"
	"reflect"
	"testing"
)

func createTestTrafBox() *TrafBox {
	traf := &TrafBox{}
	tfhd := &TfhdBox{}
	_ = traf.AddChild(tfhd)
	trun := CreateTrun(0)
	_ = traf.AddChild(trun)
	return traf
}

type testSamples struct {
	name    string
	samples []Sample
}

func TestTrafTrunWithoutOptimization(t *testing.T) {

	tests := []testSamples{
		{
			"audioSamples",
			[]Sample{
				{SyncSampleFlags, 1024, 234, 0},
				{SyncSampleFlags, 1024, 235, 0},
				{SyncSampleFlags, 1024, 235, 0},
			},
		},
		{
			"videoWithInitialSyncSample",
			[]Sample{
				{SyncSampleFlags, 1024, 234, 0},
				{NonSyncSampleFlags, 1024, 235, 0},
				{NonSyncSampleFlags, 1024, 235, 0},
			},
		},
		{
			"videoWithMultipleSyncSamplesAndCto",
			[]Sample{
				{SyncSampleFlags, 1024, 234, 0},
				{NonSyncSampleFlags, 1024, 235, 2048},
				{SyncSampleFlags, 1024, 235, -1024},
			},
		},
		{
			"singleSample",
			[]Sample{
				{SyncSampleFlags, 1024, 234, 0},
			},
		},
		{
			"sameSize",
			[]Sample{
				{SyncSampleFlags, 1024, 234, 0},
				{SyncSampleFlags, 1024, 234, 0},
			},
		},
	}

	// Without optimizations
	optimization := false
	for _, test := range tests {
		runEncodeDecode(t, test, optimization)
	}
	// With optimizations (smaller trun box)
	optimization = true
	for _, test := range tests {
		runEncodeDecode(t, test, optimization)
	}
}

func runEncodeDecode(t *testing.T, test testSamples, withOptimization bool) {
	traf := createTestTrafBox()
	for _, s := range test.samples {
		traf.Trun.AddSample(s)
	}
	traf.Trun.DataOffset = 100 // Needs to be set. Value not important in test
	var buf bytes.Buffer
	if withOptimization {
		err := traf.OptimizeTfhdTrun()
		if err != nil {
			t.Error(err)
		}
	}
	err := traf.Encode(&buf)
	if err != nil {
		t.Error(err)
	}
	box, err := DecodeBox(0, &buf)
	if err != nil {
		t.Error(err)
	}
	outTraf := box.(*TrafBox)
	outTrun := outTraf.Trun
	trex := &TrexBox{}
	outTrun.AddSampleDefaultValues(outTraf.Tfhd, trex)
	outSamples := outTrun.Samples
	if !reflect.DeepEqual(outSamples, test.samples) {
		t.Errorf("Case %s optimization=%v failed. Got %v instead of %v",
			test.name, withOptimization, outSamples, test.samples)
	}
}
