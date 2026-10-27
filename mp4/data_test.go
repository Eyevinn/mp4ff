package mp4_test

import (
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

func TestEncodeData(t *testing.T) {
	data := []byte("dummy")
	db := &mp4.DataBox{Data: data}
	boxDiffAfterEncodeAndDecode(t, db)
}
