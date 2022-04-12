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

	wantedVUI := VUIParameters{
		SampleAspectRatioWidth:     1,
		SampleAspectRatioHeight:    1,
		VideoSignalTypePresentFlag: true,
		VideoFormat:                5,
		ColourDescriptionFlag:      true,
		ColourPrimaries:            1,
		TransferCharacteristics:    1,
		MatrixCoefficients:         1,
	}
	wanted := SPS{
		VpsID:                 0,
		MaxSubLayersMinus1:    0,
		TemporalIDNestingFlag: true,
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
		ShortTermRefPicSets: []ShortTermRPS{
			{
				DeltaPocS0:      []uint32{8, 8},
				DeltaPocS1:      []uint32{},
				UsedByCurrPicS0: []bool{true, true},
				UsedByCurrPicS1: []bool{},
				NumNegativePics: 2,
				NumPositivePics: 0,
				NumDeltaPocs:    2,
			},
			{
				DeltaPocS0:      []uint32{1},
				DeltaPocS1:      []uint32{7},
				UsedByCurrPicS0: []bool{true},
				UsedByCurrPicS1: []bool{true},
				NumNegativePics: 1,
				NumPositivePics: 1,
				NumDeltaPocs:    2,
			},
			{
				DeltaPocS0:      []uint32{2},
				DeltaPocS1:      []uint32{6},
				UsedByCurrPicS0: []bool{true},
				UsedByCurrPicS1: []bool{true},
				NumNegativePics: 1,
				NumPositivePics: 1,
				NumDeltaPocs:    2,
			},
			{
				DeltaPocS0:      []uint32{3},
				DeltaPocS1:      []uint32{5},
				UsedByCurrPicS0: []bool{true},
				UsedByCurrPicS1: []bool{true},
				NumNegativePics: 1,
				NumPositivePics: 1,
				NumDeltaPocs:    2,
			},
			{
				DeltaPocS0:      []uint32{4},
				DeltaPocS1:      []uint32{4},
				UsedByCurrPicS0: []bool{true},
				UsedByCurrPicS1: []bool{true},
				NumNegativePics: 1,
				NumPositivePics: 1,
				NumDeltaPocs:    2,
			},
			{
				DeltaPocS0:      []uint32{5},
				DeltaPocS1:      []uint32{3},
				UsedByCurrPicS0: []bool{true},
				UsedByCurrPicS1: []bool{true},
				NumNegativePics: 1,
				NumPositivePics: 1,
				NumDeltaPocs:    2,
			},
			{
				DeltaPocS0:      []uint32{6},
				DeltaPocS1:      []uint32{2},
				UsedByCurrPicS0: []bool{true},
				UsedByCurrPicS1: []bool{true},
				NumNegativePics: 1,
				NumPositivePics: 1,
				NumDeltaPocs:    2,
			},
			{
				DeltaPocS0:      []uint32{7},
				DeltaPocS1:      []uint32{1},
				UsedByCurrPicS0: []bool{true},
				UsedByCurrPicS1: []bool{true},
				NumNegativePics: 1,
				NumPositivePics: 1,
				NumDeltaPocs:    2,
			},
			{
				DeltaPocS0:      []uint32{8},
				DeltaPocS1:      []uint32{},
				UsedByCurrPicS0: []bool{true},
				UsedByCurrPicS1: []bool{},
				NumNegativePics: 1,
				NumPositivePics: 0,
				NumDeltaPocs:    1,
			},
		},
		LongTermRefPicsPresentFlag:      false,
		SpsTemporalMvpEnabledFlag:       false,
		StrongIntraSmoothingEnabledFlag: false,
		VUIParametersPresentFlag:        true,
		VUI:                             &wantedVUI,
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
