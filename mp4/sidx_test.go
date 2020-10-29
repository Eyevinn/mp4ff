package mp4

import (
	"bytes"
	"testing"

	"github.com/go-test/deep"
)

func TestSidx(t *testing.T) {

	sidx := &SidxBox{}

	sidx.ReferenceID = 1
	sidx.Timescale = 48000
	sidx.EarliestPresentationTime = 12
	sidx.FirstOffset = 1024

	ref := SidxRef{
		ReferenceType:      0, // Media
		ReferencedSize:     2048,
		SubSegmentDuration: 1024 * 15,
		StartsWithSAP:      1,
		SAPType:            1,
		SAPDeltaTime:       0,
	}
	sidx.SidxRefs = append(sidx.SidxRefs, ref)

	buf := bytes.Buffer{}
	err := sidx.Encode(&buf)
	if err != nil {
		t.Error(err)
	}

	sidxDec, err := DecodeBox(0, &buf)
	if err != nil {
		t.Error(err)
	}

	if diff := deep.Equal(sidxDec, sidx); diff != nil {
		t.Error(diff)
	}
}
