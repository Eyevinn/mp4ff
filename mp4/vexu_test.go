package mp4_test

import (
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

func TestVexuRoundTrip(t *testing.T) {
	vexu := mp4.CreateVexuBox(0x03, 1, 63500, "rect")
	boxDiffAfterEncodeAndDecode(t, vexu)
}

func TestVexuReversedEyes(t *testing.T) {
	vexu := mp4.CreateVexuBox(0x0B, 2, 70000, "rect")
	boxDiffAfterEncodeAndDecode(t, vexu)
}

func TestStri(t *testing.T) {
	stri := &mp4.StriBox{StereoFlags: 0x03}
	if !stri.HasLeftEye() {
		t.Error("expected HasLeftEye")
	}
	if !stri.HasRightEye() {
		t.Error("expected HasRightEye")
	}
	if stri.EyeViewsReversed() {
		t.Error("expected not reversed")
	}
	boxDiffAfterEncodeAndDecode(t, stri)

	stri2 := &mp4.StriBox{StereoFlags: 0x0B}
	if !stri2.EyeViewsReversed() {
		t.Error("expected reversed")
	}
	boxDiffAfterEncodeAndDecode(t, stri2)
}

func TestHero(t *testing.T) {
	hero := &mp4.HeroBox{HeroEye: 1}
	if hero.HeroEyeName() != "left" {
		t.Errorf("HeroEyeName() = %q, want left", hero.HeroEyeName())
	}
	boxDiffAfterEncodeAndDecode(t, hero)
}

func TestBlin(t *testing.T) {
	blin := &mp4.BlinBox{Baseline: 63500}
	boxDiffAfterEncodeAndDecode(t, blin)
}

func TestPrji(t *testing.T) {
	prji := &mp4.PrjiBox{ProjectionType: "rect"}
	boxDiffAfterEncodeAndDecode(t, prji)
}

func TestHfov(t *testing.T) {
	hfov := &mp4.HfovBox{FieldOfView: 63500}
	boxDiffAfterEncodeAndDecode(t, hfov)
}

func TestEyesContainer(t *testing.T) {
	eyes := &mp4.EyesBox{}
	eyes.AddChild(&mp4.StriBox{StereoFlags: 0x03})
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
