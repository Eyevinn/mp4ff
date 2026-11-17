package main

import (
	"math"
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
	"github.com/go-test/deep"
)

func TestSttsCrop(t *testing.T) {
	stts1 := mp4.SttsBox{
		SampleCount:     []uint32{3189, 1, 22968, 3, 1, 130878},
		SampleTimeDelta: []uint32{1024, 6752, 1024, 1, 61, 1024},
	}
	cases := []struct {
		sttsIn               mp4.SttsBox
		lastSampleNr         uint32
		expectedSampleCounts []uint32
		expectedTimeDeltas   []uint32
	}{
		{stts1, 1, []uint32{1}, []uint32{1024}},
		{stts1, 2, []uint32{2}, []uint32{1024}},
		{stts1, 3189, []uint32{3189}, []uint32{1024}},
		{stts1, 3190, []uint32{3189, 1}, []uint32{1024, 6752}},
		{stts1, 3191, []uint32{3189, 1, 1}, []uint32{1024, 6752, 1024}},
		{stts1, 3191, []uint32{3189, 1, 1}, []uint32{1024, 6752, 1024}},
		{stts1, 157040, []uint32{3189, 1, 22968, 3, 1, 130878}, []uint32{1024, 6752, 1024, 1, 61, 1024}},
	}

	for _, c := range cases {
		stts := mp4.SttsBox{
			SampleCount:     make([]uint32, len(c.sttsIn.SampleCount)),
			SampleTimeDelta: make([]uint32, len(c.sttsIn.SampleTimeDelta)),
		}
		copy(stts.SampleCount, c.sttsIn.SampleCount)
		copy(stts.SampleTimeDelta, c.sttsIn.SampleTimeDelta)
		if err := cropStts(&stts, c.lastSampleNr); err != nil {
			t.Fatal(err)
		}
		if len(stts.SampleCount) != len(c.expectedSampleCounts) {
			t.Errorf("Expected %d sampleCounts, got %d", len(c.expectedSampleCounts), len(stts.SampleCount))
		}
		if len(stts.SampleTimeDelta) != len(c.expectedTimeDeltas) {
			t.Errorf("Expected %d timeDeltas, got %d", len(c.expectedTimeDeltas), len(stts.SampleTimeDelta))
		}
		for i := 0; i < len(stts.SampleCount); i++ {
			if stts.SampleCount[i] != c.expectedSampleCounts[i] {
				t.Errorf("Expected sampleCount %d to be %d, got %d", i, c.expectedSampleCounts[i], stts.SampleCount[i])
			}
		}
		for i := 0; i < len(stts.SampleTimeDelta); i++ {
			if stts.SampleTimeDelta[i] != c.expectedTimeDeltas[i] {
				t.Errorf("Expected timeDelta %d to be %d, got %d", i, c.expectedTimeDeltas[i], stts.SampleTimeDelta[i])
			}
		}
	}
}

