package mp4_test

import (
	"fmt"
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

func TestClli(t *testing.T) {
	clli := mp4.CreateClliBox(
		1000, // max_content_light_level
		500,  // max_pic_average_light_level
	)

	boxDiffAfterEncodeAndDecode(t, clli)

	data := encodeBox(t, clli)
	changeBoxSizeAndAssertError(t, data, 0, uint32(clli.Size()-1), fmt.Sprintf("decode clli pos 0: invalid box size %d", clli.Size()-1))
}
