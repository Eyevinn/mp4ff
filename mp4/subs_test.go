package mp4

import (
	"bytes"
	"testing"

	"github.com/go-test/deep"
)

func TestSubs(t *testing.T) {

	subs := &SubsBox{}
	e := SubsSample{SubsampleSize: 1000, SubsamplePriority: 255, Discardable: 0, CodecSpecificParameters: 0}
	subs.Entries = append(subs.Entries, SubsEntry{SampleDelta: 100, SubSamples: []SubsSample{e}})

	buf := bytes.Buffer{}
	err := subs.Encode(&buf)
	if err != nil {
		t.Error(err)
	}

	subsDec, err := DecodeBox(0, &buf)
	if err != nil {
		t.Error(err)
	}

	if diff := deep.Equal(subsDec, subs); diff != nil {
		t.Error(diff)
	}
}
