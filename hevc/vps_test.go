package hevc

import (
	"encoding/hex"
	"testing"

	"github.com/go-test/deep"
)

func TestVPSParser(t *testing.T) {
	testCases := []struct {
		name      string
		hexData   string
		wantedVPS VPS
	}{
		{
			name:    "VPS basic - Main profile level 4",
			hexData: "40010c01ffff016000000300900000030000030078959809",
			wantedVPS: VPS{
				VpsID:                  0,
				BaseLayerInternalFlag:  true,
				BaseLayerAvailableFlag: true,
				MaxLayersMinus1:        0,
				MaxSubLayersMinus1:     0,
				TemporalIdNestingFlag:  true,
				ProfileTierLevel: ProfileTierLevel{
					GeneralProfileSpace:              0,
					GeneralTierFlag:                  false,
					GeneralProfileIDC:                1,
					GeneralProfileCompatibilityFlags: 0x60000000,
					GeneralProgressiveSourceFlag:     true,
					GeneralFrameOnlyConstraintFlag:   true,
					GeneralConstraintIndicatorFlags:  0x900000000000,
					GeneralLevelIDC:                  120,
				},
				SubLayerOrderingInfoPresentFlag: true,
				MaxDecPicBufferingMinus1:        []uint{4},
				MaxNumReorderPics:               []uint{2},
				MaxLatencyIncreasePlus1:         []uint{5},
				MaxLayerID:                      0,
				NumLayerSetsMinus1:              0,
				TimingInfoPresentFlag:           false,
			},
		},
		{
			name:    "VPS with timing info",
			hexData: "40010c01ffff01600000030090000003000003005dac0c0000030004000003006540",
			wantedVPS: VPS{
				VpsID:                  0,
				BaseLayerInternalFlag:  true,
				BaseLayerAvailableFlag: true,
				MaxLayersMinus1:        0,
				MaxSubLayersMinus1:     0,
				TemporalIdNestingFlag:  true,
				ProfileTierLevel: ProfileTierLevel{
					GeneralProfileSpace:              0,
					GeneralTierFlag:                  false,
					GeneralProfileIDC:                1,
					GeneralProfileCompatibilityFlags: 0x60000000,
					GeneralProgressiveSourceFlag:     true,
					GeneralFrameOnlyConstraintFlag:   true,
					GeneralConstraintIndicatorFlags:  0x900000000000,
					GeneralLevelIDC:                  93,
				},
				SubLayerOrderingInfoPresentFlag: true,
				MaxDecPicBufferingMinus1:        []uint{1},
				MaxNumReorderPics:               []uint{0},
				MaxLatencyIncreasePlus1:         []uint{0},
				MaxLayerID:                      0,
				NumLayerSetsMinus1:              0,
				TimingInfoPresentFlag:           true,
				TimingInfo: &VPSTimingInfo{
					NumUnitsInTick:              1,
					TimeScale:                   25,
					PocProportionalToTimingFlag: false,
					NumHrdParameters:            0,
				},
				ExtensionFlag: false,
			},
		},
		{
			name:    "ffmpeg 50i interlaced with timing info",
			hexData: "40010c01ffff0140000003004000000300000300783c0c00000fa000030d4140",
			wantedVPS: VPS{
				VpsID:                  0,
				BaseLayerInternalFlag:  true,
				BaseLayerAvailableFlag: true,
				MaxLayersMinus1:        0,
				MaxSubLayersMinus1:     0,
				TemporalIdNestingFlag:  true,
				ProfileTierLevel: ProfileTierLevel{
					GeneralProfileIDC:                1,
					GeneralProfileCompatibilityFlags: 0x40000000,
					GeneralInterlacedSourceFlag:      true,
					GeneralConstraintIndicatorFlags:  0x400000000000,
					GeneralLevelIDC:                  120,
				},
				SubLayerOrderingInfoPresentFlag: false,
				MaxDecPicBufferingMinus1:        []uint{2},
				MaxNumReorderPics:               []uint{0},
				MaxLatencyIncreasePlus1:         []uint{0},
				MaxLayerID:                      0,
				NumLayerSetsMinus1:              0,
				TimingInfoPresentFlag:           true,
				TimingInfo: &VPSTimingInfo{
					NumUnitsInTick:              1000,
					TimeScale:                   50000,
					PocProportionalToTimingFlag: false,
					NumHrdParameters:            0,
				},
				ExtensionFlag: false,
			},
		},
		{
			name:    "VPS with timing and HRD parameters",
			hexData: "40010c01ffff016000000300900000030000030078ac0c000003000400000300c8a680",
			wantedVPS: VPS{
				VpsID:                  0,
				BaseLayerInternalFlag:  true,
				BaseLayerAvailableFlag: true,
				MaxLayersMinus1:        0,
				MaxSubLayersMinus1:     0,
				TemporalIdNestingFlag:  true,
				ProfileTierLevel: ProfileTierLevel{
					GeneralProfileIDC:                1,
					GeneralProfileCompatibilityFlags: 0x60000000,
					GeneralProgressiveSourceFlag:     true,
					GeneralFrameOnlyConstraintFlag:   true,
					GeneralConstraintIndicatorFlags:  0x900000000000,
					GeneralLevelIDC:                  120,
				},
				SubLayerOrderingInfoPresentFlag: true,
				MaxDecPicBufferingMinus1:        []uint{1},
				MaxNumReorderPics:               []uint{0},
				MaxLatencyIncreasePlus1:         []uint{0},
				MaxLayerID:                      0,
				NumLayerSetsMinus1:              0,
				TimingInfoPresentFlag:           true,
				TimingInfo: &VPSTimingInfo{
					NumUnitsInTick:              1,
					TimeScale:                   50,
					PocProportionalToTimingFlag: false,
					NumHrdParameters:            1,
					HrdParameters: []*HrdParameters{
						{
							SubLayerHrd: []SubLayerHrd{
								{
									FixedPicRateGeneralFlag:   true,
									FixedPicRateWithinCvsFlag: true,
									CpbCntMinus1:              1,
								},
							},
						},
					},
				},
				ExtensionFlag: false,
			},
		},
	}
	for _, c := range testCases {
		t.Run(c.name, func(t *testing.T) {
			data, err := hex.DecodeString(c.hexData)
			if err != nil {
				t.Fatalf("Error decoding hex string: %v", err)
			}
			gotVPS, err := ParseVPSNALUnit(data)
			if err != nil {
				t.Fatalf("Error parsing VPS: %v", err)
			}
			if diff := deep.Equal(*gotVPS, c.wantedVPS); diff != nil {
				t.Error(diff)
			}
		})
	}
}

func TestVPSParseError(t *testing.T) {
	// SPS NALU type instead of VPS
	spsHex := "42010101600000030090000003000003007ba003c080109640"
	data, err := hex.DecodeString(spsHex)
	if err != nil {
		t.Fatal(err)
	}
	_, err = ParseVPSNALUnit(data)
	if err == nil {
		t.Error("expected error for non-VPS NALU type")
	}
}
