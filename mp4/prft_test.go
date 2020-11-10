package mp4

import "testing"

func TestPrft(t *testing.T) {
	prfts := []*PrftBox{
		CreatePrftBox(0, 8998, 98),
		CreatePrftBox(1, 8998, 98),
	}
	for _, prft := range prfts {
		boxDiffAfterEncodeAndDecode(t, prft)
	}

}
