package mp4_test

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

func createTestTrafBox() *mp4.TrafBox {
	traf := &mp4.TrafBox{}
	tfhd := &mp4.TfhdBox{}
	_ = traf.AddChild(tfhd)
	trun := mp4.CreateTrun(0)
	_ = traf.AddChild(trun)
	return traf
}

type testSamples struct {
	name    string
	samples []mp4.Sample
}

func TestTrafTrunWithoutOptimization(t *testing.T) {

	tests := []testSamples{
		{
			"audioSamples",
			[]mp4.Sample{
				{mp4.SyncSampleFlags, 1024, 234, 0},
				{mp4.SyncSampleFlags, 1024, 235, 0},
				{mp4.SyncSampleFlags, 1024, 235, 0},
			},
		},
		{
			"videoWithInitialSyncSample",
			[]mp4.Sample{
				{mp4.SyncSampleFlags, 1024, 234, 0},
				{mp4.NonSyncSampleFlags, 1024, 235, 0},
				{mp4.NonSyncSampleFlags, 1024, 235, 0},
			},
		},
		{
			"videoWithMultipleSyncSamplesAndCto",
			[]mp4.Sample{
				{mp4.SyncSampleFlags, 1024, 234, 0},
				{mp4.NonSyncSampleFlags, 1024, 235, 2048},
				{mp4.SyncSampleFlags, 1024, 235, -1024},
			},
		},
		{
			"singleSample",
			[]mp4.Sample{
				{mp4.SyncSampleFlags, 1024, 234, 0},
			},
		},
		{
			"sameSize",
			[]mp4.Sample{
				{mp4.SyncSampleFlags, 1024, 234, 0},
				{mp4.SyncSampleFlags, 1024, 234, 0},
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
	box, err := mp4.DecodeBox(0, &buf)
	if err != nil {
		t.Error(err)
	}
	outTraf := box.(*mp4.TrafBox)
	outTrun := outTraf.Trun
	trex := &mp4.TrexBox{}
	outTrun.AddSampleDefaultValues(outTraf.Tfhd, trex)
	outSamples := outTrun.Samples
	if !reflect.DeepEqual(outSamples, test.samples) {
		t.Errorf("Case %s optimization=%v failed. Got %v instead of %v",
			test.name, withOptimization, outSamples, test.samples)
	}
}
