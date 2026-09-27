package mp4_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

// newBrandTestInit returns an init segment with one track per media type.
func newBrandTestInit(t *testing.T, lang string, mediaTypes ...string) *mp4.InitSegment {
	t.Helper()
	init := mp4.CreateEmptyInit()
	for _, mt := range mediaTypes {
		init.AddEmptyTrack(1000, mt, lang)
	}
	return init
}

func TestGenerateFtyp(t *testing.T) {
	noFragments := &mp4.FragmentFeatures{}
	cases := []struct {
		desc       string
		init       func(t *testing.T) *mp4.InitSegment
		opts       mp4.FtypOptions
		wantBrands []string // major brand first, as in compatible_brands
		wantErr    string
	}{
		{
			desc:       "video with mp4ff fragments",
			init:       func(t *testing.T) *mp4.InitSegment { return newBrandTestInit(t, "und", "video") },
			wantBrands: []string{"mp42", "iso6"},
		},
		{
			desc:       "CMAF video",
			init:       func(t *testing.T) *mp4.InitSegment { return newBrandTestInit(t, "und", "video") },
			opts:       mp4.FtypOptions{CMAF: mp4.BrandCmfc},
			wantBrands: []string{"cmfc", "iso6"},
		},
		{
			desc:       "cmf2 is accompanied by cmfc",
			init:       func(t *testing.T) *mp4.InitSegment { return newBrandTestInit(t, "und", "audio") },
			opts:       mp4.FtypOptions{CMAF: mp4.BrandCmf2},
			wantBrands: []string{"cmf2", "cmfc", "iso6"},
		},
		{
			desc:       "sthd needs iso8",
			init:       func(t *testing.T) *mp4.InitSegment { return newBrandTestInit(t, "und", "subtitle") },
			opts:       mp4.FtypOptions{CMAF: mp4.BrandCmfc, Extra: []string{mp4.BrandIm1t}},
			wantBrands: []string{"cmfc", "iso8", "im1t"},
		},
		{
			desc:       "elng needs iso9",
			init:       func(t *testing.T) *mp4.InitSegment { return newBrandTestInit(t, "en-US", "audio") },
			wantBrands: []string{"mp42", "iso9"},
		},
		{
			desc:       "no fragment features",
			init:       func(t *testing.T) *mp4.InitSegment { return newBrandTestInit(t, "und", "video") },
			opts:       mp4.FtypOptions{Fragments: noFragments},
			wantBrands: []string{"mp42", "isom"},
		},
		{
			desc: "default-base-is-moof alone needs iso5",
			init: func(t *testing.T) *mp4.InitSegment { return newBrandTestInit(t, "und", "video") },
			opts: mp4.FtypOptions{Fragments: &mp4.FragmentFeatures{
				DefaultBaseIsMoof: true,
			}},
			wantBrands: []string{"mp42", "iso5"},
		},
		{
			desc: "sdtp gives iso2 rather than avc1",
			init: func(t *testing.T) *mp4.InitSegment {
				init := newBrandTestInit(t, "und", "video")
				init.Moov.Trak.Mdia.Minf.Stbl.AddChild(&mp4.SdtpBox{})
				return init
			},
			opts:       mp4.FtypOptions{Fragments: noFragments},
			wantBrands: []string{"mp42", "iso2"},
		},
		{
			desc: "ctts version 1 needs iso4",
			init: func(t *testing.T) *mp4.InitSegment {
				init := newBrandTestInit(t, "und", "video")
				init.Moov.Trak.Mdia.Minf.Stbl.AddChild(&mp4.CttsBox{Version: 1})
				return init
			},
			opts:       mp4.FtypOptions{Fragments: noFragments},
			wantBrands: []string{"mp42", "iso4"},
		},
		{
			desc: "codec brands",
			init: func(t *testing.T) *mp4.InitSegment {
				init := newBrandTestInit(t, "und", "video", "audio")
				init.Moov.Traks[0].Mdia.Minf.Stbl.Stsd.AddChild(
					mp4.CreateVisualSampleEntryBox("av01", 1280, 720, &mp4.Av1CBox{}))
				init.Moov.Traks[1].Mdia.Minf.Stbl.Stsd.AddChild(
					mp4.CreateAudioSampleEntryBox("iamf", 0, 16, 0, nil))
				return init
			},
			opts:       mp4.FtypOptions{Extra: []string{mp4.BrandIso6, mp4.BrandDash}},
			wantBrands: []string{"mp42", "iso6", "av01", "iamf", "dash"},
		},
		{
			desc: "Dolby Vision configuration box",
			init: func(t *testing.T) *mp4.InitSegment {
				init := newBrandTestInit(t, "und", "video")
				init.Moov.Trak.Mdia.Minf.Stbl.Stsd.AddChild(
					mp4.CreateVisualSampleEntryBox("hvc1", 1920, 1080, &mp4.DoViConfigurationBox{DVProfile: 8}))
				return init
			},
			wantBrands: []string{"mp42", "iso6", "dby1"},
		},
		{
			desc:    "CMAF header with two tracks",
			init:    func(t *testing.T) *mp4.InitSegment { return newBrandTestInit(t, "und", "video", "audio") },
			opts:    mp4.FtypOptions{CMAF: mp4.BrandCmfc},
			wantErr: "one track",
		},
		{
			desc:    "CMAF without tfdt",
			init:    func(t *testing.T) *mp4.InitSegment { return newBrandTestInit(t, "und", "video") },
			opts:    mp4.FtypOptions{CMAF: mp4.BrandCmfc, Fragments: noFragments},
			wantErr: "tfdt",
		},
		{
			desc: "CMAF with an edit list rate beyond iso9",
			init: func(t *testing.T) *mp4.InitSegment {
				init := newBrandTestInit(t, "und", "video")
				elst := &mp4.ElstBox{Entries: []mp4.ElstEntry{{MediaRateInteger: 2}}}
				edts := &mp4.EdtsBox{}
				edts.AddChild(elst)
				init.Moov.Trak.AddChild(edts)
				return init
			},
			opts:    mp4.FtypOptions{CMAF: mp4.BrandCmfc},
			wantErr: "isob",
		},
		{
			desc:    "unknown CMAF brand",
			init:    func(t *testing.T) *mp4.InitSegment { return newBrandTestInit(t, "und", "video") },
			opts:    mp4.FtypOptions{CMAF: "cmfx"},
			wantErr: "unknown CMAF",
		},
		{
			desc:    "extra brand below the needed one",
			init:    func(t *testing.T) *mp4.InitSegment { return newBrandTestInit(t, "und", "video") },
			opts:    mp4.FtypOptions{Extra: []string{mp4.BrandIsom}},
			wantErr: "too low",
		},
		{
			desc:       "extra brand above the needed one",
			init:       func(t *testing.T) *mp4.InitSegment { return newBrandTestInit(t, "und", "video") },
			opts:       mp4.FtypOptions{Extra: []string{mp4.BrandIso9}},
			wantBrands: []string{"mp42", "iso6", "iso9"},
		},
		{
			desc:    "extra brand of wrong length",
			init:    func(t *testing.T) *mp4.InitSegment { return newBrandTestInit(t, "und", "video") },
			opts:    mp4.FtypOptions{Extra: []string{"abc"}},
			wantErr: "four characters",
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			ftyp, err := c.init(t).GenerateFtyp(c.opts)
			if c.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), c.wantErr) {
					t.Fatalf("got error %v, want one containing %q", err, c.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := ftyp.MajorBrand(); got != c.wantBrands[0] {
				t.Errorf("major brand %s, want %s", got, c.wantBrands[0])
			}
			if ftyp.MinorVersion() != 0 {
				t.Errorf("minor version %d, want 0", ftyp.MinorVersion())
			}
			if got := ftyp.CompatibleBrands(); !slices.Equal(got, c.wantBrands) {
				t.Errorf("compatible brands %v, want %v", got, c.wantBrands)
			}
		})
	}
}

