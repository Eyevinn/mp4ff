package mp4_test

import (
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

func TestVexuRoundTrip(t *testing.T) {
	vexu := mp4.CreateVexuBox(mp4.StriHasLeftEyeView|mp4.StriHasRightEyeView, 1, 63500, "rect")
	boxDiffAfterEncodeAndDecode(t, vexu)
}

func TestVexuReversedEyes(t *testing.T) {
	flags := byte(mp4.StriHasLeftEyeView | mp4.StriHasRightEyeView | mp4.StriEyeViewsReversed)
	vexu := mp4.CreateVexuBox(flags, 2, 70000, "rect")
	boxDiffAfterEncodeAndDecode(t, vexu)
}

func TestStri(t *testing.T) {
	stri := &mp4.StriBox{StereoFlags: mp4.StriHasLeftEyeView | mp4.StriHasRightEyeView}
	if !stri.HasLeftEye() || !stri.HasRightEye() {
		t.Error("expected HasLeftEye and HasRightEye")
	}
	if stri.HasAdditionalViews() || stri.EyeViewsReversed() {
		t.Error("expected no additional views and not reversed")
	}
	boxDiffAfterEncodeAndDecode(t, stri)

	// All four defined flags set.
	full := byte(mp4.StriHasLeftEyeView | mp4.StriHasRightEyeView | mp4.StriHasAdditionalViews | mp4.StriEyeViewsReversed)
	stri2 := &mp4.StriBox{StereoFlags: full}
	if !stri2.HasAdditionalViews() {
		t.Error("expected HasAdditionalViews")
	}
	if !stri2.EyeViewsReversed() {
		t.Error("expected EyeViewsReversed")
	}
	boxDiffAfterEncodeAndDecode(t, stri2)
}

func TestHero(t *testing.T) {
	for eye, want := range map[byte]string{0: "none", 1: "left", 2: "right", 3: "reserved(3)"} {
		hero := &mp4.HeroBox{HeroEye: eye}
		if hero.HeroEyeName() != want {
			t.Errorf("HeroEyeName(%d) = %q, want %q", eye, hero.HeroEyeName(), want)
		}
		boxDiffAfterEncodeAndDecode(t, hero)
	}
}

func TestBlin(t *testing.T) {
	boxDiffAfterEncodeAndDecode(t, &mp4.BlinBox{Baseline: 63500})
}

func TestPrji(t *testing.T) {
	boxDiffAfterEncodeAndDecode(t, &mp4.PrjiBox{ProjectionType: "rect"})
	boxDiffAfterEncodeAndDecode(t, &mp4.PrjiBox{ProjectionType: "equi"})
}

func TestHfov(t *testing.T) {
	boxDiffAfterEncodeAndDecode(t, &mp4.HfovBox{FieldOfView: 104000}) // 104 degrees
}

func TestEyesContainer(t *testing.T) {
	eyes := &mp4.EyesBox{}
	eyes.AddChild(&mp4.StriBox{StereoFlags: mp4.StriHasLeftEyeView | mp4.StriHasRightEyeView})
	eyes.AddChild(&mp4.HeroBox{HeroEye: 1})
	cams := &mp4.CamsBox{}
	cams.AddChild(&mp4.BlinBox{Baseline: 63500})
	eyes.AddChild(cams)
	boxDiffAfterEncodeAndDecode(t, eyes)
}

func TestProjContainer(t *testing.T) {
	proj := &mp4.ProjBox{}
	proj.AddChild(&mp4.PrjiBox{ProjectionType: "rect"})
	boxDiffAfterEncodeAndDecode(t, proj)
}

// TestVisualSampleEntryWithVexu verifies that vexu and the sibling hfov box
// decode as children of a visual sample entry and are reachable via pointers,
// with the full nested hierarchy preserved.
func TestVisualSampleEntryWithVexu(t *testing.T) {
	hvc1 := mp4.CreateVisualSampleEntryBox("hvc1", 1920, 1080, nil)
	hvc1.AddChild(mp4.CreateVexuBox(mp4.StriHasLeftEyeView|mp4.StriHasRightEyeView, 1, 63500, "rect"))
	hvc1.AddChild(&mp4.HfovBox{FieldOfView: 104000})

	boxDiffAfterEncodeAndDecode(t, hvc1)

	decoded := boxAfterEncodeAndDecode(t, hvc1).(*mp4.VisualSampleEntryBox)
	if decoded.Vexu == nil {
		t.Fatal("expected decoded vexu child via Vexu pointer")
	}
	if decoded.Hfov == nil || decoded.Hfov.FieldOfView != 104000 {
		t.Fatal("expected decoded hfov child with FieldOfView 104000")
	}
	if decoded.Vexu.Eyes == nil || decoded.Vexu.Eyes.Stri == nil {
		t.Fatal("expected vexu -> eyes -> stri")
	}
	if !decoded.Vexu.Eyes.Stri.HasLeftEye() || !decoded.Vexu.Eyes.Stri.HasRightEye() {
		t.Error("expected stri left+right eye flags")
	}
	if decoded.Vexu.Eyes.Cams == nil || decoded.Vexu.Eyes.Cams.Blin == nil ||
		decoded.Vexu.Eyes.Cams.Blin.Baseline != 63500 {
		t.Error("expected vexu -> eyes -> cams -> blin baseline 63500")
	}
	if decoded.Vexu.Proj == nil || decoded.Vexu.Proj.Prji == nil ||
		decoded.Vexu.Proj.Prji.ProjectionType != "rect" {
		t.Error("expected vexu -> proj -> prji rect")
	}
}
