package mp4_test

import (
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

func createDolbyVisionInit(t *testing.T, descriptorType string, includePS bool) (*mp4.InitSegment, error) {
	t.Helper()
	vps, _ := hex.DecodeString(hevcVPSnalu)
	sps, _ := hex.DecodeString(hevcSPSnalu)
	pps, _ := hex.DecodeString(hevcPPSnalu)
	init := mp4.CreateEmptyInit()
	init.AddEmptyTrack(180000, "video", "und")
	trak := init.Moov.Trak
	err := trak.SetHEVCDescriptor(descriptorType, [][]byte{vps}, [][]byte{sps}, [][]byte{pps}, nil, includePS)
	return init, err
}

// TestDolbyVisionDescriptor checks that dvh1/dvhe sample entries are created and
// round-trip like their hvc1/hev1 counterparts. dvh1 requires complete parameter
// sets (like hvc1), while dvhe may omit them (like hev1).
func TestDolbyVisionDescriptor(t *testing.T) {
	init, err := createDolbyVisionInit(t, "dvh1", true)
	if err != nil {
		t.Fatal(err)
	}
	hvcX := init.Moov.Trak.Mdia.Minf.Stbl.Stsd.HvcX
	if hvcX == nil {
		t.Fatal("expected HvcX to be set for dvh1")
	}
	if hvcX.Type() != "dvh1" {
		t.Errorf("got sample entry type %q, want dvh1", hvcX.Type())
	}
	if hvcX.HvcC == nil {
		t.Fatal("expected hvcC child in dvh1 sample entry")
	}

	// dvh1 without parameter sets must be rejected (like hvc1).
	if _, err := createDolbyVisionInit(t, "dvh1", false); err == nil {
		t.Error("expected error for dvh1 without parameter sets")
	}

	// dvhe may omit complete parameter sets (like hev1).
	initHE, err := createDolbyVisionInit(t, "dvhe", false)
	if err != nil {
		t.Fatalf("dvhe without complete parameter sets should be allowed: %v", err)
	}
	if got := initHE.Moov.Trak.Mdia.Minf.Stbl.Stsd.HvcX.Type(); got != "dvhe" {
		t.Errorf("got sample entry type %q, want dvhe", got)
	}

	// A Dolby Vision sample entry carries a Dolby Vision Configuration Box
	// (dvcC for dv_profile <= 7) alongside the hvcC.
	dvcc := mp4.CreateDoViConfigurationBox(1, 0, 5, 6, true, false, true, 1)
	hvcX.AddChild(dvcc)
	if hvcX.DoViConfig == nil {
		t.Fatal("expected DoViConfig child to be set on dvh1 sample entry")
	}

	// The dvh1 sample entry (hvcC + dvcC) must encode and decode back to an
	// identical box, exercising the dvh1 and dvcC registrations.
	boxDiffAfterEncodeAndDecode(t, hvcX)
}

// TestDoViConfigurationBox checks round-trips of the Dolby Vision Configuration
// Box for both the dvcC form (dv_profile <= 7) and the dvvC form (dv_profile > 7).
func TestDoViConfigurationBox(t *testing.T) {
	dvcc := mp4.CreateDoViConfigurationBox(1, 0, 5, 6, true, false, true, 1)
	if dvcc.Type() != "dvcC" {
		t.Errorf("got box type %q, want dvcC for dv_profile 5", dvcc.Type())
	}
	boxDiffAfterEncodeAndDecode(t, dvcc)

	dvvc := mp4.CreateDoViConfigurationBox(1, 0, 8, 9, true, true, false, 0)
	if dvvc.Type() != "dvvC" {
		t.Errorf("got box type %q, want dvvC for dv_profile 8", dvvc.Type())
	}
	boxDiffAfterEncodeAndDecode(t, dvvc)

	dvwc := mp4.CreateDoViConfigurationBox(1, 0, 10, 0, true, false, true, 0)
	if dvwc.Type() != "dvwC" {
		t.Errorf("got box type %q, want dvwC for dv_profile 10", dvwc.Type())
	}
	boxDiffAfterEncodeAndDecode(t, dvwc)

	data := encodeBox(t, dvcc)
	changeBoxSizeAndAssertError(t, data, 0, uint32(dvcc.Size()-1),
		fmt.Sprintf("decode dvcC pos 0: invalid box size %d", dvcc.Size()-1))
}
