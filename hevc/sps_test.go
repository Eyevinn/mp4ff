package hevc

import (
	"encoding/hex"
	"testing"

	"github.com/go-test/deep"
)

const (
	spsNalu = "420101022000000300b0000003000003007ba0078200887db6718b92448053888892cf24a69272c9124922dc91aa48fca223ff000100016a02020201"
)

func TestSPSParser1(t *testing.T) {
	byteData, _ := hex.DecodeString(spsNalu)

	wanted := SPS{
		VpsID:                 0,
		MaxSubLayersMinus1:    0,
		TemporalIdNestingFlag: true,
		ProfileTierLevel: ProfileTierLevel{
			GeneralProfileSpace:              0,
			GeneralTierFlag:                  false,
			GeneralProfileIDC:                2,
			GeneralProfileCompatibilityFlags: 536870912,
			GeneralConstraintIndicatorFlags:  193514046488576,
			GeneralProgressiveSourceFlag:     false,
			GeneralInterlacedSourceFlag:      false,
			GeneralNonPackedConstraintFlag:   false,
			GeneralFrameOnlyConstraintFlag:   false,
			GeneralLevelIDC:                  123,
		},
		SpsID:                   0,
		ChromaFormatIDC:         1,
		SeparateColourPlaneFlag: false,
		ConformanceWindowFlag:   true,
		PicWidthInLumaSamples:   960,
		PicHeightInLumaSamples:  544,
		ConformanceWindow: ConformanceWindow{
			LeftOffset:   0,
			RightOffset:  0,
			TopOffset:    0,
			BottomOffset: 2,
		},
		BitDepthLumaMinus8:              2,
		BitDepthChromaMinus8:            2,
		Log2MaxPicOrderCntLsbMinus4:     6,
		SubLayerOrderingInfoPresentFlag: false,
		SubLayeringOrderingInfos: []SubLayerOrderingInfo{
			{
				MaxDecPicBufferingMinus1: 5,
				MaxNumReorderPics:        4,
				MaxLatencyIncreasePlus1:  0,
			},
		},
		Log2MinLumaCodingBlockSizeMinus3:     0,
		Log2DiffMaxMinLumaCodingBlockSize:    3,
		Log2MinLumaTransformBlockSizeMinus2:  0,
		Log2DiffMaxMinLumaTransformBlockSize: 3,
		MaxTransformHierarchyDepthInter:      1,
		MaxTransformHierarchyDepthIntra:      1,
		ScalingListEnabledFlag:               false,
		ScalingListDataPresentFlag:           false,
		AmpEnabledFlag:                       false,
		SampleAdaptiveOffsetEnabledFlag:      false,
		PCMEnabledFlag:                       false,
		NumShortTermRefPicSets:               9,
		LongTermRefPicsPresentFlag:           false,
		SpsTemporalMvpEnabledFlag:            false,
		StrongIntraSmoothingEnabledFlag:      false,
		VUIParametersPresentFlag:             false,
	}
	got, err := ParseSPSNALUnit(byteData)
	if err != nil {
		t.Error("Error parsing SPS")
	}
	if diff := deep.Equal(*got, wanted); diff != nil {
		t.Error(diff)
	}
	gotWidth, gotHeight := got.ImageSize()
	var expWidth, expHeight uint32 = 960, 540
	if gotWidth != expWidth || gotHeight != expHeight {
		t.Errorf("Got %dx%d instead of %dx%d", gotWidth, gotHeight, expWidth, expHeight)
	}
}

func TestCodecString(t *testing.T) {
	expected := "hvc1.1.6.L120.90"
	byteData, _ := hex.DecodeString("420101016000000300900000030000030078a0021c801e0596566924caf01680800001f480003a9804")
	sps, err := ParseSPSNALUnit(byteData)
	if err != nil {
		t.Error(err)
	}
	if sps.CodecString() != expected {
		t.Errorf("expected %s, got %s", expected, sps.CodecString())
	}
}
