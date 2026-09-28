package mp4_test

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

// encodeAndDecodeFile writes init and segs one after the other, and decodes
// the result as one file.
func encodeAndDecodeFile(t *testing.T, init *mp4.InitSegment, segs ...*mp4.MediaSegment) *mp4.File {
	t.Helper()
	var buf bytes.Buffer
	if err := init.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	for _, seg := range segs {
		if err := seg.Encode(&buf); err != nil {
			t.Fatal(err)
		}
	}
	f, err := mp4.DecodeFile(&buf)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// newProgressiveBrandTestFile returns a file with the ftyp and moov of init.
func newProgressiveBrandTestFile(init *mp4.InitSegment) *mp4.File {
	f := mp4.NewFile()
	f.AddChild(init.Ftyp, 0)
	f.AddChild(init.Moov, 0)
	return f
}

// TestCheckBrandsOfGeneratedBrands checks that the brands mp4ff writes by
// default, and those it generates, raise no issues.
func TestCheckBrandsOfGeneratedBrands(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		init := newBrandTestInit(t, "und", "video")
		f := encodeAndDecodeFile(t, init,
			newBrandTestSegment(t, mp4.SyncSampleFlags), newBrandTestSegment(t, mp4.SyncSampleFlags))
		if issues := f.CheckBrands(); issues != nil {
			t.Errorf("issues: %v", issues)
		}
	})
	t.Run("generated for two tracks and a last segment", func(t *testing.T) {
		init := newBrandTestInit(t, "en-US", "video", "subtitle")
		ftyp, err := init.GenerateFtyp(mp4.FtypOptions{})
		if err != nil {
			t.Fatal(err)
		}
		init.SetFtyp(ftyp)
		seg1 := newBrandTestSegment(t, mp4.SyncSampleFlags)
		seg1.Styp = seg1.GenerateStyp(mp4.StypOptions{})
		seg2 := newBrandTestSegment(t, mp4.SyncSampleFlags)
		seg2.Styp = seg2.GenerateStyp(mp4.StypOptions{Last: true})
		f := encodeAndDecodeFile(t, init, seg1, seg2)
		if issues := f.CheckBrands(); issues != nil {
			t.Errorf("issues: %v", issues)
		}
	})
	t.Run("golden init segment", func(t *testing.T) {
		f, err := mp4.ReadMP4File("testdata/golden_init_video.mp4")
		if err != nil {
			t.Fatal(err)
		}
		if issues := f.CheckBrands(); issues != nil {
			t.Errorf("issues: %v", issues)
		}
	})
}

