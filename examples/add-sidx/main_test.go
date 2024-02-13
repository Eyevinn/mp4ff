package main

import (
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

func TestAddSidx(t *testing.T) {
	sidxOut := "testV300_sidx.mp4"
	inPath := "../resegmenter/testdata/testV300.mp4"
	err := run(inPath, sidxOut, false)
	if err != nil {
		t.Error(err)
	}
	reRead, err := mp4.ReadMP4File(sidxOut)
	if err != nil {
		t.Error(err)
	}
	if reRead.Sidx == nil {
		t.Error("No sidx box added")
	}
}
