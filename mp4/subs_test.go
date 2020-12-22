package mp4

import (
	"bytes"
	"io/ioutil"
	"testing"
)

func TestSubs(t *testing.T) {

	subs := &SubsBox{}
	e := SubsSample{SubsampleSize: 1000, SubsamplePriority: 255, Discardable: 0, CodecSpecificParameters: 0}
	subs.Entries = append(subs.Entries, SubsEntry{SampleDelta: 100, SubSamples: []SubsSample{e}})
	boxDiffAfterEncodeAndDecode(t, subs)
}

func TestSubsDump(t *testing.T) {
	goldenAssetPath := "testdata/golden_subs_dump.txt"
	subs := &SubsBox{}
	e := SubsSample{SubsampleSize: 1000, SubsamplePriority: 255, Discardable: 0, CodecSpecificParameters: 0}
	subs.Entries = append(subs.Entries, SubsEntry{SampleDelta: 100, SubSamples: []SubsSample{e}})

	specificBoxLevels := "subs:1"
	buf := bytes.Buffer{}
	err := subs.Dump(&buf, specificBoxLevels, "", "  ")
	if err != nil {
		t.Error(err)
	}

	if *update {
		err = writeGolden(t, goldenAssetPath, buf.Bytes())
		if err != nil {
			t.Error(err)
		}
		return
	}
	got := buf.String()
	golden, err := ioutil.ReadFile(goldenAssetPath)
	if err != nil {
		t.Error(err)
	}
	want := string(golden)
	if got != want {
		t.Errorf("Got %s instead of %s", got, want)
	}
}
