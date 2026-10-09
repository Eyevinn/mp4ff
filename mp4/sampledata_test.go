package mp4_test

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
	"github.com/go-test/deep"
)

// readAtFunc is an io.ReaderAt that calls the function.
type readAtFunc func(p []byte, off int64) (int, error)

func (f readAtFunc) ReadAt(p []byte, off int64) (int, error) { return f(p, off) }

// resolverFunc is a DataResolver that calls the function.
type resolverFunc func(entry mp4.Box) (io.ReaderAt, error)

func (f resolverFunc) ResolveDataEntry(entry mp4.Box) (io.ReaderAt, error) { return f(entry) }

// recordingResolver resolves through a LocalFileResolver and records the ReadAt calls on the result.
type recordingResolver struct {
	res        *mp4.LocalFileResolver
	nrResolved int
	reads      []mp4.DataRange
}

func (r *recordingResolver) ResolveDataEntry(entry mp4.Box) (io.ReaderAt, error) {
	r.nrResolved++
	ra, err := r.res.ResolveDataEntry(entry)
	if err != nil {
		return nil, err
	}
	return readAtFunc(func(p []byte, off int64) (int, error) {
		r.reads = append(r.reads, mp4.DataRange{Offset: uint64(off), Size: uint64(len(p))})
		return ra.ReadAt(p, off)
	}), nil
}

func newTestResolver(t *testing.T) *mp4.LocalFileResolver {
	t.Helper()
	res := mp4.NewLocalFileResolver("testdata")
	t.Cleanup(func() {
		if err := res.Close(); err != nil {
			t.Error(err)
		}
	})
	return res
}

func decodeTestFile(t *testing.T, name string, options ...mp4.Option) (*mp4.File, *os.File) {
	t.Helper()
	fd, err := os.Open(name)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { fd.Close() })
	f, err := mp4.DecodeFile(fd, options...)
	if err != nil {
		t.Fatal(err)
	}
	return f, fd
}

// TestReadSampleData checks that the samples of prog_8s_dref.mp4, read from prog_8s.mp4 through
// a LocalFileResolver, are those of prog_8s.mp4 read from its mdat, in memory and lazily, and that
// the latter are what CopySampleData copies.
func TestReadSampleData(t *testing.T) {
	inMem, _ := decodeTestFile(t, "testdata/prog_8s.mp4")
	lazy, lazyFd := decodeTestFile(t, "testdata/prog_8s.mp4", mp4.WithDecodeMode(mp4.DecModeLazyMdat))
	dref, _ := decodeTestFile(t, "testdata/prog_8s_dref.mp4")
	res := newTestResolver(t)
	for trakIdx, trak := range inMem.Moov.Traks {
		nrSamples := trak.GetNrSamples()
		for _, interval := range [][2]uint32{{1, nrSamples}, {1, 1}, {nrSamples, nrSamples}, {3, 77}, {100, 101}} {
			start, end := interval[0], interval[1]
			want, err := inMem.ReadSampleData(trak, start, end, nil, nil)
			if err != nil {
				t.Fatalf("track %d samples %d-%d in memory: %v", trakIdx+1, start, end, err)
			}
			var copied bytes.Buffer
			if err := inMem.CopySampleData(&copied, nil, trak, start, end, nil); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(want, copied.Bytes()) {
				t.Errorf("track %d samples %d-%d: ReadSampleData differs from CopySampleData", trakIdx+1, start, end)
			}
			got, err := lazy.ReadSampleData(lazy.Moov.Traks[trakIdx], start, end, lazyFd, nil)
			if err != nil {
				t.Fatalf("track %d samples %d-%d lazy: %v", trakIdx+1, start, end, err)
			}
			if !bytes.Equal(want, got) {
				t.Errorf("track %d samples %d-%d: lazy data differs", trakIdx+1, start, end)
			}
			got, err = dref.ReadSampleData(dref.Moov.Traks[trakIdx], start, end, nil, res)
			if err != nil {
				t.Fatalf("track %d samples %d-%d via dref: %v", trakIdx+1, start, end, err)
			}
			if !bytes.Equal(want, got) {
				t.Errorf("track %d samples %d-%d: dref data differs", trakIdx+1, start, end)
			}
		}
	}
}

