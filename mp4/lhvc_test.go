package mp4_test

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/Eyevinn/mp4ff/hevc"
	"github.com/Eyevinn/mp4ff/mp4"
)

func TestLhvC(t *testing.T) {
	// Enhancement-layer SPS and PPS from a reference MV-HEVC mp4 (GPAC output).
	spsNalu, err := hex.DecodeString("42090e85924cae6a020202028180")
	if err != nil {
		t.Fatal(err)
	}
	ppsNalu, err := hex.DecodeString("440948572b062a0140")
	if err != nil {
		t.Fatal(err)
	}

	lhvC := mp4.CreateLhvCFromNalus([][]byte{spsNalu}, [][]byte{ppsNalu})
	boxDiffAfterEncodeAndDecode(t, lhvC)
}

func TestLhvCDecodeFromHex(t *testing.T) {
	// Full lhvC box bytes from a reference MV-HEVC mp4 (47 bytes including header).
	boxHex := "0000002f6c68764301f000ffcf02a10001000e42090e85924cae6a020202028180a200010009440948572b062a0140"
	data, err := hex.DecodeString(boxHex)
	if err != nil {
		t.Fatal(err)
	}

	box, err := mp4.DecodeBox(0, bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	lhvC, ok := box.(*mp4.LhvCBox)
	if !ok {
		t.Fatalf("expected *LhvCBox, got %T", box)
	}
	if lhvC.Type() != "lhvC" {
		t.Errorf("Type() = %q, want lhvC", lhvC.Type())
	}
	if lhvC.LengthSizeMinusOne != 3 {
		t.Errorf("LengthSizeMinusOne = %d, want 3", lhvC.LengthSizeMinusOne)
	}
	if lhvC.NumTemporalLayers != 1 {
		t.Errorf("NumTemporalLayers = %d, want 1", lhvC.NumTemporalLayers)
	}
	if len(lhvC.NaluArrays) != 2 {
		t.Fatalf("NaluArrays len = %d, want 2", len(lhvC.NaluArrays))
	}

	spsNalus := lhvC.GetNalusForType(hevc.NALU_SPS)
	if len(spsNalus) != 1 {
		t.Fatalf("SPS nalus = %d, want 1", len(spsNalus))
	}
	if hex.EncodeToString(spsNalus[0]) != "42090e85924cae6a020202028180" {
		t.Errorf("SPS nalu mismatch: got %s", hex.EncodeToString(spsNalus[0]))
	}

	boxDiffAfterEncodeAndDecode(t, lhvC)
}

// TestVisualSampleEntryWithLhvC verifies that an lhvC box decodes as a child of
// a visual sample entry (alongside hvcC) and is reachable via the LhvC pointer.
func TestVisualSampleEntryWithLhvC(t *testing.T) {
	hvc1 := mp4.CreateVisualSampleEntryBox("hvc1", 1920, 1080, nil)
	spsNalu, _ := hex.DecodeString("42090e85924cae6a020202028180")
	ppsNalu, _ := hex.DecodeString("440948572b062a0140")
	hvc1.AddChild(mp4.CreateLhvCFromNalus([][]byte{spsNalu}, [][]byte{ppsNalu}))

	boxDiffAfterEncodeAndDecode(t, hvc1)

	decoded := boxAfterEncodeAndDecode(t, hvc1).(*mp4.VisualSampleEntryBox)
	if decoded.LhvC == nil {
		t.Fatal("expected decoded lhvC child reachable via LhvC pointer")
	}
	if len(decoded.LhvC.GetNalusForType(hevc.NALU_SPS)) != 1 {
		t.Error("expected one SPS nalu in decoded lhvC")
	}
}
