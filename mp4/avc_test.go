package mp4

import (
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// From ~tobbe/content/encmompass/dazn_ad/video_4400kbps/init.cmfv (dropped 67)
const sps1 = "640020accac05005bb0169e0000003002000000c9c4c000432380008647c12401cb1c31380"

// From test in repackaging-poc

const sps2 = "64000dacd941419f9e10000003001000000303c0f1429960"

func TestSPSParser1(t *testing.T) {
	byteData, _ := hex.DecodeString(sps1)

	wanted := AvcSPS{
		Profile:                         100,
		ProfileCompatibility:            0,
		Level:                           32,
		ParameterID:                     0,
		ChromaFormatIDC:                 1,
		SeparateColourPlaneFlag:         0,
		BitDepthLumaMinus8:              0,
		BitDepthChromaMinus8:            0,
		QPPrimeYZeroTransformBypassFlag: 0,
		SeqScalingMatrixPresentFlag:     0,
		Log2MaxFrameNumMinus4:           0,
		PicOrderCntType:                 0,
		Log2MaxPicOrderCntLsbMinus4:     4,
		DeltaPicOrderAlwaysZeroFlag:     0,
		OffsetForNonRefPic:              0,
		RefFramesInPicOrderCntCycle:     nil,
		NumRefFrames:                    2,
		GapsInFrameNumValueAllowedFlag:  0,
		FrameMbsOnlyFlag:                1,
		MbAdaptiveFrameFieldFlag:        0,
		Direct8x8InferenceFlag:          1,
		FrameCroppingFlag:               0,
		FrameCropLeftOffset:             0,
		FrameCropRightOffset:            0,
		FrameCropTopOffset:              0,
		FrameCropBottomOffset:           0,
		Width:                           1280,
		Height:                          720,
		SampleAspectRatioWidth:          1,
		SampleAspectRatioHeight:         1,
	}
	got, err := ParseSPS(byteData)
	fmt.Println(got)
	if err != nil {
		t.Errorf("Error parsing SPS")
	}
	if !cmp.Equal(*got, wanted) {
		t.Errorf("SPS got %+v is not same as wanted %+v", *got, wanted)

	}
}

func TestSPSParser2(t *testing.T) {
	byteData, _ := hex.DecodeString(sps2)

	wanted := AvcSPS{
		Profile:                         100,
		ProfileCompatibility:            0,
		Level:                           13,
		ParameterID:                     0,
		ChromaFormatIDC:                 1,
		SeparateColourPlaneFlag:         0,
		BitDepthLumaMinus8:              0,
		BitDepthChromaMinus8:            0,
		QPPrimeYZeroTransformBypassFlag: 0,
		SeqScalingMatrixPresentFlag:     0,
		Log2MaxFrameNumMinus4:           0,
		PicOrderCntType:                 0,
		Log2MaxPicOrderCntLsbMinus4:     2,
		DeltaPicOrderAlwaysZeroFlag:     0,
		OffsetForNonRefPic:              0,
		RefFramesInPicOrderCntCycle:     nil,
		NumRefFrames:                    4,
		GapsInFrameNumValueAllowedFlag:  0,
		FrameMbsOnlyFlag:                1,
		MbAdaptiveFrameFieldFlag:        0,
		Direct8x8InferenceFlag:          1,
		FrameCroppingFlag:               1,
		FrameCropLeftOffset:             0,
		FrameCropRightOffset:            0,
		FrameCropTopOffset:              0,
		FrameCropBottomOffset:           6,
		Width:                           320,
		Height:                          180,
		SampleAspectRatioWidth:          0,
		SampleAspectRatioHeight:         0,
	}
	got, err := ParseSPS(byteData)
	fmt.Println(got)
	if err != nil {
		t.Errorf("Error parsing SPS")
	}
	if !cmp.Equal(*got, wanted) {
		t.Errorf("SPS got %+v is not same as\n wanted %+v", *got, wanted)
	}
}