func TestSetFtyp(t *testing.T) {
	init := newBrandTestInit(t, "und", "video")
	ftyp := mp4.NewFtyp(mp4.BrandMp42, 0, []string{mp4.BrandMp42, mp4.BrandIso6})
	init.SetFtyp(ftyp)
	if init.Ftyp != ftyp || init.Children[0] != mp4.Box(ftyp) || len(init.Children) != 2 {
		t.Errorf("ftyp not replaced: %d children", len(init.Children))
	}

	noFtyp := mp4.NewMP4Init()
	noFtyp.AddChild(init.Moov)
	noFtyp.SetFtyp(ftyp)
	if noFtyp.Ftyp != ftyp || noFtyp.Children[0] != mp4.Box(ftyp) || len(noFtyp.Children) != 2 {
		t.Errorf("ftyp not inserted first: %d children", len(noFtyp.Children))
	}
}

// newBrandTestSegment returns a media segment with one fragment made by
// CreateFragment and two samples, the first with firstFlags.
func newBrandTestSegment(t *testing.T, firstFlags uint32) *mp4.MediaSegment {
	t.Helper()
	seg := mp4.NewMediaSegment()
	frag, err := mp4.CreateFragment(1, 1)
	if err != nil {
		t.Fatal(err)
	}
	seg.AddFragment(frag)
	frag.AddFullSample(mp4.FullSample{
		Sample: mp4.Sample{Flags: firstFlags, Dur: 1000, Size: 4}, Data: []byte{0, 1, 2, 3},
	})
	frag.AddFullSample(mp4.FullSample{
		Sample:     mp4.Sample{Flags: mp4.NonSyncSampleFlags, Dur: 1000, Size: 4},
		DecodeTime: 1000, Data: []byte{4, 5, 6, 7},
	})
	return seg
}

