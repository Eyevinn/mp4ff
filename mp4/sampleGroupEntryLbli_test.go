package mp4_test

import (
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

func TestLbliSgpd(t *testing.T) {
	cases := []struct {
		name string
		e    *mp4.LbliSampleGroupEntry
	}{
		{"irap", &mp4.LbliSampleGroupEntry{BlIrapPicFlag: true, BlIrapNalUnitType: 21, SampleOffset: 0}},
		{"non-irap", &mp4.LbliSampleGroupEntry{BlIrapPicFlag: false, BlIrapNalUnitType: 0, SampleOffset: 1}},
		{"negative offset", &mp4.LbliSampleGroupEntry{BlIrapPicFlag: false, BlIrapNalUnitType: 0, SampleOffset: -3}},
		{"max nal type", &mp4.LbliSampleGroupEntry{BlIrapPicFlag: true, BlIrapNalUnitType: 0x3f, SampleOffset: 127}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.e.Type() != "lbli" {
				t.Errorf("Type() = %q, want lbli", c.e.Type())
			}
			sgpd := &mp4.SgpdBox{
				Version:            2,
				GroupingType:       "lbli",
				DefaultLength:      uint32(c.e.Size()),
				SampleGroupEntries: []mp4.SampleGroupEntry{c.e},
			}
			boxDiffAfterEncodeAndDecode(t, sgpd)
		})
	}
}
