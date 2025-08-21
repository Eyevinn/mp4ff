package avc_test

import (
	"encoding/hex"
	"testing"

	"github.com/Eyevinn/mp4ff/avc"
	"github.com/go-test/deep"
)

const (
	sps1nalu = "67640020accac05005bb0169e0000003002000000c9c4c000432380008647c12401cb1c31380"
	sps2nalu = "6764000dacd941419f9e10000003001000000303c0f1429960"
	sps3nalu = "27640020ac2ec05005bb011000000300100000078e840016e300005b8d8bdef83b438627"
	sps4nalu = "674d00329a64015005fff8037010101400000fa000013883a1800fee0003fb52ef2e343001fdc0007f6a5de5c280"
)

func TestSPSParser1(t *testing.T) {
	byteData, _ := hex.DecodeString(sps1nalu)

	wanted := avc.SPS{
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
		VUI: &avc.VUIParameters{
			SampleAspectRatioWidth:      1,
			SampleAspectRatioHeight:     1,
			VideoSignalTypePresentFlag:  true,
			VideoFormat:                 5,
			ChromaLocInfoPresentFlag:    true,
			TimingInfoPresentFlag:       true,
			NumUnitsInTick:              1,
			TimeScale:                   100,
			FixedFrameRateFlag:          true,
			NalHrdParametersPresentFlag: true,
			NalHrdParameters: &avc.HrdParameters{
				CpbCountMinus1: 0,
				BitRateScale:   1,
				CpbSizeScale:   3,
				CpbEntries: []avc.CpbEntry{{
					34374, 34374, true,
				}},
				InitialCpbRemovalDelayLengthMinus1: 16,
				CpbRemovalDelayLengthMinus1:        9,
				DpbOutputDelayLengthMinus1:         4,
				TimeOffsetLength:                   0,
			},
			PicStructPresentFlag:               true,
			BitstreamRestrictionFlag:           true,
			MotionVectorsOverPicBoundariesFlag: true,
			MaxBytesPerPicDenom:                4,
			MaxBitsPerMbDenom:                  0,
			Log2MaxMvLengthHorizontal:          13,
			Log2MaxMvLengthVertical:            11,
			MaxNumReorderFrames:                1,
			MaxDecFrameBuffering:               2,
		},
	}
	got, err := avc.ParseSPSNALUnit(byteData, true)
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

	wanted := avc.SPS{
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
		VUI: &avc.VUIParameters{
			SampleAspectRatioWidth:             0,
			SampleAspectRatioHeight:            0,
			TimingInfoPresentFlag:              true,
			NumUnitsInTick:                     1,
			TimeScale:                          60,
			BitstreamRestrictionFlag:           true,
			MotionVectorsOverPicBoundariesFlag: true,
			Log2MaxMvLengthHorizontal:          9,
			Log2MaxMvLengthVertical:            9,
			MaxNumReorderFrames:                2,
			MaxDecFrameBuffering:               4,
		},
	}
	got, err := avc.ParseSPSNALUnit(byteData, true)
	got.NrBytesBeforeVUI = 0
	got.NrBytesRead = 0
	if err != nil {
		t.Error("Error parsing SPS")
	}
	if diff := deep.Equal(*got, wanted); diff != nil {
		t.Error(diff)
	}
}

func TestSPSParser3(t *testing.T) {
	byteData, _ := hex.DecodeString(sps3nalu)

	wanted := avc.SPS{
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
		Log2MaxFrameNumMinus4:           4,
		PicOrderCntType:                 0,
		Log2MaxPicOrderCntLsbMinus4:     0,
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
		VUI: &avc.VUIParameters{
			SampleAspectRatioWidth:      1,
			SampleAspectRatioHeight:     1,
			TimingInfoPresentFlag:       true,
			NumUnitsInTick:              1,
			TimeScale:                   120,
			FixedFrameRateFlag:          true,
			NalHrdParametersPresentFlag: true,
			NalHrdParameters: &avc.HrdParameters{
				CpbCountMinus1: 0,
				BitRateScale:   4,
				CpbSizeScale:   2,
				CpbEntries: []avc.CpbEntry{{
					5858, 187499, false,
				}},
				InitialCpbRemovalDelayLengthMinus1: 23,
				CpbRemovalDelayLengthMinus1:        23,
				DpbOutputDelayLengthMinus1:         23,
				TimeOffsetLength:                   24,
			},
			PicStructPresentFlag:               true,
			BitstreamRestrictionFlag:           true,
			MotionVectorsOverPicBoundariesFlag: true,
			MaxBytesPerPicDenom:                2,
			MaxBitsPerMbDenom:                  1,
			Log2MaxMvLengthHorizontal:          13,
			Log2MaxMvLengthVertical:            11,
			MaxNumReorderFrames:                1,
			MaxDecFrameBuffering:               2,
		},
	}
	got, err := avc.ParseSPSNALUnit(byteData, true)
	got.NrBytesBeforeVUI = 0
	got.NrBytesRead = 0
	if err != nil {
		t.Error("Error parsing SPS")
	}
	if diff := deep.Equal(*got, wanted); diff != nil {
		t.Error(diff)
	}
}

