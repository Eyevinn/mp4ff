package avc

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/go-test/deep"
)

const pps1 = "68e84332c8b0"

func TestPPSParser(t *testing.T) {
	byteData, _ := hex.DecodeString(pps1)

	wanted := &PPS{
		PicParameterSetID:                     0,
		SeqParameterSetID:                     0,
		EntropyCodingModeFlag:                 true,
		BottomFieldPicOrderInFramePresentFlag: false,
		NumSliceGroupsMinus1:                  0,
		NumRefIdxI0DefaultActiveMinus1:        15,
		NumRefIdxI1DefaultActiveMinus1:        0,
		WeightedPredFlag:                      true,
		WeightedBipredIDC:                     0,
		PicInitQpMinus26:                      0,
		PicInitQsMinus26:                      0,
		ChromaQpIndexOffset:                   -2,
		DeblockingFilterControlPresentFlag:    true,
		ConstrainedIntraPredFlag:              false,
		RedundantPicCntPresentFlag:            false,
		Transform8x8ModeFlag:                  true,
		PicScalingMatrixPresentFlag:           false,
		PicScalingLists:                       nil,
		SecondChromaQpIndexOffset:             -2,
	}
	got, err := ParsePPSNALUnit(byteData, nil)
	if err != nil {
		t.Error(err)
		return
	}
	if diff := deep.Equal(got, wanted); diff != nil {
		t.Error(diff)
	}
}

// TestPPSParserNumSliceGroupsOutOfRange verifies that an out-of-range
// num_slice_groups_minus1 is rejected up front. Without the bound check, the
// run_length_minus1 loop keeps appending for every signalled slice group, so a
// 7-byte NAL unit made ParsePPSNALUnit allocate over 10 GiB.
func TestPPSParserNumSliceGroupsOutOfRange(t *testing.T) {
	// num_slice_groups_minus1 = 1 << 20, slice_group_map_type = 0
	byteData, err := hex.DecodeString("28c0000080000e")
	if err != nil {
		t.Fatal(err)
	}
	pps, err := ParsePPSNALUnit(byteData, nil)
	if err == nil {
		t.Fatal("expected an error for out-of-range num_slice_groups_minus1")
	}
	if !strings.Contains(err.Error(), "num_slice_groups_minus1") {
		t.Errorf("expected a num_slice_groups_minus1 error, got %q", err.Error())
	}
	if pps != nil {
		t.Errorf("expected nil PPS on error, got %+v", pps)
	}
}
