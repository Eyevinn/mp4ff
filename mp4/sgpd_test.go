package mp4

import (
	"testing"
)

func TestSgpd(t *testing.T) {

	rollEntry := &RollSampleGroupEntry{RollDistance: -1}
	rapEntry := &RapSampleGroupEntry{NumLeadingSamplesKnown: 1, NumLeadingSamples: 12}
	alstEntry := &AlstSampleGroupEntry{RollCount: 2, FirstOutputSample: 1, SampleOffset: []uint32{7000, 1234}}
	unknownEntry := &UnknownSampleGroupEntry{Name: "tele", Data: []byte{0x80}}
	unknownEntry2 := &UnknownSampleGroupEntry{Name: "tele", Data: []byte{0x00}}

	sgpds := []*SgpdBox{
		{Version: 1, GroupingType: "roll", DefaultLength: 2, SampleGroupEntries: []SampleGroupEntry{rollEntry}},
		{Version: 1, GroupingType: "rap ", DefaultLength: 1, SampleGroupEntries: []SampleGroupEntry{rapEntry}},
		{Version: 1, GroupingType: "alst", DefaultLength: 12, SampleGroupEntries: []SampleGroupEntry{alstEntry}},
		{Version: 1, GroupingType: "tele", DefaultLength: 1, SampleGroupEntries: []SampleGroupEntry{unknownEntry, unknownEntry2}},
	}

	for _, sgpd := range sgpds {
		boxDiffAfterEncodeAndDecode(t, sgpd)
	}

}
