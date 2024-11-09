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

	_, err := b.GetOffset(0)
	if err == nil {
		t.Error("should not be able to get offset for nr 0")
	}

	offset, err := b.GetOffset(1)
	if err != nil {
		t.Error(err)
	}
	if offset != 1234 {
		t.Errorf("offset = %d instead of 1234", offset)
	}
}
