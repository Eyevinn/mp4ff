package sei

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/go-test/deep"
)

func TestRecoveryPointHevcSEI(t *testing.T) {
	testCases := []struct {
		name       string
		hexPayload string
		want       RecoveryPointHevcSEI
		wantString string
	}{
		{
			"pocCnt=0 exact",
			"d0", // 1 (se=0) 1 0 | 1 (trailing) 0000 -> 1101 0000 = 0xd0
			RecoveryPointHevcSEI{
				RecoveryPocCnt: 0,
				ExactMatchFlag: true,
				BrokenLinkFlag: false,
			},
			"SEIRecoveryPointType (6), size=1, recoveryPocCnt=0, exactMatch=true, brokenLink=false",
		},
		{
			"pocCnt=-1 broken",
			"6c", // 011 (se=-1) 0 1 | 1 (trailing) 00 -> 01101 100 = 0x6c
			RecoveryPointHevcSEI{
				RecoveryPocCnt: -1,
				ExactMatchFlag: false,
				BrokenLinkFlag: true,
			},
			"SEIRecoveryPointType (6), size=1, recoveryPocCnt=-1, exactMatch=false, brokenLink=true",
		},
	}
	for i, tc := range testCases {
		t.Run(fmt.Sprintf("case %d %s", i, tc.name), func(t *testing.T) {
			pl, err := hex.DecodeString(tc.hexPayload)
			if err != nil {
				t.Fatal(err)
			}
			seiData := NewSEIData(SEIRecoveryPointType, pl)
			msg, err := DecodeRecoveryPointHevcSEI(seiData)
			if err != nil {
				t.Error(err)
			}
			if msg.Type() != SEIRecoveryPointType {
				t.Errorf("got SEI type %d, wanted %d", msg.Type(), SEIRecoveryPointType)
			}
			rp := msg.(*RecoveryPointHevcSEI)
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

func TestRecoveryPointHevcSEIRoundTrip(t *testing.T) {
	cases := []RecoveryPointHevcSEI{
		{RecoveryPocCnt: 0, ExactMatchFlag: true, BrokenLinkFlag: false},
		{RecoveryPocCnt: 1, ExactMatchFlag: false, BrokenLinkFlag: true},
		{RecoveryPocCnt: -1, ExactMatchFlag: true, BrokenLinkFlag: true},
		{RecoveryPocCnt: 16383, ExactMatchFlag: false, BrokenLinkFlag: false},
		{RecoveryPocCnt: -16384, ExactMatchFlag: true, BrokenLinkFlag: false},
	}
	for i, c := range cases {
		t.Run(fmt.Sprintf("case %d", i), func(t *testing.T) {
			in := c
			pl := in.Payload()
			if uint(len(pl)) != in.Size() {
				t.Errorf("Size() %d != len(Payload()) %d", in.Size(), len(pl))
			}
			seiData := NewSEIData(SEIRecoveryPointType, pl)
			msg, err := DecodeRecoveryPointHevcSEI(seiData)
			if err != nil {
				t.Fatal(err)
			}
			out := msg.(*RecoveryPointHevcSEI)
			if diff := deep.Equal(in, *out); diff != nil {
				t.Error(diff)
			}
		})
	}
}

// TestRecoveryPointViaDecodeSEIMessage checks dispatch through DecodeSEIMessage for both codecs.
func TestRecoveryPointViaDecodeSEIMessage(t *testing.T) {
	avcData := NewSEIData(SEIRecoveryPointType, []byte{0xc4})
	msg, err := DecodeSEIMessage(avcData, AVC)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := msg.(*RecoveryPointAvcSEI); !ok {
		t.Errorf("expected *RecoveryPointAvcSEI, got %T", msg)
	}

	hevcData := NewSEIData(SEIRecoveryPointType, []byte{0xd0})
	msg, err = DecodeSEIMessage(hevcData, HEVC)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := msg.(*RecoveryPointHevcSEI); !ok {
		t.Errorf("expected *RecoveryPointHevcSEI, got %T", msg)
	}
}
