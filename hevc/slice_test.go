package hevc

import (
	"os"
	"testing"

	"github.com/Eyevinn/mp4ff/avc"

	"github.com/go-test/deep"
)

func TestParseSliceHeader(t *testing.T) {
	wantedHdr := map[NaluType]SliceHeader{
		NALU_IDR_N_LP: {
			SliceType:                         SLICE_I,
			FirstSliceSegmentInPicFlag:        true,
			PicOutputFlag:                     true,
			SaoLumaFlag:                       true,
			SaoChromaFlag:                     true,
			QpDelta:                           7,
			LoopFilterAcrossSlicesEnabledFlag: true,
			NumEntryPointOffsets:              1,
			OffsetLenMinus1:                   3,
			CollocatedFromL0Flag:              true,
			EntryPointOffsetMinus1:            []uint32{12},
			Size:                              6},
		NALU_TRAIL_N: {
			SliceType:                  SLICE_B,
			FirstSliceSegmentInPicFlag: true,
			PicOutputFlag:              true,
			PicOrderCntLsb:             1,
			ShortTermRefPicSet: ShortTermRPS{
				DeltaPocS0:      []uint32{1},
				DeltaPocS1:      []uint32{2, 2},
				UsedByCurrPicS0: []bool{true},
				UsedByCurrPicS1: []bool{true, true},
				NumNegativePics: 1,
				NumPositivePics: 2,
				NumDeltaPocs:    3,
			},
			SaoLumaFlag:                       true,
			SaoChromaFlag:                     true,
			TemporalMvpEnabledFlag:            true,
			NumRefIdxActiveOverrideFlag:       true,
			NumRefIdxL0ActiveMinus1:           0,
			NumRefIdxL1ActiveMinus1:           1,
			FiveMinusMaxNumMergeCand:          2,
			QpDelta:                           10,
			LoopFilterAcrossSlicesEnabledFlag: true,
			NumEntryPointOffsets:              1,
			OffsetLenMinus1:                   1,
			CollocatedFromL0Flag:              false,
			EntryPointOffsetMinus1:            []uint32{1},
			Size:                              10,
		},
		NALU_TRAIL_R: {
			SliceType:                  SLICE_P,
			FirstSliceSegmentInPicFlag: true,
			PicOutputFlag:              true,
			PicOrderCntLsb:             5,
			ShortTermRefPicSet: ShortTermRPS{
				DeltaPocS0:      []uint32{5},
				DeltaPocS1:      []uint32{},
				UsedByCurrPicS0: []bool{true},
				UsedByCurrPicS1: []bool{},
				NumNegativePics: 1,
				NumDeltaPocs:    1,
			},
			SaoLumaFlag:            true,
			SaoChromaFlag:          true,
			TemporalMvpEnabledFlag: true,
			PredWeightTable: &PredWeightTable{
				LumaLog2WeightDenom:        7,
				DeltaChromaLog2WeightDenom: -1,
				WeightsL0: []WeightingFactors{
					{
						LumaWeightFlag:   false,
						ChromaWeightFlag: false,
					},
				},
			},
			CollocatedFromL0Flag:     true,
			FiveMinusMaxNumMergeCand: 2,
			QpDelta:                  7,
			NumEntryPointOffsets:     1,
			OffsetLenMinus1:          1,
			EntryPointOffsetMinus1:   []uint32{2},
			Size:                     10,
		},
	}
	data, err := os.ReadFile("testdata/blackframe.265")
	if err != nil {
		t.Error(err)
	}
	nalus := avc.ExtractNalusFromByteStream(data)
	spsMap := make(map[uint32]*SPS, 1)
	ppsMap := make(map[uint32]*PPS, 1)
	gotHdr := make(map[NaluType]SliceHeader)
	for _, nalu := range nalus {
		switch GetNaluType(nalu[0]) {
		case NALU_SPS:
			sps, err := ParseSPSNALUnit(nalu)
			if err != nil {
				t.Error(err)
			}
			spsMap[uint32(sps.SpsID)] = sps
		case NALU_PPS:
			pps, err := ParsePPSNALUnit(nalu, spsMap)
			if err != nil {
				t.Error(err)
			}
			ppsMap[pps.PicParameterSetID] = pps
		case NALU_IDR_N_LP, NALU_TRAIL_R, NALU_TRAIL_N:
			hdr, err := ParseSliceHeader(nalu, spsMap, ppsMap)
			if err != nil {
				t.Error(err)
			}
			gotHdr[GetNaluType(nalu[0])] = *hdr
		}
	}
	if diff := deep.Equal(wantedHdr, gotHdr); diff != nil {
		t.Errorf("Got Slice Headers: %+v\n Diff is %v", gotHdr, diff)
	}
}

