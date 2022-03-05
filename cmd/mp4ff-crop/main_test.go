package main

import (
	"bytes"
	"log"
	"os"
	"testing"

	"github.com/edgeware/mp4ff/mp4"
)

// TestCroppedFileDuration - simple test to check that cropped file has right duration
// In general, this duration will not be exactly the same as the one asked for.
func TestCroppedFileDuration(t *testing.T) {
	testFile := "../../mp4/testdata/prog_8s.mp4"
	cropDur := 2000

	ifh, err := os.Open(testFile)
	if err != nil {
		t.Error(err)
	}
	defer ifh.Close()
	parsedMp4, err := mp4.DecodeFile(ifh, mp4.WithDecodeMode(mp4.DecModeLazyMdat))
	if err != nil {
		t.Error(err)
	}

	buf := bytes.Buffer{}

	err = cropMP4(parsedMp4, cropDur, &buf, ifh)
	if err != nil {
		log.Fatal(err)
	}

	decCropped, err := mp4.DecodeFile(&buf)
	if err != nil {
		t.Error(err)
	}
	moovDur := decCropped.Moov.Mvhd.Duration
	moovTimescale := decCropped.Moov.Mvhd.Timescale
	if uint64(cropDur)*uint64(moovTimescale) != moovDur*1000 {
		t.Errorf("got %d/%dms instead of %dms", moovDur, moovTimescale, cropDur)
	}
}
