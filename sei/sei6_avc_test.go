package sei

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/go-test/deep"
)

func TestRecoveryPointAvcSEI(t *testing.T) {
	testCases := []struct {
		name       string
		hexPayload string
		want       RecoveryPointAvcSEI
		wantString string
	}{
		{
			"frameCnt=0 exact",
			"c4", // 1 (ue=0) 1 0 00 | 1 (trailing) 00 -> 11000 100 = 0xc4
			RecoveryPointAvcSEI{
				RecoveryFrameCnt:      0,
				ExactMatchFlag:        true,
				BrokenLinkFlag:        false,
				ChangingSliceGroupIdc: 0,
			},
			"SEIRecoveryPointType (6), size=1, recoveryFrameCnt=0, exactMatch=true, brokenLink=false, changingSliceGroupIdc=0",
		},
		{
			"frameCnt=3 broken idc=2",
			"2340", // 00100 (ue=3) 0 1 10 | 1 0000000 -> 00100011 01000000
			RecoveryPointAvcSEI{
				RecoveryFrameCnt:      3,
				ExactMatchFlag:        false,
				BrokenLinkFlag:        true,
				ChangingSliceGroupIdc: 2,
			},
			"SEIRecoveryPointType (6), size=2, recoveryFrameCnt=3, exactMatch=false, brokenLink=true, changingSliceGroupIdc=2",
		},
	}
	for i, tc := range testCases {
		t.Run(fmt.Sprintf("case %d %s", i, tc.name), func(t *testing.T) {
			pl, err := hex.DecodeString(tc.hexPayload)
			if err != nil {
				t.Fatal(err)
			}
			seiData := NewSEIData(SEIRecoveryPointType, pl)
			msg, err := DecodeRecoveryPointAvcSEI(seiData)
			if err != nil {
				t.Error(err)
			}
			if msg.Type() != SEIRecoveryPointType {
				t.Errorf("got SEI type %d, wanted %d", msg.Type(), SEIRecoveryPointType)
			}
			rp := msg.(*RecoveryPointAvcSEI)
			if diff := deep.Equal(*rp, tc.want); diff != nil {
				t.Error(diff)
			}
			if msg.String() != tc.wantString {
				t.Errorf("got %q, wanted %q", msg.String(), tc.wantString)
			}
			decPl := msg.Payload()
			if !bytes.Equal(decPl, pl) {
				t.Errorf("payload differs: got %s, wanted %s", hex.EncodeToString(decPl), tc.hexPayload)
			}
		})
	}
}

func TestRecoveryPointAvcSEIRoundTrip(t *testing.T) {
	cases := []RecoveryPointAvcSEI{
		{RecoveryFrameCnt: 0, ExactMatchFlag: true, BrokenLinkFlag: false, ChangingSliceGroupIdc: 0},
		{RecoveryFrameCnt: 1, ExactMatchFlag: false, BrokenLinkFlag: true, ChangingSliceGroupIdc: 1},
		{RecoveryFrameCnt: 255, ExactMatchFlag: true, BrokenLinkFlag: true, ChangingSliceGroupIdc: 2},
		{RecoveryFrameCnt: 65535, ExactMatchFlag: false, BrokenLinkFlag: false, ChangingSliceGroupIdc: 0},
	}
	for i, c := range cases {
		t.Run(fmt.Sprintf("case %d", i), func(t *testing.T) {
			in := c
			pl := in.Payload()
			if uint(len(pl)) != in.Size() {
				t.Errorf("Size() %d != len(Payload()) %d", in.Size(), len(pl))
			}
			seiData := NewSEIData(SEIRecoveryPointType, pl)
			msg, err := DecodeRecoveryPointAvcSEI(seiData)
			if err != nil {
				t.Fatal(err)
			}
			out := msg.(*RecoveryPointAvcSEI)
			if diff := deep.Equal(in, *out); diff != nil {
				t.Error(diff)
			}
		})
	}
}
