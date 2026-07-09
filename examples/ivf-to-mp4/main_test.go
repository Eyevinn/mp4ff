package main

import (
	"path/filepath"
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

// TestIVFToMP4 muxes the AV1 and VP9 test IVF files and checks the resulting fragmented MP4: the
// expected sample entry with a config box, the expected media timescale, one fragment per GOP,
// and a sync sample at the start of every fragment. testdata clips have keyframes at frames
// 0, 10, 20 -> 3 GOPs, 25 samples total.
func TestIVFToMP4(t *testing.T) {
	cases := []struct {
		name        string
		file        string
		sampleEntry string
	}{
		{"av1", "testdata/av1.ivf", "av01"},
		{"vp9", "testdata/vp9.ivf", "vp09"},
		{"vp8", "testdata/vp8.ivf", "vp08"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out := filepath.Join(t.TempDir(), "out.mp4")
			if err := run(c.file, out); err != nil {
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
			assertConfigBox(t, stsd, c.sampleEntry)
			if ts := mf.Init.Moov.Trak.Mdia.Mdhd.Timescale; ts != 25 {
				t.Errorf("media timescale = %d, want 25", ts)
			}

			var frags []*mp4.Fragment
			for _, seg := range mf.Segments {
				frags = append(frags, seg.Fragments...)
			}
			if len(frags) != 3 {
				t.Fatalf("got %d fragments, want 3", len(frags))
			}
			trex := mf.Init.Moov.Mvex.Trex
			total := 0
			for i, frag := range frags {
				fss, err := frag.GetFullSamples(trex)
				if err != nil {
					t.Fatal(err)
				}
				if len(fss) == 0 || !fss[0].IsSync() {
					t.Errorf("fragment %d must start with a sync sample", i)
				}
				total += len(fss)
			}
			if total != 25 {
				t.Errorf("total samples = %d, want 25", total)
			}
		})
	}
}

func assertConfigBox(t *testing.T, stsd *mp4.StsdBox, sampleEntry string) {
	t.Helper()
	switch sampleEntry {
	case "av01":
		if stsd.Av01 == nil || stsd.Av01.Av1C == nil {
			t.Fatal("no av01/av1C")
		}
		if _, err := stsd.Av01.Av1C.SequenceHeader(); err != nil {
			t.Fatalf("av1C sequence header does not parse: %v", err)
		}
	case "vp08", "vp09":
		if stsd.VpXX == nil || stsd.VpXX.VppC == nil {
			t.Fatalf("no %s/vpcC", sampleEntry)
		}
		if stsd.VpXX.Type() != sampleEntry {
			t.Errorf("sample entry type = %s, want %s", stsd.VpXX.Type(), sampleEntry)
		}
	default:
		t.Fatalf("unhandled sample entry %q", sampleEntry)
	}
}
