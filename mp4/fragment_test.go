package mp4_test

import (
	"bytes"
	"cmp"
	"encoding/binary"
	"encoding/hex"
	"io"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

func TestCreateMultiTrackFragment(t *testing.T) {

	trackIDs := []uint32{1, 2, 3}
	mFrag, err := mp4.CreateMultiTrackFragment(1, trackIDs)
	if err != nil {
		t.Error("Error creating MultiTrackFragment")
	}
	if len(mFrag.Moof.Trafs) != 3 {
		t.Error("Not 3 tracks in MultiTrackFragment")
	}
}

func TestFragmentSampleIntervals(t *testing.T) {
	frag, err := mp4.CreateFragment(12, 1)
	if err != nil {
		t.Error("Error creating Fragment")
	}
	s := mp4.NewSample(0, 100, 1, 0)
	frag.AddSample(s, 1230)
	samples := []mp4.Sample{mp4.NewSample(0, 100, 2, 0), mp4.NewSample(0, 100, 3, 0), mp4.NewSample(0, 100, 4, 0)}
	frag.AddSamples(samples, 1330)

	sampleNr, err := frag.GetSampleNrFromTime(nil, 1430)
	if err != nil {
		t.Error("Error getting sample number from time")
	}
	if sampleNr != 3 {
		t.Error("Wrong sample number from time")
	}

	sIntv, err := frag.GetSampleInterval(nil, 2, 3)
	if err != nil {
		t.Error("Error getting sample interval")
	}
	if sIntv.FirstDecodeTime != 1330 {
		t.Error("Wrong first decode time")
	}

	// Check common sample duration from trex
	_, err = frag.CommonSampleDuration(nil)
	if err == nil {
		t.Error("Should have gotten error from CommonSampleDuration")
	}

	sampleItvl := mp4.SampleInterval{
		FirstDecodeTime: 1630,
		Samples:         []mp4.Sample{{0, 100, 2, 0}},
		OffsetInMdat:    0,
		Data:            []byte{},
	}
	err = frag.AddSampleInterval(sampleItvl)
	if err != nil {
		t.Error("Error adding sample interval")
	}
	sampleItvl.Reset()
}

func TestFragmentGetFullSamplesTruncatedMdat(t *testing.T) {
	frag, err := mp4.CreateFragment(1, 1)
	if err != nil {
		t.Fatal(err)
	}
	fs := mp4.FullSample{
		Sample: mp4.Sample{
			Flags: mp4.SyncSampleFlags,
			Dur:   1024,
			Size:  4,
		},
		DecodeTime: 0,
		Data:       []byte{0, 1, 2, 3},
	}
	frag.AddFullSample(fs)
	// Simulate a truncated file: the trun declares more sample bytes
	// than the mdat carries.
	frag.Mdat.Data = frag.Mdat.Data[:2]

	if _, err := frag.GetFullSamples(nil); err == nil {
		t.Error("expected error when mdat is truncated, got nil")
	}
}

// fullSamplesFixture returns nrSamples test samples whose data is laid out as
// described by layout: "contiguous" (subslices of one buffer, like the output
// of GetFullSamples), "scattered" (each sample its own allocation), "chunked"
// (two buffers, so two contiguous runs), or "capped" (subslices of one buffer
// with their capacity cut short by three-index slicing, so never extendable).
func fullSamplesFixture(layout string, nrSamples int, firstDecodeTime uint64) []mp4.FullSample {
	sizes := make([]int, nrSamples)
	total := 0
	for i := range sizes {
		sizes[i] = 100 + i
		total += sizes[i]
	}
	fill := func(b []byte, v byte) {
		for j := range b {
			b[j] = v
		}
	}
	var datas [][]byte
	switch layout {
	case "contiguous", "capped":
		buf := make([]byte, total)
		pos := 0
		for i, size := range sizes {
			d := buf[pos : pos+size]
			if layout == "capped" {
				d = buf[pos : pos+size : pos+size]
			}
			fill(d, byte(i))
			datas = append(datas, d)
			pos += size
		}
	case "scattered":
		for i, size := range sizes {
			d := make([]byte, size)
			fill(d, byte(i))
			datas = append(datas, d)
		}
	case "chunked":
		half := nrSamples / 2
		bufs := [2][]byte{make([]byte, total), make([]byte, total)}
		pos := [2]int{}
		for i, size := range sizes {
			b := 0
			if i >= half {
				b = 1
			}
			d := bufs[b][pos[b] : pos[b]+size]
			fill(d, byte(i))
			datas = append(datas, d)
			pos[b] += size
		}
	default:
		panic("unknown layout " + layout)
	}
	samples := make([]mp4.FullSample, 0, nrSamples)
	decTime := firstDecodeTime
	for i, d := range datas {
		samples = append(samples, mp4.FullSample{
			Sample: mp4.Sample{
				Flags:                 mp4.SyncSampleFlags,
				Dur:                   1024,
				Size:                  uint32(len(d)),
				CompositionTimeOffset: int32(i % 3),
			},
			DecodeTime: decTime,
			Data:       d,
		})
		decTime += 1024
	}
	return samples
}

func encodeFragment(t *testing.T, frag *mp4.Fragment) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := frag.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func fragmentFromLoop(t *testing.T, samples []mp4.FullSample) *mp4.Fragment {
	t.Helper()
	frag, err := mp4.CreateFragment(1, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range samples {
		frag.AddFullSample(s)
	}
	return frag
}

func TestAddFullSamples(t *testing.T) {
	cases := []struct {
		layout        string
		expectedParts int
	}{
		{"contiguous", 1},
		{"chunked", 2},
		{"scattered", 20},
		{"capped", 20},
	}
	for _, c := range cases {
		t.Run(c.layout, func(t *testing.T) {
			samples := fullSamplesFixture(c.layout, 20, 10000)
			want := encodeFragment(t, fragmentFromLoop(t, samples))

			frag, err := mp4.CreateFragment(1, 1)
			if err != nil {
				t.Fatal(err)
			}
			frag.AddFullSamples(samples)
			if got := encodeFragment(t, frag); !bytes.Equal(got, want) {
				t.Error("bulk-added fragment does not encode identically to sample-by-sample fragment")
			}
			if got := frag.Moof.Traf.Tfdt.BaseMediaDecodeTime(); got != 10000 {
				t.Errorf("got baseMediaDecodeTime %d instead of 10000", got)
			}
			if nr := len(frag.Mdat.DataParts); nr != c.expectedParts {
				t.Errorf("got %d mdat data parts, expected %d", nr, c.expectedParts)
			}
			if len(frag.Mdat.Data) != 0 {
				t.Error("parts-based fragment should not hold monolithic mdat data")
			}
			// Zero copies: the first part is the caller's own buffer.
			if c.expectedParts > 0 && &frag.Mdat.DataParts[0][0] != &samples[0].Data[0] {
				t.Error("first data part does not alias the first sample's data")
			}
		})
	}
}

func TestAddFullSamplesMixing(t *testing.T) {
	first := fullSamplesFixture("contiguous", 8, 10000)
	second := fullSamplesFixture("contiguous", 8, 10000+8*1024)
	all := append(append([]mp4.FullSample{}, first...), second...)
	want := encodeFragment(t, fragmentFromLoop(t, all))

	// Monolithic data already present: it is closed into a data part and the
	// bulk samples are added as parts after it, without being copied. The
	// existing baseMediaDecodeTime is kept.
	frag, err := mp4.CreateFragment(1, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range first {
		frag.AddFullSample(s)
	}
	frag.AddFullSamples(second)
	if got := encodeFragment(t, frag); !bytes.Equal(got, want) {
		t.Error("AddFullSamples after AddFullSample does not encode identically to the loop")
	}
	if nr := len(frag.Mdat.DataParts); nr != 2 {
		t.Errorf("got %d mdat data parts, expected 2 (closed monolithic data, then the bulk run)", nr)
	}
	if len(frag.Mdat.Data) != 0 {
		t.Error("monolithic data should have been closed into a part")
	}
	// The bulk samples are referenced, not copied.
	if &frag.Mdat.DataParts[1][0] != &second[0].Data[0] {
		t.Error("second data part does not alias the first bulk sample's data")
	}
	if got := frag.Moof.Traf.Tfdt.BaseMediaDecodeTime(); got != 10000 {
		t.Errorf("got baseMediaDecodeTime %d instead of 10000", got)
	}

	// Two bulk calls: both become parts, and the second keeps the first's
	// baseMediaDecodeTime.
	frag, err = mp4.CreateFragment(1, 1)
	if err != nil {
		t.Fatal(err)
	}
	frag.AddFullSamples(first)
	frag.AddFullSamples(second)
	if got := encodeFragment(t, frag); !bytes.Equal(got, want) {
		t.Error("two AddFullSamples calls do not encode identically to the loop")
	}
	if nr := len(frag.Mdat.DataParts); nr != 2 {
		t.Errorf("got %d mdat data parts, expected 2", nr)
	}
	if got := frag.Moof.Traf.Tfdt.BaseMediaDecodeTime(); got != 10000 {
		t.Errorf("got baseMediaDecodeTime %d instead of 10000", got)
	}

	// Zero-length sample data adds a trun entry but no data part.
	withEmpty := fullSamplesFixture("scattered", 4, 10000)
	withEmpty[1].Data = nil
	withEmpty[1].Size = 0
	frag, err = mp4.CreateFragment(1, 1)
	if err != nil {
		t.Fatal(err)
	}
	frag.AddFullSamples(withEmpty)
	if got := encodeFragment(t, frag); !bytes.Equal(got, encodeFragment(t, fragmentFromLoop(t, withEmpty))) {
		t.Error("fragment with an empty sample does not encode identically to the loop")
	}
	if nr := len(frag.Mdat.DataParts); nr != 3 {
		t.Errorf("got %d mdat data parts, expected 3", nr)
	}

	// An empty slice is a no-op and must not set a decode time.
	frag, err = mp4.CreateFragment(1, 1)
	if err != nil {
		t.Fatal(err)
	}
	frag.AddFullSamples(nil)
	if nr := frag.Moof.Traf.Trun.SampleCount(); nr != 0 {
		t.Errorf("got %d samples after adding none", nr)
	}
}

// TestAddFullSamplesLargeSizeMdat checks the extended-size (64-bit) mdat
// header, which is legal at any payload size: bulk and sample-by-sample
// fragments encode identically, the header really is extended, and a decode
// round trip recovers the samples.
func TestAddFullSamplesLargeSizeMdat(t *testing.T) {
	samples := fullSamplesFixture("contiguous", 20, 10000)
	fragLoop := fragmentFromLoop(t, samples)
	normal := encodeFragment(t, fragLoop)
	fragLoop.Mdat.LargeSize = true
	want := encodeFragment(t, fragLoop)

	frag, err := mp4.CreateFragment(1, 1)
	if err != nil {
		t.Fatal(err)
	}
	frag.AddFullSamples(samples)
	frag.Mdat.LargeSize = true
	got := encodeFragment(t, frag)
	if !bytes.Equal(got, want) {
		t.Error("large-size mdat: bulk fragment does not encode identically to sample-by-sample fragment")
	}
	if len(got) != len(normal)+8 {
		t.Errorf("large-size mdat output is %d bytes, expected %d (8 more than normal)", len(got), len(normal)+8)
	}
	mdatStart := int(frag.Moof.Size())
	if size := binary.BigEndian.Uint32(got[mdatStart:]); size != 1 {
		t.Errorf("mdat size field is %d, expected 1 (extended size follows)", size)
	}
	if typ := string(got[mdatStart+4 : mdatStart+8]); typ != "mdat" {
		t.Errorf("box type at mdat start is %q", typ)
	}
	if largeSize := binary.BigEndian.Uint64(got[mdatStart+8:]); largeSize != uint64(len(got)-mdatStart) {
		t.Errorf("mdat largesize is %d, expected %d", largeSize, len(got)-mdatStart)
	}

	decoded, err := mp4.DecodeFileSR(bits.NewFixedSliceReader(got))
	if err != nil {
		t.Fatal(err)
	}
	if len(decoded.Segments) != 1 || len(decoded.Segments[0].Fragments) != 1 {
		t.Fatalf("decoded %d segments", len(decoded.Segments))
	}
	fss, err := decoded.Segments[0].Fragments[0].GetFullSamples(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(fss) != len(samples) {
		t.Fatalf("decoded %d samples, expected %d", len(fss), len(samples))
	}
	for i := range fss {
		if !bytes.Equal(fss[i].Data, samples[i].Data) {
			t.Errorf("sample %d data differs after large-size mdat round trip", i+1)
		}
		if fss[i].DecodeTime != samples[i].DecodeTime {
			t.Errorf("sample %d decode time %d, expected %d", i+1, fss[i].DecodeTime, samples[i].DecodeTime)
		}
	}
}

// TestAddFullSamplesThenAddFullSample - adding single samples after a bulk
// call must extend the fragment rather than being silently dropped. Before
// mdat combined parts and monolithic data, the mdat encoded only the parts,
// so the trun declared samples whose data was never written and the fragment
// was rejected on decode.
func TestAddFullSamplesThenAddFullSample(t *testing.T) {
	bulk := fullSamplesFixture("contiguous", 8, 10000)
	extra := fullSamplesFixture("scattered", 3, 10000+8*1024)
	all := append(append([]mp4.FullSample{}, bulk...), extra...)
	want := encodeFragment(t, fragmentFromLoop(t, all))

	frag, err := mp4.CreateFragment(1, 1)
	if err != nil {
		t.Fatal(err)
	}
	frag.AddFullSamples(bulk)
	for _, s := range extra {
		frag.AddFullSample(s)
	}

	var declared uint64
	for _, s := range frag.Moof.Traf.Trun.Samples {
		declared += uint64(s.Size)
	}
	if got := frag.Mdat.DataLength(); got != declared {
		t.Errorf("mdat holds %d payload bytes but trun declares %d", got, declared)
	}
	if got := encodeFragment(t, frag); !bytes.Equal(got, want) {
		t.Error("bulk followed by single samples does not encode identically to the loop")
	}

	// The fragment must survive a decode round trip with its samples intact.
	decoded, err := mp4.DecodeFile(bytes.NewReader(encodeFragment(t, frag)))
	if err != nil {
		t.Fatal(err)
	}
	fss, err := decoded.Segments[0].Fragments[0].GetFullSamples(nil)
	if err != nil {
		t.Fatalf("GetFullSamples after mixed adds: %v", err)
	}
	if len(fss) != len(all) {
		t.Fatalf("decoded %d samples, expected %d", len(fss), len(all))
	}
	for i := range fss {
		if !bytes.Equal(fss[i].Data, all[i].Data) {
			t.Errorf("sample %d data differs after round trip", i+1)
		}
	}
}

// checkFragmentSamples checks that GetFullSamples, Samples and GetSampleInterval of frag give back samples.
func checkFragmentSamples(t *testing.T, frag *mp4.Fragment, trex *mp4.TrexBox, samples []mp4.FullSample) {
	t.Helper()
	fss, err := frag.GetFullSamples(trex)
	if err != nil {
		t.Fatalf("GetFullSamples: %v", err)
	}
	if len(fss) != len(samples) {
		t.Fatalf("GetFullSamples gave %d samples, expected %d", len(fss), len(samples))
	}
	for i := range fss {
		if !bytes.Equal(fss[i].Data, samples[i].Data) {
			t.Errorf("GetFullSamples: sample %d data differs", i+1)
		}
		if fss[i].DecodeTime != samples[i].DecodeTime {
			t.Errorf("GetFullSamples: sample %d decode time %d, expected %d", i+1, fss[i].DecodeTime,
				samples[i].DecodeTime)
		}
	}
	nr := 0
	for s, err := range frag.Samples(trex) {
		if err != nil {
			t.Fatalf("Samples: %v", err)
		}
		if !bytes.Equal(s.Data, samples[nr].Data) {
			t.Errorf("Samples: sample %d data differs", nr+1)
		}
		nr++
	}
	if nr != len(samples) {
		t.Errorf("Samples gave %d samples, expected %d", nr, len(samples))
	}
	if len(frag.Moof.Trafs) != 1 {
		return // GetSampleInterval needs a single track
	}
	for _, itv := range [][2]int{{1, len(samples)}, {2, len(samples) - 1}, {len(samples), len(samples)}} {
		si, err := frag.GetSampleInterval(trex, uint32(itv[0]), uint32(itv[1]))
		if err != nil {
			t.Fatalf("GetSampleInterval(%d, %d): %v", itv[0], itv[1], err)
		}
		var want []byte
		for _, s := range samples[itv[0]-1 : itv[1]] {
			want = append(want, s.Data...)
		}
		if !bytes.Equal(si.Data, want) {
			t.Errorf("GetSampleInterval(%d, %d): data differs", itv[0], itv[1])
		}
		if si.FirstDecodeTime != samples[itv[0]-1].DecodeTime {
			t.Errorf("GetSampleInterval(%d, %d): first decode time %d, expected %d", itv[0], itv[1],
				si.FirstDecodeTime, samples[itv[0]-1].DecodeTime)
		}
	}
}

// TestBuiltFragmentSamples - the samples of a fragment can be read back with GetFullSamples, Samples and
// GetSampleInterval however it was built, before it is encoded, after it is encoded, and after it is decoded
// again. A fragment built with AddFullSamples has its sample data in mdat data parts until it is decoded.
func TestBuiltFragmentSamples(t *testing.T) {
	builds := []struct {
		name  string
		build func(f *mp4.Fragment, ss []mp4.FullSample)
	}{
		{"AddFullSample", func(f *mp4.Fragment, ss []mp4.FullSample) {
			for _, s := range ss {
				f.AddFullSample(s)
			}
		}},
		{"AddFullSamples", func(f *mp4.Fragment, ss []mp4.FullSample) { f.AddFullSamples(ss) }},
		{"mixed", func(f *mp4.Fragment, ss []mp4.FullSample) {
			f.AddFullSample(ss[0])
			f.AddFullSamples(ss[1:5])
			f.AddFullSample(ss[5])
			f.AddFullSamples(ss[6:])
		}},
	}
	for _, b := range builds {
		for _, layout := range []string{"contiguous", "chunked", "scattered"} {
			t.Run(b.name+"/"+layout, func(t *testing.T) {
				samples := fullSamplesFixture(layout, 10, 10000)
				frag, err := mp4.CreateFragment(1, 1)
				if err != nil {
					t.Fatal(err)
				}
				b.build(frag, samples)
				checkFragmentSamples(t, frag, nil, samples)
				if b.name == "AddFullSamples" {
					fss, _ := frag.GetFullSamples(nil)
					if &fss[0].Data[0] != &samples[0].Data[0] {
						t.Error("GetFullSamples data is not a view into the added buffer")
					}
					allocs := testing.AllocsPerRun(10, func() {
						fss, _ = frag.AppendFullSamples(fss[:0], nil)
					})
					if allocs != 0 {
						t.Errorf("AppendFullSamples made %.0f allocations, expected none", allocs)
					}
				}
				encoded := encodeFragment(t, frag)
				checkFragmentSamples(t, frag, nil, samples)

				decoded, err := mp4.DecodeFile(bytes.NewReader(encoded))
				if err != nil {
					t.Fatal(err)
				}
				checkFragmentSamples(t, decoded.Segments[0].Fragments[0], nil, samples)
			})
		}
	}
}

// TestBuiltMultiTrackFragmentSamples - the samples of each track of a built fragment are found where Encode
// writes them, in write order, before it is encoded, after it is encoded, and after it is decoded again.
func TestBuiltMultiTrackFragmentSamples(t *testing.T) {
	video := fullSamplesFixture("scattered", 6, 10000)
	audio := fullSamplesFixture("contiguous", 6, 20000)
	for i := range audio {
		for j := range audio[i].Data {
			audio[i].Data[j] += 100 // differ from video
		}
	}
	frag, err := mp4.CreateMultiTrackFragment(1, []uint32{1, 2})
	if err != nil {
		t.Fatal(err)
	}
	// Two truns per track, so that the write order interleaves the tracks.
	for _, part := range [][2]int{{0, 3}, {3, 6}} {
		for _, s := range video[part[0]:part[1]] {
			if err := frag.AddFullSampleToTrack(s, 1); err != nil {
				t.Fatal(err)
			}
		}
		for _, s := range audio[part[0]:part[1]] {
			if err := frag.AddFullSampleToTrack(s, 2); err != nil {
				t.Fatal(err)
			}
		}
	}
	trexs := []*mp4.TrexBox{{TrackID: 1}, {TrackID: 2}}
	check := func(frag *mp4.Fragment) {
		t.Helper()
		checkFragmentSamples(t, frag, trexs[0], video)
		checkFragmentSamples(t, frag, trexs[1], audio)
	}
	check(frag)
	encoded := encodeFragment(t, frag)
	check(frag)
	decoded, err := mp4.DecodeFile(bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	check(decoded.Segments[0].Fragments[0])
}

// TestBuiltFragmentTrunLayouts - in a built fragment, the trun data are in write order, and truns with the same
// write order number are in moof order. The samples of each track are found there, whether the truns of every
// traf are in write order or not, and agree with what Encode writes.
func TestBuiltFragmentTrunLayouts(t *testing.T) {
	cases := []struct {
		desc    string
		truns   [][]uint32 // for each track, the write order numbers of its truns
		encodes bool       // whether Encode lays out the trun data unambiguously, which needs distinct numbers
	}{
		{"interleaved", [][]uint32{{0, 3, 6, 9}, {1, 4, 7}, {2, 5, 8, 10, 11}}, true},
		{"one track after the other", [][]uint32{{0, 1}, {2, 3}}, true},
		{"empty track", [][]uint32{{0, 2}, {}, {1}}, true},
		{"not in write order", [][]uint32{{3, 0}, {1, 2}}, true},
		{"equal write order numbers", [][]uint32{{1, 1}, {1, 2}, {0, 2}}, false},
		{"unset write order", [][]uint32{{0, 0}, {0}}, false},
	}
	type trunData struct {
		writeOrderNr uint32
		data         []byte
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			trackIDs := make([]uint32, len(c.truns))
			for i := range trackIDs {
				trackIDs[i] = uint32(i + 1)
			}
			frag, err := mp4.CreateMultiTrackFragment(1, trackIDs)
			if err != nil {
				t.Fatal(err)
			}
			var truns []trunData // in moof order
			samples := make([][]mp4.FullSample, len(c.truns))
			for i, writeOrderNrs := range c.truns {
				var decodeTime uint64
				for j, writeOrderNr := range writeOrderNrs {
					trun := mp4.CreateTrun(writeOrderNr)
					if err := frag.Moof.Trafs[i].AddChild(trun); err != nil {
						t.Fatal(err)
					}
					var data []byte
					for k := 0; k < 2; k++ {
						d := bytes.Repeat([]byte{byte(16*i + 4*j + k)}, 10*i+3*j+k+1) // unique size and content
						s := mp4.Sample{Flags: mp4.SyncSampleFlags, Dur: 1000, Size: uint32(len(d))}
						trun.AddSample(s)
						samples[i] = append(samples[i], mp4.FullSample{Sample: s, DecodeTime: decodeTime, Data: d})
						decodeTime += 1000
						data = append(data, d...)
					}
					truns = append(truns, trunData{writeOrderNr, data})
				}
			}
			slices.SortStableFunc(truns, func(a, b trunData) int { return cmp.Compare(a.writeOrderNr, b.writeOrderNr) })
			for _, td := range truns {
				frag.Mdat.AddSampleData(td.data)
			}
			for i := range c.truns {
				checkFragmentSamples(t, frag, &mp4.TrexBox{TrackID: trackIDs[i]}, samples[i])
			}
			if !c.encodes {
				return
			}
			decoded, err := mp4.DecodeFile(bytes.NewReader(encodeFragment(t, frag)))
			if err != nil {
				t.Fatal(err)
			}
			for i := range c.truns {
				checkFragmentSamples(t, decoded.Segments[0].Fragments[0], &mp4.TrexBox{TrackID: trackIDs[i]},
					samples[i])
			}
		})
	}
}

// TestSampleSpanningDataParts - a sample whose data is split over two mdat data parts cannot be returned as a
// view, so GetFullSamples and Samples give an error, while GetSampleInterval returns a copy of the data.
func TestSampleSpanningDataParts(t *testing.T) {
	frag, err := mp4.CreateFragment(1, 1)
	if err != nil {
		t.Fatal(err)
	}
	frag.AddSample(mp4.Sample{Flags: mp4.SyncSampleFlags, Dur: 1024, Size: 4}, 0)
	frag.AddSample(mp4.Sample{Flags: mp4.NonSyncSampleFlags, Dur: 1024, Size: 3}, 0)
	frag.Mdat.SetLazyDataSize(0)
	frag.Mdat.AddSampleDataPart([]byte{1, 2, 3, 4, 5})
	frag.Mdat.AddSampleDataPart([]byte{6, 7})

	if _, err := frag.GetFullSamples(nil); err == nil || !strings.Contains(err.Error(), "sample 2") ||
		!strings.Contains(err.Error(), "spans more than one mdat data part") {
		t.Errorf("GetFullSamples: got error %v, expected sample 2 to span data parts", err)
	}
	nr := 0
	for _, err := range frag.Samples(nil) {
		if err == nil || !strings.Contains(err.Error(), "spans more than one mdat data part") {
			t.Errorf("Samples: got error %v, expected sample 2 to span data parts", err)
		}
		nr++
	}
	if nr != 1 {
		t.Errorf("Samples yielded %d times, expected once with the error", nr)
	}
	si, err := frag.GetSampleInterval(nil, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if want := []byte{1, 2, 3, 4, 5, 6, 7}; !bytes.Equal(si.Data, want) {
		t.Errorf("GetSampleInterval data is %v, expected %v", si.Data, want)
	}
}

// fragmentedTestFiles are the test files with fragments that have both moof and mdat.
var fragmentedTestFiles = []string{
	"1.m4s", "aac_1.m4s", "av1_multitile_seg.m4s", "bbb5s_aac_sidx.mp4", "cbcs.mp4", "golden_1_frag.m4s",
	"hvc1_seg_1.m4s", "interleaved_sidxs_segment.m4s", "multi_sidx_segment.m4s", "opus.mp4",
	"prog_8s_enc_dashinit.mp4", "seg_cenc_no_seig.m4v", "v300_multiple_segments.mp4", "vvc_400kbps_2s.mp4",
}

func encodeFragmentSW(t *testing.T, frag *mp4.Fragment) []byte {
	t.Helper()
	sw := bits.NewFixedSliceWriter(int(frag.Size()))
	if err := frag.EncodeSW(sw); err != nil {
		t.Fatal(err)
	}
	return sw.Bytes()
}

// TestFragmentEncodeMatchesEncodeSW checks that Fragment.Encode, which assembles everything but the mdat
// payload in one buffer, writes the same bytes as Fragment.EncodeSW for every fragment of the test files.
// With a lazily decoded mdat, both write only its header.
func TestFragmentEncodeMatchesEncodeSW(t *testing.T) {
	for _, name := range fragmentedTestFiles {
		data, err := os.ReadFile("testdata/" + name)
		if err != nil {
			t.Fatal(err)
		}
		for _, lazy := range []bool{false, true} {
			var f *mp4.File
			if lazy {
				f, err = mp4.DecodeFile(bytes.NewReader(data), mp4.WithDecodeMode(mp4.DecModeLazyMdat))
			} else {
				f, err = mp4.DecodeFileSR(bits.NewFixedSliceReader(data))
			}
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			nrFrags := 0
			for _, seg := range f.Segments {
				for _, frag := range seg.Fragments {
					nrFrags++
					if got, want := encodeFragment(t, frag), encodeFragmentSW(t, frag); !bytes.Equal(got, want) {
						t.Errorf("%s (lazy=%t) fragment %d: Encode and EncodeSW differ", name, lazy, nrFrags)
					}
				}
			}
			if nrFrags == 0 {
				t.Errorf("%s (lazy=%t): no fragments", name, lazy)
			}
		}
	}
}

// TestFragmentEncodeAllocations checks that Fragment.Encode of a decoded fragment does not allocate once its
// pooled buffer exists. A garbage collection that empties the pool costs a couple of allocations, which the
// average over the runs rounds away.
func TestFragmentEncodeAllocations(t *testing.T) {
	data, err := os.ReadFile("testdata/1.m4s")
	if err != nil {
		t.Fatal(err)
	}
	f, err := mp4.DecodeFileSR(bits.NewFixedSliceReader(data))
	if err != nil {
		t.Fatal(err)
	}
	frag := f.Segments[0].Fragments[0]
	allocs := testing.AllocsPerRun(100, func() {
		if err := frag.Encode(io.Discard); err != nil {
			t.Fatal(err)
		}
	})
	if allocs != 0 {
		t.Errorf("got %.0f allocations, want 0", allocs)
	}
}

// TestFragmentEncodeTopLevelBoxes checks Fragment.Encode on a fragment with emsg and prft before moof, and
// with more truns than SetTrunDataOffsets sorts without allocating, since two tracks alternate sample by
// sample. Every trun data offset must point at its sample in the output.
func TestFragmentEncodeTopLevelBoxes(t *testing.T) {
	frag := mp4.NewFragment()
	frag.AddChild(&mp4.EmsgBox{Version: 1, TimeScale: 90000, SchemeIDURI: "urn:mp4ff:test", Value: "1",
		MessageData: []byte("event")})
	frag.AddChild(mp4.CreatePrftBox(1, 24, 1, 0x1234567890abcdef, 0))
	moof := &mp4.MoofBox{}
	frag.AddChild(moof)
	if err := moof.AddChild(mp4.CreateMfhd(1)); err != nil {
		t.Fatal(err)
	}
	for _, trackID := range []uint32{1, 2} {
		traf := &mp4.TrafBox{}
		if err := moof.AddChild(traf); err != nil {
			t.Fatal(err)
		}
		if err := traf.AddChild(mp4.CreateTfhd(trackID)); err != nil {
			t.Fatal(err)
		}
		if err := traf.AddChild(&mp4.TfdtBox{}); err != nil {
			t.Fatal(err)
		}
	}
	frag.AddChild(&mp4.MdatBox{})
	samples := fullSamplesFixture("scattered", 12, 0)
	for i, s := range samples {
		if err := frag.AddFullSampleToTrack(s, uint32(1+i%2)); err != nil {
			t.Fatal(err)
		}
	}

	got := encodeFragment(t, frag)
	if want := encodeFragmentSW(t, frag); !bytes.Equal(got, want) {
		t.Fatal("Encode and EncodeSW differ")
	}
	decoded, err := mp4.DecodeFileSR(bits.NewFixedSliceReader(got))
	if err != nil {
		t.Fatal(err)
	}
	dFrag := decoded.Segments[0].Fragments[0]
	if len(dFrag.Emsgs) != 1 || len(dFrag.Prfts) != 1 {
		t.Fatalf("decoded %d emsg and %d prft boxes, expected 1 of each", len(dFrag.Emsgs), len(dFrag.Prfts))
	}
	moofStart := dFrag.Moof.StartPos
	nrTruns := 0
	for tr, traf := range dFrag.Moof.Trafs {
		for j, trun := range traf.Truns {
			nrTruns++
			s := samples[2*j+tr]
			start := moofStart + uint64(trun.DataOffset)
			if !bytes.Equal(got[start:start+uint64(len(s.Data))], s.Data) {
				t.Errorf("track %d trun %d: data offset %d does not point at its sample", tr+1, j+1, trun.DataOffset)
			}
		}
	}
	if nrTruns != len(samples) {
		t.Errorf("got %d truns, expected %d", nrTruns, len(samples))
	}
}

// TestFragmentReset - a fragment that is reset and built again encodes as a new fragment from CreateFragment does,
// whatever was added to it before: sample data as parts and as a tail, emsg and prft boxes, the boxes that
// encryption adds, and an optimized trun.
func TestFragmentReset(t *testing.T) {
	key, _ := hex.DecodeString("00112233445566778899aabbccddeeff")
	iv, _ := hex.DecodeString("ffeeddccbbaa99887766554433221100")
	kid, _ := mp4.NewUUIDFromString("11112222333344445555666677778888")
	init, err := mp4.ReadMP4File("testdata/init.mp4")
	if err != nil {
		t.Fatal(err)
	}
	ipd, err := mp4.InitProtect(init.Init, key, iv, "cbcs", kid, nil)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("testdata/1.m4s")
	if err != nil {
		t.Fatal(err)
	}
	seg, err := mp4.DecodeFile(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	inFrag := seg.Segments[0].Fragments[0]
	samples, err := inFrag.GetFullSamples(ipd.Trex)
	if err != nil {
		t.Fatal(err)
	}
	trackID := inFrag.Moof.Traf.Tfhd.TrackID
	half := len(samples) / 2
	// build adds the first half of the samples as data parts and the rest as copies, and encodes the fragment.
	build := func(f *mp4.Fragment, ss []mp4.FullSample) []byte {
		f.AddFullSamples(ss[:half])
		for _, s := range ss[half:] {
			f.AddFullSample(s)
		}
		var buf bytes.Buffer
		if err := f.Encode(&buf); err != nil {
			t.Fatal(err)
		}
		return buf.Bytes()
	}
	fresh, err := mp4.CreateFragment(2, trackID)
	if err != nil {
		t.Fatal(err)
	}
	fresh.EncOptimize = mp4.OptimizeTrun // Reset keeps the setting
	want := build(fresh, samples)

	f, err := mp4.CreateFragment(1, trackID)
	if err != nil {
		t.Fatal(err)
	}
	clones := make([]mp4.FullSample, len(samples)) // encryption is in place
	for i, s := range samples {
		clones[i] = s
		clones[i].Data = bytes.Clone(s.Data)
	}
	f.AddEmsg(&mp4.EmsgBox{Version: 1, TimeScale: 1000, SchemeIDURI: "urn:mp4ff:test", Value: "1"})
	f.AddChild(mp4.CreatePrftBox(1, 24, trackID, 0, 0))
	f.EncOptimize = mp4.OptimizeTrun
	_ = build(f, clones)
	if _, err := mp4.EncryptFragment(f, key, iv, ipd); err != nil {
		t.Fatal(err)
	}
	if err := f.Encode(io.Discard); err != nil {
		t.Fatal(err)
	}

	if err := f.Reset(2); err != nil {
		t.Fatal(err)
	}
	if f.EncOptimize != mp4.OptimizeTrun {
		t.Errorf("Reset changed EncOptimize to %d", f.EncOptimize)
	}
	if got := build(f, samples); !bytes.Equal(got, want) {
		t.Error("a reset fragment encodes differently from a new one")
	}
}

// TestFragmentResetReleasesData - Reset drops the references to sample data, and does not write later sample data
// into a buffer that the mdat did not allocate.
func TestFragmentResetReleasesData(t *testing.T) {
	sample := func(data ...byte) mp4.FullSample {
		return mp4.FullSample{Sample: mp4.Sample{Dur: 1, Size: uint32(len(data))}, Data: data}
	}
	t.Run("dataParts", func(t *testing.T) {
		f, err := mp4.CreateFragment(1, 1)
		if err != nil {
			t.Fatal(err)
		}
		f.AddFullSamples([]mp4.FullSample{sample(1, 2, 3), sample(4, 5)})
		if err := f.Reset(2); err != nil {
			t.Fatal(err)
		}
		for i, p := range f.Mdat.DataParts[:cap(f.Mdat.DataParts)] {
			if p != nil {
				t.Errorf("data part %d still references %v after Reset", i, p)
			}
		}
	})
	t.Run("setData", func(t *testing.T) {
		f, err := mp4.CreateFragment(1, 1)
		if err != nil {
			t.Fatal(err)
		}
		own := make([]byte, 0, 16)
		f.Mdat.SetData(own)
		f.AddFullSample(sample(1, 2, 3)) // appended in the spare capacity of own
		if err := f.Reset(2); err != nil {
			t.Fatal(err)
		}
		f.AddFullSample(sample(7, 8, 9))
		if got := own[:3]; !bytes.Equal(got, []byte{1, 2, 3}) {
			t.Errorf("sample data added after Reset overwrote the buffer given to SetData: %v", got)
		}
	})
}

// TestFragmentResetErrors - Reset refuses a fragment that is not single-track, and leaves it unchanged.
func TestFragmentResetErrors(t *testing.T) {
	if err := mp4.NewFragment().Reset(1); err == nil {
		t.Error("Reset of an empty fragment gave no error")
	}
	f, err := mp4.CreateMultiTrackFragment(1, []uint32{1, 2})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Reset(2); err == nil {
		t.Error("Reset of a multi-track fragment gave no error")
	}
	if got := f.Moof.Mfhd.SequenceNumber; got != 1 {
		t.Errorf("failed Reset changed the sequence number to %d", got)
	}
}

// TestFragmentResetAllocations - building fragment after fragment in one Fragment with Reset and Encode does not
// allocate, whether the samples are added as data parts with AddFullSamples or copied with AddFullSample, since
// Reset keeps the payload that AddFullSample copies into.
func TestFragmentResetAllocations(t *testing.T) {
	buf := make([]byte, 4*7000)
	samples := make([]mp4.FullSample, 4)
	for i := range samples {
		samples[i] = mp4.FullSample{
			Sample:     mp4.Sample{Flags: mp4.SyncSampleFlags, Dur: 1024, Size: 7000},
			DecodeTime: uint64(i) * 1024,
			Data:       buf[i*7000 : (i+1)*7000],
		}
	}
	adders := map[string]func(f *mp4.Fragment){
		"AddFullSamples": func(f *mp4.Fragment) { f.AddFullSamples(samples) },
		"AddFullSample": func(f *mp4.Fragment) {
			for _, s := range samples {
				f.AddFullSample(s)
			}
		},
	}
	for name, add := range adders {
		t.Run(name, func(t *testing.T) {
			f, err := mp4.CreateFragment(1, 1)
			if err != nil {
				t.Fatal(err)
			}
			add(f) // Grow the storage that Reset keeps
			allocs := testing.AllocsPerRun(100, func() {
				if err := f.Reset(2); err != nil {
					t.Fatal(err)
				}
				add(f)
				if err := f.Encode(io.Discard); err != nil {
					t.Fatal(err)
				}
			})
			if allocs != 0 {
				t.Errorf("Reset, %s and Encode made %.0f allocations, want none", name, allocs)
			}
		})
	}
}

// TestFragmentResetEncryption - fragments built and encrypted one after the other in one Fragment, whose senc,
// saiz and saio boxes Reset keeps for the encryptor, encode as fragments built and encrypted from scratch, and
// cbcs encryption then allocates nothing.
func TestFragmentResetEncryption(t *testing.T) {
	key, _ := hex.DecodeString("00112233445566778899aabbccddeeff")
	kid, _ := mp4.NewUUIDFromString("11112222333344445555666677778888")
	raw, err := os.ReadFile("testdata/1.m4s")
	if err != nil {
		t.Fatal(err)
	}
	for _, scheme := range []string{"cenc", "cbcs"} {
		t.Run(scheme, func(t *testing.T) {
			iv, _ := hex.DecodeString("ffeeddccbbaa99887766554433221100")
			if scheme == "cenc" {
				iv = iv[:8]
			}
			init, err := mp4.ReadMP4File("testdata/init.mp4")
			if err != nil {
				t.Fatal(err)
			}
			ipd, err := mp4.InitProtect(init.Init, key, iv, scheme, kid, nil)
			if err != nil {
				t.Fatal(err)
			}
			seg, err := mp4.DecodeFile(bytes.NewReader(raw))
			if err != nil {
				t.Fatal(err)
			}
			samples, err := seg.Segments[0].Fragments[0].GetFullSamples(ipd.Trex)
			if err != nil {
				t.Fatal(err)
			}
			trackID := seg.Segments[0].Fragments[0].Moof.Traf.Tfhd.TrackID
			// The samples are copied into the fragments, so encryption leaves them unchanged.
			build := func(f *mp4.Fragment, e *mp4.FragmentEncryptor, ss []mp4.FullSample, w io.Writer) {
				for _, s := range ss {
					f.AddFullSample(s)
				}
				if err := e.EncryptFragment(f); err != nil {
					t.Fatal(err)
				}
				if err := f.Encode(w); err != nil {
					t.Fatal(err)
				}
			}
			freshEnc, err := ipd.NewFragmentEncryptor(key, iv)
			if err != nil {
				t.Fatal(err)
			}
			resetEnc, err := ipd.NewFragmentEncryptor(key, iv)
			if err != nil {
				t.Fatal(err)
			}
			f, err := mp4.CreateFragment(1, trackID)
			if err != nil {
				t.Fatal(err)
			}
			// Fragments of different sizes, so that the kept storage both grows and shrinks.
			seqNr := uint32(1)
			for start, n := 0, 7; start < len(samples); start, n = start+n, 3+(n+5)%11 {
				ss := samples[start:min(start+n, len(samples))]
				fresh, err := mp4.CreateFragment(seqNr, trackID)
				if err != nil {
					t.Fatal(err)
				}
				var want, got bytes.Buffer
				build(fresh, freshEnc, ss, &want)
				if seqNr > 1 {
					if err := f.Reset(seqNr); err != nil {
						t.Fatal(err)
					}
				}
				if build(f, resetEnc, ss, &got); !bytes.Equal(got.Bytes(), want.Bytes()) {
					t.Fatalf("fragment %d: a reset fragment encrypts differently from a new one", seqNr)
				}
				seqNr++
			}
			if scheme != "cbcs" {
				return // cenc makes a CTR stream per sample in crypto/cipher
			}
			ss := samples[:10]
			allocs := testing.AllocsPerRun(20, func() {
				if err := f.Reset(seqNr); err != nil {
					t.Fatal(err)
				}
				build(f, resetEnc, ss, io.Discard)
			})
			if allocs != 0 {
				t.Errorf("Reset, AddFullSample, cbcs encryption and Encode made %.0f allocations, want none", allocs)
			}
		})
	}
}
