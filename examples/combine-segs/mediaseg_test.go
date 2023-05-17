package main

import (
	"testing"
)

// TestCombineMediaSegments combines two media segments into a common one with two track IDs.
func TestCombineMediaSegments(t *testing.T) {
	files := []string{
		"./testdata/V300/1.m4s",
		"./testdata/A48/1.m4s",
	}
	trackIDs := []uint32{1, 2}
	combinedMediaSeg, err := combineMediaSegments(files, trackIDs)
	if err != nil {
		t.Error(err)
	}
	nrTracks := len(combinedMediaSeg.Fragments[0].Moof.Trafs)
	if nrTracks != 2 {
		t.Errorf("expected 2 tracks, got %d", nrTracks)
	}
}
