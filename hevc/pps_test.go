package hevc

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/go-test/deep"
)

func TestPPSParser(t *testing.T) {
	testCases := []struct {
		hexData string
		spsID   uint32
		wanted  PPS
	}{
		{
			"4401c0f7c0cc90",
			0,
			PPS{
				CabacInitPresentFlag:               true,
				TransformSkipEnabledFlag:           true,
				CuQpDeltaEnabledFlag:               true,
				LoopFilterAcrossSlicesEnabledFlag:  true,
				DeblockingFilterControlPresentFlag: true,
			},
		},
		{
			"4401c172b46240",
			0,
			PPS{
				SignDataHidingEnabledFlag:         true,
				CuQpDeltaEnabledFlag:              true,
				DiffCuQpDeltaDepth:                1,
				WeightedPredFlag:                  true,
				LoopFilterAcrossSlicesEnabledFlag: true,
				EntropyCodingSyncEnabledFlag:      true,
			},
		},
		{
			"4401c1ac9383b240",
			0,
			PPS{
				CabacInitPresentFlag:                true,
				NumRefIdxL0DefaultActiveMinus1:      1,
				SignDataHidingEnabledFlag:           true,
				CuQpDeltaEnabledFlag:                true,
				DiffCuQpDeltaDepth:                  3,
				DeblockingFilterControlPresentFlag:  true,
				DeblockingFilterOverrideEnabledFlag: true,
				LoopFilterAcrossSlicesEnabledFlag:   true,
				SliceChromaQpOffsetsPresentFlag:     true,
			},
		},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("case %d_%s", i, tc.hexData), func(t *testing.T) {

			byteData, err := hex.DecodeString(tc.hexData)
			if err != nil {
				t.Error(err)
			}
			spsMap := map[uint32]*SPS{
				tc.spsID: nil,
			}
			got, err := ParsePPSNALUnit(byteData, spsMap)
			if err != nil {
				t.Error(err)
				return
			}
			if diff := deep.Equal(&tc.wanted, got); diff != nil {
				t.Error(diff)
			}
		})
	}
}

func TestPPSNumRefIdxDefaultRange(t *testing.T) {
	spsMap := map[uint32]*SPS{0: {SpsID: 0}}
	cases := []struct {
		nrRefIdxMinus1 uint
		wantErr        string
	}{
		{0, ""},
		{14, ""},
		{15, "num_ref_idx_l0_default_active_minus1 = 15 is outside the range 0 to 14"},
		{255, "num_ref_idx_l0_default_active_minus1 = 255 is outside the range 0 to 14"},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("num_ref_idx_l0_default_active_minus1=%d", c.nrRefIdxMinus1), func(t *testing.T) {
			buf := bytes.Buffer{}
			w := bits.NewEBSPWriter(&buf)
			w.Write(uint(NALU_PPS)<<9|0x01, 16) // nal_unit_header
			w.WriteExpGolomb(0)                 // pps_pic_parameter_set_id
			w.WriteExpGolomb(0)                 // pps_seq_parameter_set_id
			w.Write(0, 1)                       // dependent_slice_segments_enabled_flag
			w.Write(0, 1)                       // output_flag_present_flag
			w.Write(0, 3)                       // num_extra_slice_header_bits
			w.Write(0, 1)                       // sign_data_hiding_enabled_flag
			w.Write(0, 1)                       // cabac_init_present_flag
			w.WriteExpGolomb(c.nrRefIdxMinus1)  // num_ref_idx_l0_default_active_minus1
			w.WriteExpGolomb(0)                 // num_ref_idx_l1_default_active_minus1
			w.WriteExpGolomb(0)                 // init_qp_minus26
			w.Write(0, 3)                       // constrained_intra_pred, transform_skip, cu_qp_delta_enabled
			w.WriteExpGolomb(0)                 // pps_cb_qp_offset
			w.WriteExpGolomb(0)                 // pps_cr_qp_offset
			w.Write(0, 6)                       // slice_chroma_qp_offsets_present .. entropy_coding_sync_enabled
			w.Write(0, 1)                       // pps_loop_filter_across_slices_enabled_flag
			w.Write(0, 1)                       // deblocking_filter_control_present_flag
			w.Write(0, 1)                       // pps_scaling_list_data_present_flag
			w.Write(0, 1)                       // lists_modification_present_flag
			w.WriteExpGolomb(0)                 // log2_parallel_merge_level_minus2
			w.Write(0, 1)                       // slice_segment_header_extension_present_flag
			w.Write(0, 1)                       // pps_extension_present_flag
			w.WriteRbspTrailingBits()
			if w.AccError() != nil {
				t.Fatal(w.AccError())
			}

			pps, err := ParsePPSNALUnit(buf.Bytes(), spsMap)
			if c.wantErr != "" {
				if err == nil || err.Error() != c.wantErr {
					t.Errorf("got error %v, wanted %q", err, c.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if uint(pps.NumRefIdxL0DefaultActiveMinus1) != c.nrRefIdxMinus1 {
				t.Errorf("NumRefIdxL0DefaultActiveMinus1 = %d, wanted %d",
					pps.NumRefIdxL0DefaultActiveMinus1, c.nrRefIdxMinus1)
			}
		})
	}
}