// TestStscCrop checks the entries and their sample description IDs after cropping. Whole chunks keep
// their entries, a partly kept chunk after whole ones gets an entry of its own, and an entry whose first
// chunk is partly kept is shrunk, and merged into the entry before if they then match.
func TestStscCrop(t *testing.T) {
	threeIDs := [][3]uint32{{1, 2, 1}, {3, 2, 2}, {5, 2, 3}}
	cases := []struct {
		desc         string
		entries      [][3]uint32 // firstChunk, samplesPerChunk, sampleDescriptionID
		lastSampleNr uint32
		wantEntries  []mp4.StscEntry
		wantSDIDs    []uint32
	}{
		{
			desc:         "whole chunks",
			entries:      threeIDs,
			lastSampleNr: 8,
			wantEntries: []mp4.StscEntry{{FirstChunk: 1, SamplesPerChunk: 2, FirstSampleNr: 1},
				{FirstChunk: 3, SamplesPerChunk: 2, FirstSampleNr: 5}},
			wantSDIDs: []uint32{1, 2},
		},
		{
			desc:         "partly kept chunk",
			entries:      threeIDs,
			lastSampleNr: 7,
			wantEntries: []mp4.StscEntry{{FirstChunk: 1, SamplesPerChunk: 2, FirstSampleNr: 1},
				{FirstChunk: 3, SamplesPerChunk: 2, FirstSampleNr: 5}, {FirstChunk: 4, SamplesPerChunk: 1, FirstSampleNr: 7}},
			wantSDIDs: []uint32{1, 2, 2},
		},
		{
			desc:         "partly kept first chunk of an entry",
			entries:      threeIDs,
			lastSampleNr: 5,
			wantEntries: []mp4.StscEntry{{FirstChunk: 1, SamplesPerChunk: 2, FirstSampleNr: 1},
				{FirstChunk: 3, SamplesPerChunk: 1, FirstSampleNr: 5}},
			wantSDIDs: []uint32{1, 2},
		},
		{
			desc:         "partly kept chunk in last entry",
			entries:      threeIDs,
			lastSampleNr: 11,
			wantEntries: []mp4.StscEntry{{FirstChunk: 1, SamplesPerChunk: 2, FirstSampleNr: 1},
				{FirstChunk: 3, SamplesPerChunk: 2, FirstSampleNr: 5}, {FirstChunk: 5, SamplesPerChunk: 2, FirstSampleNr: 9},
				{FirstChunk: 6, SamplesPerChunk: 1, FirstSampleNr: 11}},
			wantSDIDs: []uint32{1, 2, 3, 3},
		},
		{
			desc:         "partly kept only chunk",
			entries:      [][3]uint32{{1, 432, 1}},
			lastSampleNr: 45,
			wantEntries:  []mp4.StscEntry{{FirstChunk: 1, SamplesPerChunk: 45, FirstSampleNr: 1}},
			wantSDIDs:    []uint32{1},
		},
		{
			desc:         "whole only chunk",
			entries:      [][3]uint32{{1, 432, 1}},
			lastSampleNr: 432,
			wantEntries:  []mp4.StscEntry{{FirstChunk: 1, SamplesPerChunk: 432, FirstSampleNr: 1}},
			wantSDIDs:    []uint32{1},
		},
		{
			desc:         "shrunk entry merged into the one before",
			entries:      [][3]uint32{{1, 1, 1}, {2, 2, 1}},
			lastSampleNr: 2,
			wantEntries:  []mp4.StscEntry{{FirstChunk: 1, SamplesPerChunk: 1, FirstSampleNr: 1}},
			wantSDIDs:    []uint32{1},
		},
		{
			desc:         "shrunk entry with other sample description ID",
			entries:      [][3]uint32{{1, 1, 1}, {2, 2, 2}},
			lastSampleNr: 2,
			wantEntries: []mp4.StscEntry{{FirstChunk: 1, SamplesPerChunk: 1, FirstSampleNr: 1},
				{FirstChunk: 2, SamplesPerChunk: 1, FirstSampleNr: 2}},
			wantSDIDs: []uint32{1, 2},
		},
		{
			desc:         "shrunk entry with other samples per chunk",
			entries:      [][3]uint32{{1, 1, 1}, {2, 3, 1}},
			lastSampleNr: 3,
			wantEntries: []mp4.StscEntry{{FirstChunk: 1, SamplesPerChunk: 1, FirstSampleNr: 1},
				{FirstChunk: 2, SamplesPerChunk: 2, FirstSampleNr: 2}},
			wantSDIDs: []uint32{1, 1},
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			stsc := &mp4.StscBox{}
			for _, e := range c.entries {
				if err := stsc.AddEntry(e[0], e[1], e[2]); err != nil {
					t.Fatal(err)
				}
			}
			if err := cropStsc(stsc, c.lastSampleNr); err != nil {
				t.Fatal(err)
			}
			if diff := deep.Equal(stsc.Entries, c.wantEntries); diff != nil {
				t.Errorf("entries: %v", diff)
			}
			var sdids []uint32
			for _, e := range stsc.Entries {
				sdids = append(sdids, stsc.GetSampleDescriptionID(int(e.FirstChunk)))
			}
			if diff := deep.Equal(sdids, c.wantSDIDs); diff != nil {
				t.Errorf("sample description IDs: %v", diff)
			}
		})
	}
}

// TestSbgpCrop checks that only the kept samples are mapped to groups.
func TestSbgpCrop(t *testing.T) {
	cases := []struct {
		desc         string
		lastSampleNr uint32
		wantCounts   []uint32
		wantIndices  []uint32
	}{
		{desc: "inside first entry", lastSampleNr: 3, wantCounts: []uint32{3}, wantIndices: []uint32{0}},
		{desc: "end of first entry", lastSampleNr: 4, wantCounts: []uint32{4}, wantIndices: []uint32{0}},
		{desc: "inside second entry", lastSampleNr: 25, wantCounts: []uint32{4, 21}, wantIndices: []uint32{0, 1}},
		{desc: "end of last entry", lastSampleNr: 50, wantCounts: []uint32{4, 46}, wantIndices: []uint32{0, 1}},
		{desc: "after last entry", lastSampleNr: 60, wantCounts: []uint32{4, 46}, wantIndices: []uint32{0, 1}},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			sbgp := &mp4.SbgpBox{GroupingType: "roll", SampleCounts: []uint32{4, 46}, GroupDescriptionIndices: []uint32{0, 1}}
			cropSbgp(sbgp, c.lastSampleNr)
			if diff := deep.Equal(sbgp.SampleCounts, c.wantCounts); diff != nil {
				t.Errorf("sample counts: %v", diff)
			}
			if diff := deep.Equal(sbgp.GroupDescriptionIndices, c.wantIndices); diff != nil {
				t.Errorf("group description indices: %v", diff)
			}
		})
	}
}

