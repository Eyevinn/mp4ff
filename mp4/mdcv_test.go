package mp4_test

import (
	"fmt"
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

func TestMdcv(t *testing.T) {
	mdcv := mp4.CreateMdcvBox(
		[3]uint16{1000, 3000, 5000},
		[3]uint16{2000, 4000, 6000},
		7000, 8000,
		10000, 100,
	)

	boxDiffAfterEncodeAndDecode(t, mdcv)

	data := encodeBox(t, mdcv)
	changeBoxSizeAndAssertError(t, data, 0, uint32(mdcv.Size()-1), fmt.Sprintf("decode mdcv pos 0: invalid box size %d", mdcv.Size()-1))
}