func TestGenerateStyp(t *testing.T) {
	cases := []struct {
		desc       string
		seg        func(t *testing.T) *mp4.MediaSegment
		opts       mp4.StypOptions
		wantBrands []string // nil means no styp
	}{
		{
			desc:       "CMAF segment starting with a sync sample",
			seg:        func(t *testing.T) *mp4.MediaSegment { return newBrandTestSegment(t, mp4.SyncSampleFlags) },
			opts:       mp4.StypOptions{CMAF: true},
			wantBrands: []string{"cmfs", "cmff", "cmfl", "cmfr", "msdh"},
		},
		{
			desc:       "CMAF segment starting with a non-sync sample",
			seg:        func(t *testing.T) *mp4.MediaSegment { return newBrandTestSegment(t, mp4.NonSyncSampleFlags) },
			opts:       mp4.StypOptions{CMAF: true},
			wantBrands: []string{"cmfs", "cmff", "cmfl", "msdh"},
		},
		{
			desc:       "last DASH segment",
			seg:        func(t *testing.T) *mp4.MediaSegment { return newBrandTestSegment(t, mp4.SyncSampleFlags) },
			opts:       mp4.StypOptions{Last: true},
			wantBrands: []string{"msdh", "lmsg"},
		},
		{
			desc: "sidx covering the segment",
			seg: func(t *testing.T) *mp4.MediaSegment {
				seg := newBrandTestSegment(t, mp4.SyncSampleFlags)
				seg.AddSidx(&mp4.SidxBox{ReferenceID: 1, Timescale: 1000, SidxRefs: []mp4.SidxRef{
					{ReferencedSize: uint32(seg.Fragments[0].Size()), SubSegmentDuration: 2000},
				}})
				return seg
			},
			wantBrands: []string{"msdh", "msix"},
		},
		{
			desc: "sidx not covering the segment",
			seg: func(t *testing.T) *mp4.MediaSegment {
				seg := newBrandTestSegment(t, mp4.SyncSampleFlags)
				seg.AddSidx(&mp4.SidxBox{ReferenceID: 1, Timescale: 1000, SidxRefs: []mp4.SidxRef{
					{ReferencedSize: 8, SubSegmentDuration: 2000},
				}})
				return seg
			},
			wantBrands: nil,
		},
		{
			desc: "no tfdt",
			seg: func(t *testing.T) *mp4.MediaSegment {
				seg := newBrandTestSegment(t, mp4.SyncSampleFlags)
				seg.Fragments[0].Moof.Traf.Tfdt = nil
				return seg
			},
			wantBrands: nil,
		},
		{
			desc: "absolute addressing",
			seg: func(t *testing.T) *mp4.MediaSegment {
				seg := newBrandTestSegment(t, mp4.SyncSampleFlags)
				seg.Fragments[0].Moof.Traf.Tfhd.Flags &^= mp4.TfhdDefaultBaseIsMoofFlag
				return seg
			},
			opts:       mp4.StypOptions{CMAF: true},
			wantBrands: []string{"cmfs", "cmff", "cmfl", "cmfr"},
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			styp := c.seg(t).GenerateStyp(c.opts)
			if c.wantBrands == nil {
				if styp != nil {
					t.Fatalf("got styp with %v, want none", styp.CompatibleBrands())
				}
				return
			}
			if styp == nil {
				t.Fatalf("got no styp, want %v", c.wantBrands)
			}
			if got := styp.MajorBrand(); got != c.wantBrands[0] {
				t.Errorf("major brand %s, want %s", got, c.wantBrands[0])
			}
			if got := styp.CompatibleBrands(); !slices.Equal(got, c.wantBrands) {
				t.Errorf("compatible brands %v, want %v", got, c.wantBrands)
			}
		})
	}
}

// TestDefaultBrandsMatchGenerated checks that CreateFtyp and CreateStyp,
// which cannot see the content, give the brands that GenerateFtyp and
// GenerateStyp give for a single CMAF track written by mp4ff.
func TestDefaultBrandsMatchGenerated(t *testing.T) {
	init := newBrandTestInit(t, "und", "video")
	ftyp, err := init.GenerateFtyp(mp4.FtypOptions{CMAF: mp4.BrandCmfc})
	if err != nil {
		t.Fatal(err)
	}
	def := mp4.CreateFtyp()
	if def.MajorBrand() != ftyp.MajorBrand() || !slices.Equal(def.CompatibleBrands(), ftyp.CompatibleBrands()) {
		t.Errorf("CreateFtyp gives %s %v, GenerateFtyp %s %v", def.MajorBrand(), def.CompatibleBrands(),
			ftyp.MajorBrand(), ftyp.CompatibleBrands())
	}
	seg := newBrandTestSegment(t, mp4.NonSyncSampleFlags)
	styp := seg.GenerateStyp(mp4.StypOptions{CMAF: true})
	defStyp := mp4.CreateStyp()
	if defStyp.MajorBrand() != styp.MajorBrand() || !slices.Equal(defStyp.CompatibleBrands(), styp.CompatibleBrands()) {
		t.Errorf("CreateStyp gives %s %v, GenerateStyp %s %v", defStyp.MajorBrand(), defStyp.CompatibleBrands(),
			styp.MajorBrand(), styp.CompatibleBrands())
	}
}
