package mp4

import (
	"testing"
)

func TestSgpd(t *testing.T) {

	rollEntry := &SgpdRollEntry{RollDistance: -1}
	rapEntry := &SgpdRapEntry{NumLeadingSamplesKnown: 1, NumLeadingSamples: 12}
	alstEntry := &SgpdAlstEntry{RollCount: 2, FirstOutputSample: 1, SampleOffset: []uint32{7000, 1234}}
	genericEntry := &SgpdGenericEntry{Payload: []byte{0x80}}
	genericEntry2 := &SgpdGenericEntry{Payload: []byte{0x00}}

	sgpds := []*SgpdBox{
		CreateSgpdBox(1, "roll", 2, 0, []SgpdEntry{rollEntry}),
		CreateSgpdBox(1, "rap ", 1, 0, []SgpdEntry{rapEntry}),
		CreateSgpdBox(1, "alst", 12, 0, []SgpdEntry{alstEntry}),
		CreateSgpdBox(1, "tele", 1, 0, []SgpdEntry{genericEntry, genericEntry2}),
	}
	for _, sgpd := range sgpds {
		boxDiffAfterEncodeAndDecode(t, sgpd)
	}

}
