package hevc

import (
	"encoding/hex"
	"testing"
)

func TestParseVPSNALUnit(t *testing.T) {
	// VPS from reference MV-HEVC MP4 (gpac output of x.hevc)
	vpsHex := "40010c11ffff016000000300900000030000030078959815bf7820001828b2e0c040000013f100000300000f11a0f0008714010a566e90"
	data, err := hex.DecodeString(vpsHex)
	if err != nil {
		t.Fatal(err)
	}

	vps, err := ParseVPSNALUnit(data)
	if err != nil {
		t.Fatal(err)
	}

	if vps.VpsID != 0 {
		t.Errorf("VpsID = %d, want 0", vps.VpsID)
	}
	if vps.MaxLayersMinus1 != 1 {
		t.Errorf("MaxLayersMinus1 = %d, want 1", vps.MaxLayersMinus1)
	}
	if vps.GetNumLayers() != 2 {
		t.Errorf("GetNumLayers() = %d, want 2", vps.GetNumLayers())
	}
	if !vps.IsMultiLayer() {
		t.Error("expected IsMultiLayer() to be true")
	}
	if vps.GetNumViews() != 2 {
		t.Errorf("GetNumViews() = %d, want 2", vps.GetNumViews())
	}
	if vps.ScalabilityMaskBits() != 0x0002 {
		t.Errorf("ScalabilityMaskBits() = 0x%04x, want 0x0002", vps.ScalabilityMaskBits())
	}
	if vps.NumProfileTierLevel < 3 {
		t.Errorf("NumProfileTierLevel = %d, want >= 3", vps.NumProfileTierLevel)
	}
	if vps.NumOutputLayerSets != 2 {
		t.Errorf("NumOutputLayerSets = %d, want 2", vps.NumOutputLayerSets)
	}
}

func TestParseVPSSingleLayer(t *testing.T) {
	// Standard single-layer HEVC VPS
	vpsHex := "40010c01ffff022000000300b0000003000003007b18b024"
	data, err := hex.DecodeString(vpsHex)
	if err != nil {
		t.Fatal(err)
	}

	vps, err := ParseVPSNALUnit(data)
	if err != nil {
		t.Fatal(err)
	}

	if vps.MaxLayersMinus1 != 0 {
		t.Errorf("MaxLayersMinus1 = %d, want 0", vps.MaxLayersMinus1)
	}
	if vps.IsMultiLayer() {
		t.Error("expected IsMultiLayer() to be false for single-layer VPS")
	}
	if vps.GetNumViews() != 1 {
		t.Errorf("GetNumViews() = %d, want 1", vps.GetNumViews())
	}
}
