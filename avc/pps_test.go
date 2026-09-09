package avc

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
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

// writeSignedGolomb writes an se(v) value using the codeNum mapping of
// ISO/IEC 14496-10 Section 9.1.1.
func writeSignedGolomb(w *bits.EBSPWriter, value int) {
	if value > 0 {
		w.WriteExpGolomb(uint(2*value - 1))
		return
	}
	w.WriteExpGolomb(uint(-2 * value))
}

// ppsWithScalingMatrix builds a PPS NAL unit with pic_scaling_matrix_present_flag
// set and nrLists pic_scaling_list_present_flag values, of which only the first
// signals a list (a flat 4x4 list with all values 16). chroma_qp_index_offset and
// second_chroma_qp_index_offset are both -2, so a wrong list count is visible
// either as a bad offset or as a failing rbsp_trailing_bits check.
func ppsWithScalingMatrix(t *testing.T, transform8x8 bool, nrLists int) []byte {
	t.Helper()
	buf := bytes.Buffer{}
	w := bits.NewEBSPWriter(&buf)
	w.Write(0x68, 8)         // NAL header: nal_ref_idc=3, nal_unit_type=8 (PPS)
	w.WriteExpGolomb(0)      // pic_parameter_set_id
	w.WriteExpGolomb(0)      // seq_parameter_set_id
	w.Write(1, 1)            // entropy_coding_mode_flag
	w.Write(0, 1)            // bottom_field_pic_order_in_frame_present_flag
	w.WriteExpGolomb(0)      // num_slice_groups_minus1
	w.WriteExpGolomb(0)      // num_ref_idx_l0_default_active_minus1
	w.WriteExpGolomb(0)      // num_ref_idx_l1_default_active_minus1
	w.Write(0, 1)            // weighted_pred_flag
	w.Write(0, 2)            // weighted_bipred_idc
	writeSignedGolomb(w, 0)  // pic_init_qp_minus26
	writeSignedGolomb(w, 0)  // pic_init_qs_minus26
	writeSignedGolomb(w, -2) // chroma_qp_index_offset
	w.Write(1, 1)            // deblocking_filter_control_present_flag
	w.Write(0, 1)            // constrained_intra_pred_flag
	w.Write(0, 1)            // redundant_pic_cnt_present_flag
	if transform8x8 {
		w.Write(1, 1) // transform_8x8_mode_flag
	} else {
		w.Write(0, 1)
	}
	w.Write(1, 1) // pic_scaling_matrix_present_flag
	for i := 0; i < nrLists; i++ {
		if i > 0 {
			w.Write(0, 1) // pic_scaling_list_present_flag[i]
			continue
		}
		w.Write(1, 1)           // pic_scaling_list_present_flag[0]
		writeSignedGolomb(w, 8) // delta_scale lifting the first value from 8 to 16
		for j := 1; j < 16; j++ {
			writeSignedGolomb(w, 0)
		}
	}
	writeSignedGolomb(w, -2) // second_chroma_qp_index_offset
	w.WriteRbspTrailingBits()
	if w.AccError() != nil {
		t.Fatal(w.AccError())
	}
	return buf.Bytes()
}

// TestPPSParserScalingMatrix verifies the number of pic scaling lists read when
// pic_scaling_matrix_present_flag is set. It is
// 6 + ((chroma_format_idc != 3) ? 2 : 6) * transform_8x8_mode_flag
// (ISO/IEC 14496-10 Section 7.3.2.2), so the six 4x4 lists are present even
// without 8x8 transform mode, in which case chroma_format_idc, and hence the
// SPS, is not needed at all.
func TestPPSParserScalingMatrix(t *testing.T) {
	flat4x4 := ScalingList(make([]int, 16))
	for i := range flat4x4 {
		flat4x4[i] = 16
	}
	cases := []struct {
		desc            string
		transform8x8    bool
		chromaFormatIDC byte
		spsMap          map[uint32]*SPS
		wantNrLists     int
	}{
		{
			desc:        "no 8x8 transform mode, no SPS needed",
			wantNrLists: 6,
		},
		{
			desc:            "8x8 transform mode, 4:2:0",
			transform8x8:    true,
			chromaFormatIDC: 1,
			wantNrLists:     8,
		},
		{
			desc:            "8x8 transform mode, 4:4:4",
			transform8x8:    true,
			chromaFormatIDC: 3,
			wantNrLists:     12,
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			var spsMap map[uint32]*SPS
			if c.transform8x8 {
				spsMap = map[uint32]*SPS{0: {ChromaFormatIDC: c.chromaFormatIDC}}
			}
			data := ppsWithScalingMatrix(t, c.transform8x8, c.wantNrLists)
			pps, err := ParsePPSNALUnit(data, spsMap)
			if err != nil {
				t.Fatal(err)
			}
			if !pps.PicScalingMatrixPresentFlag {
				t.Error("pic_scaling_matrix_present_flag not set")
			}
			if pps.Transform8x8ModeFlag != c.transform8x8 {
				t.Errorf("transform_8x8_mode_flag is %t, wanted %t", pps.Transform8x8ModeFlag, c.transform8x8)
			}
			if len(pps.PicScalingLists) != c.wantNrLists {
				t.Fatalf("got %d pic scaling lists, wanted %d", len(pps.PicScalingLists), c.wantNrLists)
			}
			if diff := deep.Equal(pps.PicScalingLists[0], flat4x4); diff != nil {
				t.Error(diff)
			}
			for i := 1; i < len(pps.PicScalingLists); i++ {
				if pps.PicScalingLists[i] != nil {
					t.Errorf("pic scaling list %d is %v, wanted nil", i, pps.PicScalingLists[i])
				}
			}
			if pps.ChromaQpIndexOffset != -2 {
				t.Errorf("chroma_qp_index_offset is %d, wanted -2", pps.ChromaQpIndexOffset)
			}
			if pps.SecondChromaQpIndexOffset != -2 {
				t.Errorf("second_chroma_qp_index_offset is %d, wanted -2", pps.SecondChromaQpIndexOffset)
			}
		})
	}
}

// TestPPSParserScalingMatrixMissingSPS verifies that a missing SPS is only an
// error when chroma_format_idc is actually needed, i.e. when both
// pic_scaling_matrix_present_flag and transform_8x8_mode_flag are set.
func TestPPSParserScalingMatrixMissingSPS(t *testing.T) {
	data := ppsWithScalingMatrix(t, true, 8)
	_, err := ParsePPSNALUnit(data, nil)
	if err == nil {
		t.Fatal("expected an error for a missing SPS")
	}
	if !strings.Contains(err.Error(), "sps ID 0 not found") {
		t.Errorf("expected a missing SPS error, got %q", err.Error())
	}
}