// TestSubsCrop checks that only the entries of kept samples remain.
func TestSubsCrop(t *testing.T) {
	newSubs := func() *mp4.SubsBox {
		// Entries for samples 2, 5 and 6
		return &mp4.SubsBox{Entries: []mp4.SubsEntry{{SampleDelta: 2}, {SampleDelta: 3}, {SampleDelta: 1}}}
	}
	for lastSampleNr, wantNrEntries := range map[uint32]int{1: 0, 2: 1, 4: 1, 5: 2, 6: 3, 7: 3} {
		subs := newSubs()
		cropSubs(subs, lastSampleNr)
		if len(subs.Entries) != wantNrEntries {
			t.Errorf("lastSampleNr %d: got %d entries instead of %d", lastSampleNr, len(subs.Entries), wantNrEntries)
		}
	}
}

// TestCropEdits checks that edits are cropped to the kept media and to the end of the movie.
// The media timescale is 44100 and the movie timescale 1000.
func TestCropEdits(t *testing.T) {
	const noMax = math.MaxUint64
	cases := []struct {
		desc     string
		edits    []mp4.ElstEntry
		compEnd  uint64
		maxDur   uint64
		wantEdit []mp4.ElstEntry
		wantDur  uint64
	}{
		{
			desc:     "media edit after priming, cropped to media",
			edits:    []mp4.ElstEntry{{SegmentDuration: 9900, MediaTime: 1024, MediaRateInteger: 1}},
			compEnd:  117760,
			maxDur:   noMax,
			wantEdit: []mp4.ElstEntry{{SegmentDuration: 2648, MediaTime: 1024, MediaRateInteger: 1}},
			wantDur:  2648,
		},
		{
			desc:     "media edit after priming, cropped to movie end",
			edits:    []mp4.ElstEntry{{SegmentDuration: 9900, MediaTime: 1024, MediaRateInteger: 1}},
			compEnd:  117760,
			maxDur:   2625,
			wantEdit: []mp4.ElstEntry{{SegmentDuration: 2625, MediaTime: 1024, MediaRateInteger: 1}},
			wantDur:  2625,
		},
		{
			desc:     "uncropped media keeps its edit",
			edits:    []mp4.ElstEntry{{SegmentDuration: 9900, MediaTime: 1024, MediaRateInteger: 1}},
			compEnd:  437614,
			maxDur:   noMax,
			wantEdit: []mp4.ElstEntry{{SegmentDuration: 9900, MediaTime: 1024, MediaRateInteger: 1}},
			wantDur:  9900,
		},
		{
			desc: "empty edit before media",
			edits: []mp4.ElstEntry{{SegmentDuration: 500, MediaTime: -1, MediaRateInteger: 1},
				{SegmentDuration: 9000, MediaTime: 0, MediaRateInteger: 1}},
			compEnd: 44100,
			maxDur:  noMax,
			wantEdit: []mp4.ElstEntry{{SegmentDuration: 500, MediaTime: -1, MediaRateInteger: 1},
				{SegmentDuration: 1000, MediaTime: 0, MediaRateInteger: 1}},
			wantDur: 1500,
		},
		{
			desc: "movie ends in empty edit",
			edits: []mp4.ElstEntry{{SegmentDuration: 500, MediaTime: -1, MediaRateInteger: 1},
				{SegmentDuration: 9000, MediaTime: 0, MediaRateInteger: 1}},
			compEnd:  44100,
			maxDur:   300,
			wantEdit: []mp4.ElstEntry{{SegmentDuration: 300, MediaTime: -1, MediaRateInteger: 1}},
			wantDur:  300,
		},
		{
			desc: "second media edit starts after kept media",
			edits: []mp4.ElstEntry{{SegmentDuration: 1000, MediaTime: 0, MediaRateInteger: 1},
				{SegmentDuration: 1000, MediaTime: 88200, MediaRateInteger: 1}},
			compEnd:  88200,
			maxDur:   noMax,
			wantEdit: []mp4.ElstEntry{{SegmentDuration: 1000, MediaTime: 0, MediaRateInteger: 1}},
			wantDur:  1000,
		},
		{
			desc:     "media edit with duration 0 lasts to the end of the media",
			edits:    []mp4.ElstEntry{{SegmentDuration: 0, MediaTime: 0, MediaRateInteger: 1}},
			compEnd:  44100,
			maxDur:   noMax,
			wantEdit: []mp4.ElstEntry{{SegmentDuration: 1000, MediaTime: 0, MediaRateInteger: 1}},
			wantDur:  1000,
		},
		{
			desc:     "dwell edit",
			edits:    []mp4.ElstEntry{{SegmentDuration: 3000, MediaTime: 0, MediaRateInteger: 0}},
			compEnd:  44100,
			maxDur:   2000,
			wantEdit: []mp4.ElstEntry{{SegmentDuration: 2000, MediaTime: 0, MediaRateInteger: 0}},
			wantDur:  2000,
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			edits := append([]mp4.ElstEntry(nil), c.edits...)
			gotEdits, gotDur := cropEdits(edits, c.compEnd, 44100, 1000, c.maxDur)
			if diff := deep.Equal(gotEdits, c.wantEdit); diff != nil {
				t.Errorf("edits: %v", diff)
			}
			if gotDur != c.wantDur {
				t.Errorf("got duration %d instead of %d", gotDur, c.wantDur)
			}
			if diff := deep.Equal(edits, c.edits); diff != nil {
				t.Errorf("input edits changed: %v", diff)
			}
		})
	}
}

