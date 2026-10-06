package mp4_test

import (
	"os"
	"strings"
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

func TestTrakSampleFunctions(t *testing.T) {
	testFile := "testdata/bbb_prog_10s.mp4"
	f, err := os.Open(testFile)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	mf, err := mp4.DecodeFile(f)
	if err != nil {
		t.Fatal(err)
	}
	moov := mf.Moov
	traks := moov.Traks
	if len(traks) != 2 {
		t.Fatalf("expected 2 tracks, got %d", len(traks))
	}
	trak := traks[0]
	if trak.Tkhd.TrackID != 1 {
		t.Fatalf("expected trackID 1, got %d", trak.Tkhd.TrackID)
	}
	first2Samples, err := trak.GetSampleData(1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(first2Samples) != 2 {
		t.Fatalf("expected 2 samples, got %d", len(first2Samples))
	}
	// An interval not starting at sample 1 must yield the same samples as
	// the corresponding tail of a from-the-start interval (this used to
	// panic on samples[nr-1] indexing past the result slice).
	first4Samples, err := trak.GetSampleData(1, 4)
	if err != nil {
		t.Fatal(err)
	}
	samples2to4, err := trak.GetSampleData(2, 4)
	if err != nil {
		t.Fatal(err)
	}
	if len(samples2to4) != 3 {
		t.Fatalf("expected 3 samples, got %d", len(samples2to4))
	}
	for i, s := range samples2to4 {
		if s != first4Samples[i+1] {
			t.Errorf("sample %d differs: got %+v, want %+v", i+2, s, first4Samples[i+1])
		}
	}
	// An interval ending before it starts is an error.
	if s, err := trak.GetSampleData(4, 2); err == nil {
		t.Errorf("expected an error for samples 4-2, got %d samples", len(s))
	}
	ranges, err := trak.GetRangesForSampleInterval(1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(ranges) != 1 {
		t.Fatalf("expected 1 range, got %d", len(ranges))
	}
}

// TestGetSampleDataShortTables checks that a ctts or sdtp box covering fewer
// samples than stsz declares does not panic. Nothing in the box definitions
// ties these tables to the same sample count, so a corrupt file can disagree.
func TestGetSampleDataShortTables(t *testing.T) {
	cases := []struct {
		desc    string
		corrupt func(stbl *mp4.StblBox)
	}{
		{
			desc: "ctts covering two samples",
			corrupt: func(stbl *mp4.StblBox) {
				stbl.Ctts = &mp4.CttsBox{}
				if err := stbl.Ctts.AddSampleCountsAndOffset([]uint32{2}, []int32{1000}); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			desc: "empty ctts",
			corrupt: func(stbl *mp4.StblBox) {
				stbl.Ctts = &mp4.CttsBox{EndSampleNr: []uint32{0}}
			},
		},
		{
			desc: "sdtp with two entries",
			corrupt: func(stbl *mp4.StblBox) {
				stbl.Sdtp = &mp4.SdtpBox{Entries: []mp4.SdtpEntry{
					mp4.NewSdtpEntry(0, 2, 0, 0), mp4.NewSdtpEntry(0, 2, 0, 0),
				}}
			},
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			fd, err := os.Open("testdata/bbb_prog_10s.mp4")
			if err != nil {
				t.Fatal(err)
			}
			defer fd.Close()
			f, err := mp4.DecodeFile(fd)
			if err != nil {
				t.Fatal(err)
			}
			trak := f.Moov.Trak
			c.corrupt(trak.Mdia.Minf.Stbl)
			samples, err := trak.GetSampleData(1, 10)
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			if len(samples) != 10 {
				t.Errorf("got %d samples instead of 10", len(samples))
			}
		})
	}
}

func TestCheckDataIsSelfContained(t *testing.T) {
	externalURL := &mp4.URLBox{Location: "other.mp4"}
	cases := []struct {
		desc    string
		file    string
		modify  func(trak *mp4.TrakBox)
		wantErr string
	}{
		{desc: "self-contained url", file: "testdata/prog_8s.mp4"},
		{desc: "dref file from MP4Box", file: "testdata/prog_8s_dref.mp4",
			wantErr: `track 1: sample description 1 (mp4a) has its data outside this file (data reference 1: url "prog_8s.mp4")`},
		{desc: "external url", file: "testdata/prog_8s.mp4",
			modify:  func(trak *mp4.TrakBox) { setDataEntries(trak, externalURL) },
			wantErr: `(data reference 1: url "other.mp4")`},
		{desc: "url without location", file: "testdata/prog_8s.mp4",
			modify: func(trak *mp4.TrakBox) { setDataEntries(trak, &mp4.URLBox{NoLocation: true}) }},
		{desc: "self-contained alis", file: "testdata/prog_8s.mp4",
			modify: func(trak *mp4.TrakBox) { setDataEntries(trak, mp4.CreateUnknownBox("alis", 12, []byte{0, 0, 0, 1})) }},
		{desc: "external urn", file: "testdata/prog_8s.mp4",
			modify: func(trak *mp4.TrakBox) {
				setDataEntries(trak, mp4.CreateUnknownBox("urn ", 16, []byte{0, 0, 0, 0, 'u', 'r', 'n', 0}))
			},
			wantErr: `(data reference 1: "urn " entry)`},
		{desc: "second entry external", file: "testdata/prog_8s.mp4",
			modify: func(trak *mp4.TrakBox) {
				setDataEntries(trak, mp4.CreateURLBox(), externalURL)
				trak.Mdia.Minf.Stbl.Stsd.Mp4a.DataReferenceIndex = 2
			},
			wantErr: `(data reference 2: url "other.mp4")`},
		{desc: "index outside dref", file: "testdata/prog_8s.mp4",
			modify: func(trak *mp4.TrakBox) {
				setDataEntries(trak, externalURL)
				trak.Mdia.Minf.Stbl.Stsd.Mp4a.DataReferenceIndex = 2
			}},
		{desc: "index 0", file: "testdata/prog_8s.mp4",
			modify: func(trak *mp4.TrakBox) {
				setDataEntries(trak, externalURL)
				trak.Mdia.Minf.Stbl.Stsd.Mp4a.DataReferenceIndex = 0
			}},
		{desc: "no dref", file: "testdata/prog_8s.mp4",
			modify: func(trak *mp4.TrakBox) { trak.Mdia.Minf.Dinf.Dref = nil }},
		{desc: "unknown sample entry", file: "testdata/prog_8s.mp4",
			modify: func(trak *mp4.TrakBox) {
				setDataEntries(trak, externalURL)
				trak.Mdia.Minf.Stbl.Stsd.Children[0] = mp4.CreateUnknownBox("xxxx", 16, []byte{0, 0, 0, 0, 0, 0, 0, 1})
			},
			wantErr: `sample description 1 (xxxx) has its data outside this file`},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			f, err := os.Open(c.file)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			mf, err := mp4.DecodeFile(f, mp4.WithDecodeMode(mp4.DecModeLazyMdat))
			if err != nil {
				t.Fatal(err)
			}
			trak := mf.Moov.Traks[0]
			if trak.Mdia.Minf.Stbl.Stsd.Mp4a == nil {
				t.Fatal("expected mp4a in first track")
			}
			if c.modify != nil {
				c.modify(trak)
			}
			err = trak.CheckDataIsSelfContained()
			switch {
			case c.wantErr == "" && err != nil:
				t.Errorf("unexpected error: %v", err)
			case c.wantErr != "" && err == nil:
				t.Errorf("expected error containing %q", c.wantErr)
			case c.wantErr != "" && !strings.Contains(err.Error(), c.wantErr):
				t.Errorf("got error %q, want it to contain %q", err, c.wantErr)
			}
		})
	}
}

func TestCheckDataIsSelfContainedIncompleteTrak(t *testing.T) {
	if err := (&mp4.TrakBox{}).CheckDataIsSelfContained(); err != nil {
		t.Errorf("unexpected error for trak without mdia: %v", err)
	}
}

// setDataEntries replaces the data entries of the dref box of trak.
func setDataEntries(trak *mp4.TrakBox, entries ...mp4.Box) {
	dref := &mp4.DrefBox{}
	for _, e := range entries {
		dref.AddChild(e)
	}
	trak.Mdia.Minf.Dinf.Dref = dref
}
