package mp4

import (
	"encoding/hex"
	"testing"

	"github.com/go-test/deep"
)

// From ~tobbe/content/encmompass/dazn_ad/video_4400kbps/init.cmfv (dropped 67)
const sps1nalu = "67640020accac05005bb0169e0000003002000000c9c4c000432380008647c12401cb1c31380"

// From test in repackaging-poc

const sps2nalu = "6764000dacd941419f9e10000003001000000303c0f1429960"

func TestSPSParser1(t *testing.T) {
	byteData, _ := hex.DecodeString(sps1nalu)

	wanted := AvcSPS{
		Profile:                         100,
		ProfileCompatibility:            0,
		Level:                           32,
		ParameterID:                     0,
		ChromaFormatIDC:                 1,
		SeparateColourPlaneFlag:         false,
		BitDepthLumaMinus8:              0,
		BitDepthChromaMinus8:            0,
		QPPrimeYZeroTransformBypassFlag: false,
		SeqScalingMatrixPresentFlag:     false,
		Log2MaxFrameNumMinus4:           0,
		PicOrderCntType:                 0,
		Log2MaxPicOrderCntLsbMinus4:     4,
		DeltaPicOrderAlwaysZeroFlag:     false,
		OffsetForNonRefPic:              0,
		RefFramesInPicOrderCntCycle:     nil,
		NumRefFrames:                    2,
		GapsInFrameNumValueAllowedFlag:  false,
		FrameMbsOnlyFlag:                true,
		MbAdaptiveFrameFieldFlag:        false,
		Direct8x8InferenceFlag:          true,
		FrameCroppingFlag:               false,
		FrameCropLeftOffset:             0,
		FrameCropRightOffset:            0,
		FrameCropTopOffset:              0,
		FrameCropBottomOffset:           0,
		Width:                           1280,
		Height:                          720,
		VUI: VUIParameters{
			SampleAspectRatioWidth:  1,
			SampleAspectRatioHeight: 1,
		},
	}
	got, err := ParseSPSNALUnit(byteData)
	got.NrBytesBeforeVUI = 0
	got.NrBytesRead = 0
	if err != nil {
		t.Error("Error parsing SPS")
	}
	if diff := deep.Equal(*got, wanted); diff != nil {
		t.Error(diff)
	}
}

func TestSPSParser2(t *testing.T) {
	byteData, _ := hex.DecodeString(sps2nalu)

	wanted := AvcSPS{
		Profile:                         100,
		ProfileCompatibility:            0,
		Level:                           13,
		ParameterID:                     0,
		ChromaFormatIDC:                 1,
		SeparateColourPlaneFlag:         false,
		BitDepthLumaMinus8:              0,
		BitDepthChromaMinus8:            0,
		QPPrimeYZeroTransformBypassFlag: false,
		SeqScalingMatrixPresentFlag:     false,
		Log2MaxFrameNumMinus4:           0,
		PicOrderCntType:                 0,
		Log2MaxPicOrderCntLsbMinus4:     2,
		DeltaPicOrderAlwaysZeroFlag:     false,
		OffsetForNonRefPic:              0,
		RefFramesInPicOrderCntCycle:     nil,
		NumRefFrames:                    4,
		GapsInFrameNumValueAllowedFlag:  false,
		FrameMbsOnlyFlag:                true,
		MbAdaptiveFrameFieldFlag:        false,
		Direct8x8InferenceFlag:          true,
		FrameCroppingFlag:               true,
		FrameCropLeftOffset:             0,
		FrameCropRightOffset:            0,
		FrameCropTopOffset:              0,
		FrameCropBottomOffset:           6,
		Width:                           320,
		Height:                          180,
		VUI: VUIParameters{
			SampleAspectRatioWidth:  0,
			SampleAspectRatioHeight: 0,
		},
	}
	got, err := ParseSPSNALUnit(byteData)
	got.NrBytesBeforeVUI = 0
	got.NrBytesRead = 0
	if err != nil {
		t.Error("Error parsing SPS")
	}
	if diff := deep.Equal(*got, wanted); diff != nil {
		t.Error(diff)
	}
}
