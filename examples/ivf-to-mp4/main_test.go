package main

import (
	"path/filepath"
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

// TestIVFToMP4 muxes the AV1 test IVF and checks the resulting fragmented MP4: an av01 sample
// entry with a parseable av1C, the expected media timescale, one fragment per GOP, and a sync
// sample at the start of every fragment.
func TestIVFToMP4(t *testing.T) {
	out := filepath.Join(t.TempDir(), "out.mp4")
	if err := run("testdata/av1.ivf", out); err != nil {
		t.Fatal(err)
	}
	mf, err := mp4.ReadMP4File(out)
	if err != nil {
		t.Fatal(err)
	}
	if mf.Init == nil {
		t.Fatal("no init segment")
	}
	stsd := mf.Init.Moov.Trak.Mdia.Minf.Stbl.Stsd
	if stsd.Av01 == nil {
		t.Fatal("no av01 sample entry")
	}
	if stsd.Av01.Av1C == nil {
		t.Fatal("no av1C configuration box")
	}
	if _, err := stsd.Av01.Av1C.SequenceHeader(); err != nil {
		t.Fatalf("av1C sequence header does not parse: %v", err)
	}
	if w, h := stsd.Av01.Width, stsd.Av01.Height; w != 320 || h != 180 {
		t.Errorf("sample entry size = %dx%d, want 320x180", w, h)
	}
	if ts := mf.Init.Moov.Trak.Mdia.Mdhd.Timescale; ts != 25 {
		t.Errorf("media timescale = %d, want 25", ts)
	}

	var frags []*mp4.Fragment
	for _, seg := range mf.Segments {
		frags = append(frags, seg.Fragments...)
	}
	// testdata/av1.ivf has keyframes at frames 0, 10, 20 -> 3 GOPs.
	if len(frags) != 3 {
		t.Fatalf("got %d fragments, want 3", len(frags))
	}
	trex := mf.Init.Moov.Mvex.Trex
	totalSamples, syncStarts := 0, 0
	for i, frag := range frags {
		fss, err := frag.GetFullSamples(trex)
		if err != nil {
			t.Fatal(err)
		}
		if len(fss) == 0 {
			t.Fatalf("fragment %d has no samples", i)
		}
		if !fss[0].IsSync() {
			t.Errorf("fragment %d does not start with a sync sample", i)
		}
		syncStarts++
		totalSamples += len(fss)
	}
	if totalSamples != 25 {
		t.Errorf("total samples = %d, want 25", totalSamples)
	}
	if syncStarts != 3 {
		t.Errorf("sync-starting fragments = %d, want 3", syncStarts)
	}
}