func TestSPSParser4(t *testing.T) {
	byteData, _ := hex.DecodeString(sps4nalu)

	wanted := avc.SPS{
		Profile:                         77,
		ProfileCompatibility:            0,
		Level:                           50,
		ParameterID:                     0,
		ChromaFormatIDC:                 1,
		SeparateColourPlaneFlag:         false,
		BitDepthLumaMinus8:              0,
		BitDepthChromaMinus8:            0,
		QPPrimeYZeroTransformBypassFlag: false,
		SeqScalingMatrixPresentFlag:     false,
		Log2MaxFrameNumMinus4:           5,
		PicOrderCntType:                 0,
		Log2MaxPicOrderCntLsbMinus4:     5,
		DeltaPicOrderAlwaysZeroFlag:     false,
		OffsetForNonRefPic:              0,
		RefFramesInPicOrderCntCycle:     nil,
		NumRefFrames:                    1,
		GapsInFrameNumValueAllowedFlag:  false,
		FrameMbsOnlyFlag:                true,
		MbAdaptiveFrameFieldFlag:        false,
		Direct8x8InferenceFlag:          true,
		FrameCroppingFlag:               true,
		FrameCropLeftOffset:             0,
		FrameCropRightOffset:            0,
		FrameCropTopOffset:              0,
		FrameCropBottomOffset:           0,
		Width:                           2688,
		Height:                          1520,
		VUI: &avc.VUIParameters{
			SampleAspectRatioWidth:      1,
			SampleAspectRatioHeight:     1,
			VideoSignalTypePresentFlag:  true,
			VideoFormat:                 5,
			VideoFullRangeFlag:          true,
			ColourDescriptionFlag:       true,
			ColourPrimaries:             1,
			TransferCharacteristics:     1,
			MatrixCoefficients:          1,
			TimingInfoPresentFlag:       true,
			NumUnitsInTick:              1000,
			TimeScale:                   20000,
			FixedFrameRateFlag:          true,
			NalHrdParametersPresentFlag: true,
			NalHrdParameters: &avc.HrdParameters{
				CpbCountMinus1: 0,
				BitRateScale:   4,
				CpbSizeScale:   3,
				CpbEntries: []avc.CpbEntry{{
					4077, 32617, false,
				}},
				InitialCpbRemovalDelayLengthMinus1: 23,
				CpbRemovalDelayLengthMinus1:        15,
				DpbOutputDelayLengthMinus1:         5,
				TimeOffsetLength:                   24,
			},
			VclHrdParametersPresentFlag: true,
			VclHrdParameters: &avc.HrdParameters{
				BitRateScale: 4,
				CpbSizeScale: 3,
				CpbEntries: []avc.CpbEntry{{
					4077, 32617, false,
				}},
				InitialCpbRemovalDelayLengthMinus1: 23,
				CpbRemovalDelayLengthMinus1:        15,
				DpbOutputDelayLengthMinus1:         5,
				TimeOffsetLength:                   24,
			},
			PicStructPresentFlag:               true,
			BitstreamRestrictionFlag:           false,
			MotionVectorsOverPicBoundariesFlag: false,
			MaxBytesPerPicDenom:                0,
			MaxBitsPerMbDenom:                  0,
			Log2MaxMvLengthHorizontal:          0,
			Log2MaxMvLengthVertical:            0,
			MaxNumReorderFrames:                0,
			MaxDecFrameBuffering:               0,
		},
	}
	got, err := avc.ParseSPSNALUnit(byteData, true)
	got.NrBytesBeforeVUI = 0
	got.NrBytesRead = 0
	if err != nil {
		t.Error("Error parsing SPS", err)
	}
	if diff := deep.Equal(*got, wanted); diff != nil {
		t.Error(diff)
	}
}

func TestCodecString(t *testing.T) {
	spsRaw, _ := hex.DecodeString(sps1nalu)
	sps, _ := avc.ParseSPSNALUnit(spsRaw, true)
	codec := avc.CodecString("avc3", sps)
	expected := "avc3.640020"
	if codec != expected {
		t.Errorf("expected codec: %q, got %q", expected, codec)
	}
}
