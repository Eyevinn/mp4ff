package mp4_test

import (
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

func TestIspe(t *testing.T) {
	boxDiffAfterEncodeAndDecode(t, &mp4.IspeBox{ImageWidth: 600, ImageHeight: 338})
}