// TestReadSampleDataMergesAdjacentChunks splits the video track of prog_8s_dref.mp4 into one chunk
// per sample and checks that adjacent chunks are read with one ReadAt call.
func TestReadSampleDataMergesAdjacentChunks(t *testing.T) {
	f, _ := decodeTestFile(t, "testdata/prog_8s_dref.mp4")
	res := newTestResolver(t)
	trak := f.Moov.Traks[1]
	stbl := trak.Mdia.Minf.Stbl
	nrSamples := trak.GetNrSamples()
	want, err := f.ReadSampleData(trak, 1, nrSamples, nil, res)
	if err != nil {
		t.Fatal(err)
	}
	stco := &mp4.StcoBox{}
	var wantReads []mp4.DataRange
	for nr := uint32(1); nr <= nrSamples; nr++ {
		ranges, err := trak.GetRangesForSampleInterval(nr, nr)
		if err != nil {
			t.Fatal(err)
		}
		r := ranges[0]
		stco.ChunkOffset = append(stco.ChunkOffset, uint32(r.Offset))
		if n := len(wantReads); n > 0 && wantReads[n-1].Offset+wantReads[n-1].Size == r.Offset {
			wantReads[n-1].Size += r.Size
		} else {
			wantReads = append(wantReads, r)
		}
	}
	if len(wantReads) >= int(nrSamples) {
		t.Fatalf("no adjacent samples among %d", nrSamples)
	}
	stsc := &mp4.StscBox{}
	if err := stsc.AddEntry(1, 1, 1); err != nil {
		t.Fatal(err)
	}
	stbl.Stsc, stbl.Stco = stsc, stco

	rr := &recordingResolver{res: res}
	got, err := f.ReadSampleData(trak, 1, nrSamples, nil, rr)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(want, got) {
		t.Error("data differs after splitting into one chunk per sample")
	}
	if diff := deep.Equal(rr.reads, wantReads); diff != nil {
		t.Errorf("reads: %v", diff)
	}
	if rr.nrResolved != 1 {
		t.Errorf("resolved %d times instead of once", rr.nrResolved)
	}
}