// TestParseSliceHeaderDeblockingDisabledInPPS covers a stream where deblocking
// is disabled in the PPS and no slice header overrides it. Then
// slice_deblocking_filter_disabled_flag is absent and inferred from the PPS,
// which in turn makes slice_loop_filter_across_slices_enabled_flag absent.
// Reading that flag anyway left the parser mid-byte and failed byte_alignment.
// The fixture is x265 output at 64x64 with --no-deblock --no-sao, SEI removed.
func TestParseSliceHeaderDeblockingDisabledInPPS(t *testing.T) {
	data, err := os.ReadFile("testdata/deblocking_disabled.265")
	if err != nil {
		t.Fatal(err)
	}
	spsMap := make(map[uint32]*SPS, 1)
	ppsMap := make(map[uint32]*PPS, 1)
	nrSliceHeaders := 0
	for _, nalu := range avc.ExtractNalusFromByteStream(data) {
		switch GetNaluType(nalu[0]) {
		case NALU_SPS:
			sps, err := ParseSPSNALUnit(nalu)
			if err != nil {
				t.Fatal(err)
			}
			spsMap[uint32(sps.SpsID)] = sps
		case NALU_PPS:
			pps, err := ParsePPSNALUnit(nalu, spsMap)
			if err != nil {
				t.Fatal(err)
			}
			ppsMap[pps.PicParameterSetID] = pps
			// The combination that makes the slice header flag absent.
			if !pps.DeblockingFilterDisabledFlag {
				t.Fatal("fixture should disable deblocking in the PPS")
			}
			if pps.DeblockingFilterOverrideEnabledFlag {
				t.Fatal("fixture should not enable a slice header override")
			}
			if !pps.LoopFilterAcrossSlicesEnabledFlag {
				t.Fatal("fixture should enable loop filter across slices")
			}
		case NALU_IDR_N_LP:
			sh, err := ParseSliceHeader(nalu, spsMap, ppsMap)
			if err != nil {
				t.Fatal(err)
			}
			if !sh.DeblockingFilterDisabledFlag {
				t.Error("slice_deblocking_filter_disabled_flag should be inferred from the PPS")
			}
			// Absent too, and inferred from the PPS rather than left false.
			if !sh.LoopFilterAcrossSlicesEnabledFlag {
				t.Error("slice_loop_filter_across_slices_enabled_flag should be inferred from the PPS")
			}
			// pic_output_flag is absent whenever output_flag_present_flag is 0,
			// and is inferred to be 1.
			if !sh.PicOutputFlag {
				t.Error("pic_output_flag should be inferred as true")
			}
			nrSliceHeaders++
		}
	}
	if nrSliceHeaders != 1 {
		t.Errorf("got %d slice headers, wanted 1", nrSliceHeaders)
	}
}

// TestParseSliceHeaderDeblockingOffsetsInheritedFromPPS covers the deblocking
// offsets, which are absent from the slice header whenever it does not override
// the deblocking parameters, and are then inferred from the PPS. The fixture is
// x265 output at 64x64 with --no-sao --deblock 3:-2, SEI removed, giving a PPS
// with non-zero offsets and no slice header override.
func TestParseSliceHeaderDeblockingOffsetsInheritedFromPPS(t *testing.T) {
	data, err := os.ReadFile("testdata/deblocking_offsets.265")
	if err != nil {
		t.Fatal(err)
	}
	spsMap := make(map[uint32]*SPS, 1)
	ppsMap := make(map[uint32]*PPS, 1)
	nrSliceHeaders := 0
	for _, nalu := range avc.ExtractNalusFromByteStream(data) {
		switch GetNaluType(nalu[0]) {
		case NALU_SPS:
			sps, err := ParseSPSNALUnit(nalu)
			if err != nil {
				t.Fatal(err)
			}
			spsMap[uint32(sps.SpsID)] = sps
		case NALU_PPS:
			pps, err := ParsePPSNALUnit(nalu, spsMap)
			if err != nil {
				t.Fatal(err)
			}
			ppsMap[pps.PicParameterSetID] = pps
			if pps.BetaOffsetDiv2 != -2 || pps.TcOffsetDiv2 != 3 {
				t.Fatalf("fixture PPS offsets are beta=%d tc=%d, wanted beta=-2 tc=3",
					pps.BetaOffsetDiv2, pps.TcOffsetDiv2)
			}
			if pps.DeblockingFilterOverrideEnabledFlag {
				t.Fatal("fixture should not enable a slice header override")
			}
		case NALU_IDR_N_LP:
			sh, err := ParseSliceHeader(nalu, spsMap, ppsMap)
			if err != nil {
				t.Fatal(err)
			}
			if sh.DeblockingFilterOverrideFlag {
				t.Fatal("slice header should not override the deblocking parameters")
			}
			if sh.BetaOffsetDiv2 != -2 {
				t.Errorf("slice_beta_offset_div2 = %d, wanted -2 inferred from the PPS",
					sh.BetaOffsetDiv2)
			}
			if sh.TcOffsetDiv2 != 3 {
				t.Errorf("slice_tc_offset_div2 = %d, wanted 3 inferred from the PPS",
					sh.TcOffsetDiv2)
			}
			nrSliceHeaders++
		}
	}
	if nrSliceHeaders != 1 {
		t.Errorf("got %d slice headers, wanted 1", nrSliceHeaders)
	}
}
