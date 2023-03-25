package hevc

import (
	"encoding/hex"
	"github.com/go-test/deep"
	"testing"
)

const pps1 = "4401c0f7c0cc90"

func TestPPSParser(t *testing.T) {
	byteData, _ := hex.DecodeString(pps1)

	wanted := &PPS{
		CabacInitPresentFlag:               true,
		TransformSkipEnabledFlag:           true,
		CuQpDeltaEnabledFlag:               true,
		LoopFilterAcrossSlicesEnabledFlag:  true,
		DeblockingFilterControlPresentFlag: true,
	}
	spsMap := map[uint32]*SPS{
		0: nil,
	}
	got, err := ParsePPSNALUnit(byteData, spsMap)
	if err != nil {
		t.Error(err)
		return
	}
	if diff := deep.Equal(got, wanted); diff != nil {
		t.Error(diff)
	}
}
