package mp4

import (
	"testing"
)

func TestEncDecClap(t *testing.T) {

	b := &ClapBox{
		CleanApertureWidthN: 1, CleanApertureWidthD: 2,
		CleanApertureHeightN: 3, CleanApertureHeightD: 4,
		HorizOffN: 5, HorizOffD: 6,
		VertOffN: 7, VertOffD: 8,
	}
	boxDiffAfterEncodeAndDecode(t, b)
}
