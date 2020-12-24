package mp4

import (
	"testing"
)

func TestSubs(t *testing.T) {

	subs := &SubsBox{}
	e := SubsSample{SubsampleSize: 1000, SubsamplePriority: 255, Discardable: 0, CodecSpecificParameters: 0}
	subs.Entries = append(subs.Entries, SubsEntry{SampleDelta: 100, SubSamples: []SubsSample{e}})
	boxDiffAfterEncodeAndDecode(t, subs)
}

func TestSubsInfo(t *testing.T) {
	goldenDumpPath := "testdata/golden_subs_dump.txt"
	subs := &SubsBox{}
	e := SubsSample{SubsampleSize: 1000, SubsamplePriority: 255, Discardable: 0, CodecSpecificParameters: 0}
	subs.Entries = append(subs.Entries, SubsEntry{SampleDelta: 100, SubSamples: []SubsSample{e}})

	err := compareOrUpdateInfo(t, subs, goldenDumpPath)
	if err != nil {
		t.Error(err)
	}
}
