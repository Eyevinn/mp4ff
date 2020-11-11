package mp4

import (
	"testing"
)

func TestDecodeElng(t *testing.T) {

	elng := &ElngBox{Language: "en-US"}
	boxDiffAfterEncodeAndDecode(t, elng)
}