// TestMediaTimeAt checks the media time, with timescale 44100, at a movie time, with timescale 1000.
func TestMediaTimeAt(t *testing.T) {
	afterEmpty := []mp4.ElstEntry{{SegmentDuration: 500, MediaTime: -1, MediaRateInteger: 1},
		{SegmentDuration: 1000, MediaTime: 0, MediaRateInteger: 1}}
	cases := []struct {
		desc      string
		edits     []mp4.ElstEntry
		movieTime uint64
		want      uint64
	}{
		{desc: "no edits", movieTime: 2000, want: 88200},
		{desc: "no edits, rounded up", movieTime: 1, want: 45},
		{desc: "priming", edits: []mp4.ElstEntry{{SegmentDuration: 9900, MediaTime: 1024, MediaRateInteger: 1}},
			movieTime: 2625, want: 1024 + 115763},
		{desc: "in empty edit", edits: afterEmpty, movieTime: 300, want: 0},
		{desc: "in media edit after empty edit", edits: afterEmpty, movieTime: 1000, want: 22050},
		{desc: "after last edit", edits: afterEmpty, movieTime: 2000, want: 44100},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			if got := mediaTimeAt(c.edits, c.movieTime, 44100, 1000); got != c.want {
				t.Errorf("got %d instead of %d", got, c.want)
			}
		})
	}
}

// TestCompositionEnd checks the composition end time of an I, P, B, B sequence in decode order,
// where the P sample is presented last.
func TestCompositionEnd(t *testing.T) {
	stts := &mp4.SttsBox{SampleCount: []uint32{4}, SampleTimeDelta: []uint32{512}}
	ctts := &mp4.CttsBox{}
	if err := ctts.AddSampleCountsAndOffset([]uint32{1, 1, 2}, []int32{1024, 2048, 512}); err != nil {
		t.Fatal(err)
	}
	// Composition times: I 1024, P 2560, B 1536, B 2048.
	for lastSampleNr, want := range map[uint32]uint64{1: 1536, 2: 3072, 3: 3072, 4: 3072} {
		if got := compositionEnd(stts, ctts, lastSampleNr); got != want {
			t.Errorf("lastSampleNr %d: got %d instead of %d", lastSampleNr, got, want)
		}
	}
}

// TestFirstSyncSampleNrAtOrAfter checks the search for the sync sample to crop before. Four samples of
// duration 512 start at decode times 0, 512, 1024 and 1536. The first sample is never chosen.
func TestFirstSyncSampleNrAtOrAfter(t *testing.T) {
	ctts := &mp4.CttsBox{}
	if err := ctts.AddSampleCountsAndOffset([]uint32{4}, []int32{1024}); err != nil {
		t.Fatal(err)
	}
	stss := &mp4.StssBox{SampleNumber: []uint32{1, 3}}
	cases := []struct {
		desc string
		stss *mp4.StssBox
		ctts *mp4.CttsBox
		t    uint64
		want uint32
	}{
		{desc: "all sync, at start", t: 0, want: 2},
		{desc: "all sync", t: 600, want: 3},
		{desc: "all sync, after last", t: 2000, want: 5},
		{desc: "stss", stss: stss, t: 600, want: 3},
		{desc: "stss, after last sync", stss: stss, t: 1500, want: 5},
		{desc: "stss and composition offsets", stss: stss, ctts: ctts, t: 1500, want: 3},
		{desc: "stss and composition offsets, after last sync", stss: stss, ctts: ctts, t: 2049, want: 5},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			stbl := &mp4.StblBox{
				Stts: &mp4.SttsBox{SampleCount: []uint32{4}, SampleTimeDelta: []uint32{512}},
				Stsz: &mp4.StszBox{SampleNumber: 4},
				Stss: c.stss,
				Ctts: c.ctts,
			}
			if got := firstSyncSampleNrAtOrAfter(stbl, c.t); got != c.want {
				t.Errorf("got sample %d instead of %d", got, c.want)
			}
		})
	}
}
