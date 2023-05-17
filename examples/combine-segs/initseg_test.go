package main

import (
	"testing"
)

// TestCombineInitSegments combines two init segments into a common one with two track IDs.
func TestCombineInitSegments(t *testing.T) {
	files := []string{
		"./testdata/V300/init.mp4",
		"./testdata/A48/init.mp4",
	}
	trackIDs := []uint32{1, 2}

	combinedInit, err := combineInitSegments(files, trackIDs)
	if err != nil {
		t.Error(err)
	}
	nrTracks := len(combinedInit.Moov.Traks)
	if nrTracks != 2 {
		t.Errorf("expected 2 tracks, got %d", nrTracks)
	}
}
