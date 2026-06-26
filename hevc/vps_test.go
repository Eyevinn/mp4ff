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

func TestVPSMultiLayerExtension(t *testing.T) {
	// MV-HEVC VPS with vps_extension() from a reference MV-HEVC mp4 (GPAC output).
	vpsHex := "40010c11ffff016000000300900000030000030078959815bf7820001828b2e0c040000013f100000300000f11a0f0008714010a566e90"
	data, err := hex.DecodeString(vpsHex)
	if err != nil {
		t.Fatal(err)
	}
	vps, err := ParseVPSNALUnit(data)
	if err != nil {
		t.Fatalf("Error parsing VPS: %v", err)
	}
	if vps.VpsID != 0 {
		t.Errorf("VpsID = %d, want 0", vps.VpsID)
	}
	if vps.MaxLayersMinus1 != 1 {
		t.Errorf("MaxLayersMinus1 = %d, want 1", vps.MaxLayersMinus1)
	}
	if got := vps.GetNumLayers(); got != 2 {
		t.Errorf("GetNumLayers() = %d, want 2", got)
	}
	if !vps.IsMultiLayer() {
		t.Error("expected IsMultiLayer() to be true")
	}
	if vps.Extension == nil {
		t.Fatal("expected non-nil VPS extension")
	}
	if got := vps.GetNumViews(); got != 2 {
		t.Errorf("GetNumViews() = %d, want 2", got)
	}
	if got := vps.ScalabilityMaskBits(); got != 0x0002 {
		t.Errorf("ScalabilityMaskBits() = 0x%04x, want 0x0002", got)
	}
	if vps.Extension.NumProfileTierLevel < 3 {
		t.Errorf("NumProfileTierLevel = %d, want >= 3", vps.Extension.NumProfileTierLevel)
	}
	if vps.Extension.NumOutputLayerSets != 2 {
		t.Errorf("NumOutputLayerSets = %d, want 2", vps.Extension.NumOutputLayerSets)
	}
	if vps.Extension.NumRepFormats != 1 {
		t.Fatalf("NumRepFormats = %d, want 1", vps.Extension.NumRepFormats)
	}
	rf := vps.Extension.RepFormats[0]
	if rf.PicWidthLumaSamples != 1920 || rf.PicHeightLumaSamples != 1080 {
		t.Errorf("RepFormat resolution = %dx%d, want 1920x1080",
			rf.PicWidthLumaSamples, rf.PicHeightLumaSamples)
	}
	// Layer 1 (dependent view) must reference layer 0 (base view).
	if !vps.Extension.DirectDependencyFlag[1][0] {
		t.Error("expected layer 1 to directly depend on layer 0")
	}
	if vps.Extension.NumDirectRefLayers[1] != 1 {
		t.Errorf("NumDirectRefLayers[1] = %d, want 1", vps.Extension.NumDirectRefLayers[1])
	}
}

func TestVPSSingleLayerNoExtension(t *testing.T) {
	// Standard single-layer HEVC VPS (no vps_extension()).
	vpsHex := "40010c01ffff022000000300b0000003000003007b18b024"
	data, err := hex.DecodeString(vpsHex)
	if err != nil {
		t.Fatal(err)
	}
	vps, err := ParseVPSNALUnit(data)
	if err != nil {
		t.Fatalf("Error parsing VPS: %v", err)
	}
	if vps.MaxLayersMinus1 != 0 {
		t.Errorf("MaxLayersMinus1 = %d, want 0", vps.MaxLayersMinus1)
	}
	if vps.IsMultiLayer() {
		t.Error("expected IsMultiLayer() to be false for single-layer VPS")
	}
	if vps.Extension != nil {
		t.Error("expected nil VPS extension for single-layer VPS")
	}
	if got := vps.GetNumViews(); got != 1 {
		t.Errorf("GetNumViews() = %d, want 1", got)
	}
	if got := vps.ScalabilityMaskBits(); got != 0 {
		t.Errorf("ScalabilityMaskBits() = 0x%04x, want 0", got)
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
