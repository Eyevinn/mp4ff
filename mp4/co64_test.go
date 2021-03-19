package mp4

import (
	"testing"
)

func TestEncDecCo64(t *testing.T) {

	b := &Co64Box{
		Version:     0,
		Flags:       2, // Just in test
		ChunkOffset: []uint64{1234, 8908080},
	}
	boxDiffAfterEncodeAndDecode(t, b)
}
