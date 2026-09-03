package mp4_test

import (
	"bytes"
	"encoding/binary"
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

	// Monolithic data already present: the samples are copied in and the
	// mdat stays monolithic, with the existing baseMediaDecodeTime kept.
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
	if len(frag.Mdat.DataParts) != 0 {
		t.Error("expected monolithic mdat after AddFullSamples onto existing data")
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
