package mp4_test

import (
	"encoding/hex"
	"testing"

	"github.com/Eyevinn/mp4ff/hevc"
	"github.com/Eyevinn/mp4ff/mp4"
)

// stereoVPSHex is a real MV-HEVC stereo VPS (GPAC output) with a vps_extension():
// 2 layers, 2 views, scalability_mask 0x0002, 3 profile-tier-levels, 2 output
// layer sets, one 1920x1080 rep format, layer 1 depends on layer 0.
const stereoVPSHex = "40010c11ffff016000000300900000030000030078959815bf7820001828b2e0c040000013f100000300000f11a0f0008714010a566e90"

func parseStereoVPS(t *testing.T) *hevc.VPS {
	t.Helper()
	data, err := hex.DecodeString(stereoVPSHex)
	if err != nil {
		t.Fatal(err)
	}
	vps, err := hevc.ParseVPSNALUnit(data)
	if err != nil {
		t.Fatalf("parse VPS: %v", err)
	}
	return vps
}

func TestBuildOinfFromVPS(t *testing.T) {
	vps := parseStereoVPS(t)
	oinf, err := mp4.BuildOinfFromVPS(vps)
	if err != nil {
		t.Fatal(err)
	}
	if oinf.ScalabilityMask != 0x0002 {
		t.Errorf("ScalabilityMask = 0x%04x, want 0x0002", oinf.ScalabilityMask)
	}
	if len(oinf.ProfileTierLevels) != 3 {
		t.Errorf("ProfileTierLevels = %d, want 3", len(oinf.ProfileTierLevels))
	}
	if len(oinf.OperatingPoints) != 2 {
		t.Fatalf("OperatingPoints = %d, want 2", len(oinf.OperatingPoints))
	}
	// OLS 0 is the base layer operating point: a single output layer (nuh_layer_id
	// 0, ptl_idx 1) at full resolution.
	op0 := oinf.OperatingPoints[0]
	if len(op0.Layers) != 1 {
		t.Fatalf("OP[0] layers = %d, want 1", len(op0.Layers))
	}
	if op0.Layers[0].LayerID != 0 || op0.Layers[0].PtlIdx != 1 || !op0.Layers[0].IsOutputLayer {
		t.Errorf("OP[0] layer = %+v, want layerID 0, ptlIdx 1, output", op0.Layers[0])
	}
	if op0.MaxPicWidth != 1920 || op0.MaxPicHeight != 1080 {
		t.Errorf("OP[0] max dims = %dx%d, want 1920x1080", op0.MaxPicWidth, op0.MaxPicHeight)
	}
	// OLS 1 contains both layers (nuh_layer_id 0 and 1, ptl_idx 1 and 2).
	op1 := oinf.OperatingPoints[1]
	if len(op1.Layers) != 2 {
		t.Fatalf("OP[1] layers = %d, want 2", len(op1.Layers))
	}
	if op1.Layers[0].LayerID != 0 || op1.Layers[1].LayerID != 1 {
		t.Errorf("OP[1] layer ids = %d,%d, want 0,1", op1.Layers[0].LayerID, op1.Layers[1].LayerID)
	}
	if op1.Layers[0].PtlIdx != 1 || op1.Layers[1].PtlIdx != 2 {
		t.Errorf("OP[1] ptl idx = %d,%d, want 1,2", op1.Layers[0].PtlIdx, op1.Layers[1].PtlIdx)
	}
	if op1.MaxPicWidth != 1920 || op1.MaxPicHeight != 1080 {
		t.Errorf("OP[1] max dims = %dx%d, want 1920x1080", op1.MaxPicWidth, op1.MaxPicHeight)
	}
	// Two dependency layers: layer 0 (independent) and layer 1 (depends on 0).
	if len(oinf.DependencyLayers) != 2 {
		t.Fatalf("DependencyLayers = %d, want 2", len(oinf.DependencyLayers))
	}
	if len(oinf.DependencyLayers[0].DependsOnLayers) != 0 {
		t.Errorf("layer 0 should be independent, got deps %v", oinf.DependencyLayers[0].DependsOnLayers)
	}
	if got := oinf.DependencyLayers[1].DependsOnLayers; len(got) != 1 || got[0] != 0 {
		t.Errorf("layer 1 deps = %v, want [0]", got)
	}

	// Round-trip the built entry through an sgpd.
	sgpd := &mp4.SgpdBox{Version: 2, GroupingType: "oinf", DefaultLength: uint32(oinf.Size()),
		SampleGroupEntries: []mp4.SampleGroupEntry{oinf}}
	boxDiffAfterEncodeAndDecode(t, sgpd)
}

func TestBuildLinfFromVPS(t *testing.T) {
	vps := parseStereoVPS(t)
	linf, err := mp4.BuildLinfFromVPS(vps, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(linf.Layers) != 2 {
		t.Fatalf("Layers = %d, want 2", len(linf.Layers))
	}
	if linf.Layers[0].LayerID != 0 || linf.Layers[1].LayerID != 1 {
		t.Errorf("layer ids = %d,%d, want 0,1", linf.Layers[0].LayerID, linf.Layers[1].LayerID)
	}

	sgpd := &mp4.SgpdBox{Version: 2, GroupingType: "linf", DefaultLength: uint32(linf.Size()),
		SampleGroupEntries: []mp4.SampleGroupEntry{linf}}
	boxDiffAfterEncodeAndDecode(t, sgpd)
}

func TestBuildFromVPSRequiresExtension(t *testing.T) {
	// Single-layer VPS has no extension; the builders must return an error.
	data, _ := hex.DecodeString("40010c01ffff022000000300b0000003000003007b18b024")
	vps, err := hevc.ParseVPSNALUnit(data)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mp4.BuildOinfFromVPS(vps); err == nil {
		t.Error("expected error for single-layer VPS (oinf)")
	}
	if _, err := mp4.BuildLinfFromVPS(vps, nil); err == nil {
		t.Error("expected error for single-layer VPS (linf)")
	}
}
