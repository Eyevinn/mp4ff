package avc

import (
	"encoding/hex"
	"testing"
)

const videoNaluStart = "25888040ffde08e47a7bff05ab"

func TestSliceTypeParser(t *testing.T) {
	byteData, _ := hex.DecodeString(videoNaluStart)
	want := SLICE_I
	got, err := GetSliceTypeFromNAL(byteData)
	if err != nil {
		t.Error(err)
	}
	if got != want {
		t.Errorf("got %s want %s", got, want)
	}
}