func TestCheckBrandsIssues(t *testing.T) {
	cases := []struct {
		desc string
		file func(t *testing.T) *mp4.File
		want []string // one substring per expected issue, in order
	}{
		{
			desc: "old default ftyp and styp",
			file: func(t *testing.T) *mp4.File {
				init := newBrandTestInit(t, "und", "video")
				init.SetFtyp(mp4.NewFtyp(mp4.BrandCmfc, 0, []string{mp4.BrandDash, mp4.BrandIso6}))
				seg := newBrandTestSegment(t, mp4.SyncSampleFlags)
				seg.Styp = mp4.NewStyp(mp4.BrandCmfs, 0, []string{mp4.BrandDash, mp4.BrandMsdh})
				return encodeAndDecodeFile(t, init, seg)
			},
			want: []string{
				"warning: ftyp: major brand cmfc is not repeated",
				"warning: ftyp: dash declares an Indexed Self-Initializing Media Segment",
				"warning: styp of segment 1: major brand cmfs is not repeated",
			},
		},
		{
			desc: "isom with fragments",
			file: func(t *testing.T) *mp4.File {
				init := newBrandTestInit(t, "und", "video")
				init.SetFtyp(mp4.NewFtyp(mp4.BrandIsom, 0, []string{mp4.BrandIsom, mp4.BrandIso6}))
				return encodeAndDecodeFile(t, init, newBrandTestSegment(t, mp4.SyncSampleFlags))
			},
			want: []string{
				"warning: ftyp: major brand isom is an ISO/IEC 14496-12 Annex E brand",
				"error: ftyp: isom is claimed, but isom readers would misinterpret default-base-is-moof (iso5), trun version 1 (iso6)",
				"warning: ftyp: isom is claimed, but isom readers need not support styp (iso6), tfdt (iso6)",
			},
		},
		{
			desc: "AV1 without av01 and Dolby Vision without dby1",
			file: func(t *testing.T) *mp4.File {
				init := newBrandTestInit(t, "und", "video", "video")
				init.Moov.Traks[0].Mdia.Minf.Stbl.Stsd.AddChild(
					mp4.CreateVisualSampleEntryBox("av01", 1280, 720, &mp4.Av1CBox{}))
				init.Moov.Traks[1].Mdia.Minf.Stbl.Stsd.AddChild(
					mp4.CreateVisualSampleEntryBox("dvh1", 1280, 720, nil))
				init.SetFtyp(mp4.NewFtyp(mp4.BrandMp42, 0, []string{mp4.BrandMp42, mp4.BrandIso6}))
				return newProgressiveBrandTestFile(init)
			},
			want: []string{
				"error: ftyp: AV1 content needs the av01 brand",
				"warning: ftyp: Dolby Vision content should have the dby1 brand",
			},
		},
		{
			desc: "CMAF header with two tracks and a minor version",
			file: func(t *testing.T) *mp4.File {
				init := newBrandTestInit(t, "und", "video", "audio")
				init.SetFtyp(mp4.NewFtyp(mp4.BrandCmfc, 512, []string{mp4.BrandCmfc, mp4.BrandIso6}))
				return newProgressiveBrandTestFile(init)
			},
			want: []string{
				"error: ftyp: minor version is 512, but shall be 0",
				"error: ftyp: a CMAF header has exactly one track, not 2",
			},
		},
		{
			desc: "CMAF fragment without tfdt",
			file: func(t *testing.T) *mp4.File {
				init := newBrandTestInit(t, "und", "video")
				seg := newBrandTestSegment(t, mp4.SyncSampleFlags)
				seg.Styp = mp4.NewStyp(mp4.BrandCmfs, 0, []string{mp4.BrandCmfs, mp4.BrandMsdh})
				removeTfdt(seg)
				return encodeAndDecodeFile(t, init, seg)
			},
			want: []string{
				"error: ftyp: CMAF needs tfdt and default-base-is-moof in every traf",
				"error: styp of segment 1: msdh is claimed, but a traf lacks tfdt",
			},
		},
		{
			desc: "msix without sidx, lmsg too early, cmfr without sync sample",
			file: func(t *testing.T) *mp4.File {
				init := newBrandTestInit(t, "und", "video")
				seg1 := newBrandTestSegment(t, mp4.NonSyncSampleFlags)
				seg1.Styp = mp4.NewStyp(mp4.BrandMsdh, 0,
					[]string{mp4.BrandMsdh, mp4.BrandMsix, mp4.BrandLmsg, mp4.BrandCmfr})
				seg2 := newBrandTestSegment(t, mp4.SyncSampleFlags)
				return encodeAndDecodeFile(t, init, seg1, seg2)
			},
			want: []string{
				"error: styp of segment 1: msix is claimed, but there is no sidx",
				"error: styp of segment 1: lmsg is claimed, but this is not the last segment",
				"error: styp of segment 1: cmfr is claimed, but the first sample is not a sync sample",
			},
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			issues := c.file(t).CheckBrands()
			got := make([]string, len(issues))
			for i, issue := range issues {
				got[i] = issue.String()
			}
			if len(got) != len(c.want) {
				t.Fatalf("got %d issues, want %d:\n%s", len(got), len(c.want), strings.Join(got, "\n"))
			}
			for i, w := range c.want {
				if !strings.Contains(got[i], w) {
					t.Errorf("issue %d is %q, want it to contain %q", i, got[i], w)
				}
			}
		})
	}
}

// removeTfdt removes the tfdt boxes from all trafs of seg.
func removeTfdt(seg *mp4.MediaSegment) {
	for _, frag := range seg.Fragments {
		for _, traf := range frag.Moof.Trafs {
			traf.Tfdt = nil
			traf.Children = slices.DeleteFunc(traf.Children, func(b mp4.Box) bool { return b.Type() == "tfdt" })
		}
	}
}