// TestReadSampleDataMixedDataEntries gives every second stsc entry of the audio track of prog_8s.mp4 a
// second sample description, whose data entry refers to prog_8s.mp4 itself, and checks that only the
// chunks of those entries are read through the resolver, and that the data stays the same.
// The entries span several chunks, so the sample description must be looked up by chunk.
func TestReadSampleDataMixedDataEntries(t *testing.T) {
	f, _ := decodeTestFile(t, "testdata/prog_8s.mp4")
	res := newTestResolver(t)
	trak := f.Moov.Traks[0]
	stbl := trak.Mdia.Minf.Stbl
	nrSamples := trak.GetNrSamples()
	want, err := f.ReadSampleData(trak, 1, nrSamples, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	setDataEntries(trak, mp4.CreateURLBox(), &mp4.URLBox{Location: "prog_8s.mp4"})
	external := *stbl.Stsd.Mp4a
	external.DataReferenceIndex = 2
	stbl.Stsd.AddChild(&external)
	stsc := &mp4.StscBox{}
	var wantReads []mp4.DataRange
	var sdID, prevNrSamples uint32 = 2, 0
	nrChunks := uint32(len(stbl.Stco.ChunkOffset))
	for chunkNr := uint32(1); chunkNr <= nrChunks; chunkNr++ {
		chunk, err := stbl.Stsc.GetChunk(chunkNr)
		if err != nil {
			t.Fatal(err)
		}
		if chunk.NrSamples != prevNrSamples || chunkNr%5 == 0 {
			sdID = 3 - sdID
			if err := stsc.AddEntry(chunkNr, chunk.NrSamples, sdID); err != nil {
				t.Fatal(err)
			}
			prevNrSamples = chunk.NrSamples
		}
		if sdID == 2 {
			ranges, err := trak.GetRangesForSampleInterval(chunk.StartSampleNr, chunk.StartSampleNr+chunk.NrSamples-1)
			if err != nil {
				t.Fatal(err)
			}
			wantReads = append(wantReads, ranges[0])
		}
	}
	if len(stsc.Entries) < 3 || len(stsc.Entries) >= int(nrChunks) {
		t.Fatalf("%d stsc entries for %d chunks", len(stsc.Entries), nrChunks)
	}
	stbl.Stsc = stsc

	if err := trak.CheckDataIsSelfContained(); err == nil ||
		!strings.Contains(err.Error(), `sample description 2 (mp4a) has its data outside this file`) {
		t.Errorf("CheckDataIsSelfContained: got %v", err)
	}
	rr := &recordingResolver{res: res}
	got, err := f.ReadSampleData(trak, 1, nrSamples, nil, rr)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(want, got) {
		t.Error("data differs with every second chunk read through the resolver")
	}
	if diff := deep.Equal(rr.reads, wantReads); diff != nil {
		t.Errorf("reads: %v", diff)
	}
	if rr.nrResolved != 1 {
		t.Errorf("resolved %d times instead of once", rr.nrResolved)
	}
}

func TestReadSampleDataErrors(t *testing.T) {
	inMem, _ := decodeTestFile(t, "testdata/prog_8s.mp4")
	lazy, _ := decodeTestFile(t, "testdata/prog_8s.mp4", mp4.WithDecodeMode(mp4.DecModeLazyMdat))
	dref, _ := decodeTestFile(t, "testdata/prog_8s_dref.mp4")
	fragmented, _ := decodeTestFile(t, "testdata/1.m4s")
	errBoom := errors.New("boom")
	// Closed before TempDir removes the directory, which Windows refuses while the resolver has it open.
	emptyDirResolver := mp4.NewLocalFileResolver(t.TempDir())
	t.Cleanup(func() { emptyDirResolver.Close() })
	cases := []struct {
		desc       string
		f          *mp4.File
		trak       *mp4.TrakBox
		start, end uint32
		resolver   mp4.DataResolver
		wantErr    string
		wantErrIs  error
	}{
		{desc: "fragmented file", f: fragmented, trak: &mp4.TrakBox{}, start: 1, end: 1,
			wantErr: "only available for progressive files"},
		{desc: "incomplete trak", f: inMem, trak: &mp4.TrakBox{}, start: 1, end: 1,
			wantErr: "complete mdia/minf/stbl"},
		{desc: "sample 0", f: inMem, trak: inMem.Moov.Traks[0], start: 0, end: 1, wantErr: "sample interval 0-1"},
		{desc: "after last sample", f: inMem, trak: inMem.Moov.Traks[0], start: 1, end: 376,
			wantErr: "sample interval 1-376"},
		{desc: "ends before start", f: inMem, trak: inMem.Moov.Traks[0], start: 3, end: 2, wantErr: "bad sample interval"},
		{desc: "lazy without ReadSeeker", f: lazy, trak: lazy.Moov.Traks[0], start: 1, end: 1,
			wantErr: "no ReadSeeker for lazy mdat"},
		{desc: "no resolver", f: dref, trak: dref.Moov.Traks[0], start: 1, end: 1,
			wantErr: `sample data in url "prog_8s.mp4" outside this file, but no DataResolver`},
		{desc: "resolver error", f: dref, trak: dref.Moov.Traks[0], start: 1, end: 1,
			resolver:  resolverFunc(func(mp4.Box) (io.ReaderAt, error) { return nil, errBoom }),
			wantErr:   `resolve url "prog_8s.mp4"`,
			wantErrIs: errBoom},
		{desc: "missing file", f: dref, trak: dref.Moov.Traks[0], start: 1, end: 1,
			resolver: emptyDirResolver, wantErrIs: fs.ErrNotExist},
		{desc: "data too short", f: dref, trak: dref.Moov.Traks[0], start: 1, end: 10,
			resolver: resolverFunc(func(mp4.Box) (io.ReaderAt, error) {
				return bytes.NewReader(make([]byte, 1000)), nil
			}),
			wantErrIs: io.ErrUnexpectedEOF},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			_, err := c.f.ReadSampleData(c.trak, c.start, c.end, nil, c.resolver)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if c.wantErr != "" && !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("got error %q, want it to contain %q", err, c.wantErr)
			}
			if c.wantErrIs != nil && !errors.Is(err, c.wantErrIs) {
				t.Errorf("got error %q, want it to wrap %q", err, c.wantErrIs)
			}
		})
	}
}
