package mp4

import (
	"testing"
)

func TestFtyp(t *testing.T) {
	ftyp := CreateFtyp()
	ftyp.AddCompatibleBrand("dash")
	boxDiffAfterEncodeAndDecode(t, ftyp)
}

func TestStyp(t *testing.T) {
	styp := CreateStyp()
	styp.AddCompatibleBrands([]string{"cmfc", "cmfs", "lmsg"})
	boxDiffAfterEncodeAndDecode(t, styp)
}
