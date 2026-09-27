package mp4_test

import (
	"bytes"
	"fmt"
	"os"
	"runtime"
	"sync"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

// interleavedSegment returns an encoded fragment with two tracks whose samples are interleaved in runs of three,
// so that every track has several truns and the data of a track is not contiguous in the mdat.
func interleavedSegment(t *testing.T) []byte {
	t.Helper()
	frag, err := mp4.CreateMultiTrackFragment(1, []uint32{1, 2})
	if err != nil {
		t.Fatal(err)
	}
	decTimes := map[uint32]uint64{1: 9000, 2: 4800}
	for i := 0; i < 14; i++ {
		trackID := uint32(1 + (i/3)%2)
		data := bytes.Repeat([]byte{byte(16*trackID) + byte(i)}, 100+i)
		s := mp4.FullSample{
			Sample:     mp4.Sample{Flags: mp4.SyncSampleFlags, Dur: 512 * trackID, Size: uint32(len(data))},
			DecodeTime: decTimes[trackID],
			Data:       data,
		}
		if err := frag.AddFullSampleToTrack(s, trackID); err != nil {
			t.Fatal(err)
		}
		decTimes[trackID] += uint64(s.Dur)
	}
	var buf bytes.Buffer
	if err := frag.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func appendTestFiles(t *testing.T) map[string][]byte {
	t.Helper()
	v300, err := os.ReadFile("testdata/v300_multiple_segments.mp4")
	if err != nil {
		t.Fatal(err)
	}
	return map[string][]byte{
		"v300":        v300,
		"interleaved": interleavedSegment(t),
	}
}

func compareFullSamples(t *testing.T, got, want []mp4.FullSample) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d samples, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Sample != want[i].Sample || got[i].DecodeTime != want[i].DecodeTime {
			t.Fatalf("sample %d: got %+v at %d, want %+v at %d", i+1, got[i].Sample, got[i].DecodeTime,
				want[i].Sample, want[i].DecodeTime)
		}
		if !bytes.Equal(got[i].Data, want[i].Data) {
			t.Fatalf("sample %d: data differs", i+1)
		}
	}
}

func TestFragmentAppendFullSamples(t *testing.T) {
	for name, data := range appendTestFiles(t) {
		t.Run(name, func(t *testing.T) {
			f, err := mp4.DecodeFileSR(bits.NewFixedSliceReader(data))
			if err != nil {
				t.Fatal(err)
			}
			var samples []mp4.FullSample
			nrChecked := 0
			for _, seg := range f.Segments {
				for _, frag := range seg.Fragments {
					for _, traf := range frag.Moof.Trafs {
						trex := &mp4.TrexBox{TrackID: traf.Tfhd.TrackID}
						want, err := frag.GetFullSamples(trex)
						if err != nil {
							t.Fatal(err)
						}
						samples, err = frag.AppendFullSamples(samples[:0], trex)
						if err != nil {
							t.Fatal(err)
						}
						compareFullSamples(t, samples, want)
						for i := range samples {
							if !sliceWithin(samples[i].Data, data) {
								t.Fatalf("sample %d data is not a view into the input", i+1)
							}
						}
						allocs := testing.AllocsPerRun(10, func() {
							samples, _ = frag.AppendFullSamples(samples[:0], trex)
						})
						if allocs != 0 {
							t.Errorf("reusing dst: got %.0f allocations, want 0", allocs)
						}
						nrChecked++
					}
				}
			}
			if nrChecked == 0 {
				t.Fatal("no fragments checked")
			}
		})
	}
}

func TestFragmentAppendFullSamplesKeepsPrefix(t *testing.T) {
	f, err := mp4.DecodeFileSR(bits.NewFixedSliceReader(interleavedSegment(t)))
	if err != nil {
		t.Fatal(err)
	}
	frag := f.Segments[0].Fragments[0]
	s1, err := frag.AppendFullSamples(nil, &mp4.TrexBox{TrackID: 1})
	if err != nil {
		t.Fatal(err)
	}
	both, err := frag.AppendFullSamples(s1, &mp4.TrexBox{TrackID: 2})
	if err != nil {
		t.Fatal(err)
	}
	s2, _ := frag.GetFullSamples(&mp4.TrexBox{TrackID: 2})
	compareFullSamples(t, both, append(append([]mp4.FullSample{}, s1...), s2...))

	none, err := frag.AppendFullSamples(s1, &mp4.TrexBox{TrackID: 3})
	if err != nil || len(none) != len(s1) {
		t.Errorf("absent track: got %d samples and error %v, want %d and nil", len(none), err, len(s1))
	}

	frag.Mdat.Data = frag.Mdat.Data[:50]
	got, err := frag.AppendFullSamples(s1, &mp4.TrexBox{TrackID: 2})
	if err == nil {
		t.Fatal("expected error for truncated mdat")
	}
	if len(got) != len(s1) {
		t.Errorf("on error: got length %d, want original length %d", len(got), len(s1))
	}
}

func TestStreamAppendSamples(t *testing.T) {
	for name, data := range appendTestFiles(t) {
		t.Run(name, func(t *testing.T) {
			var samples []mp4.FullSample
			var buf []byte
			nrChecked := 0
			sf, err := mp4.InitDecodeStream(bytes.NewReader(data),
				mp4.WithFragmentCallback(func(f *mp4.Fragment, sa mp4.SampleAccessor) error {
					for _, traf := range f.Moof.Trafs {
						trackID := traf.Tfhd.TrackID
						want, err := sa.GetSamples(trackID)
						if err != nil {
							return err
						}
						samples, buf, err = sa.AppendSamples(samples[:0], buf[:0], trackID)
						if err != nil {
							return err
						}
						compareFullSamples(t, samples, want)
						for i := range samples {
							if !sliceWithin(samples[i].Data, buf) {
								t.Fatalf("sample %d data is not a view into buf", i+1)
							}
						}
						allocs := testing.AllocsPerRun(10, func() {
							samples, buf, _ = sa.AppendSamples(samples[:0], buf[:0], trackID)
						})
						if allocs != 0 {
							t.Errorf("reusing dst and buf: got %.0f allocations, want 0", allocs)
						}
						nrChecked++
					}
					return nil
				}))
			if err != nil {
				t.Fatal(err)
			}
			if err := sf.ProcessFragments(); err != nil {
				t.Fatal(err)
			}
			if nrChecked == 0 {
				t.Fatal("no fragments checked")
			}
		})
	}
}

func TestStreamAppendSampleRange(t *testing.T) {
	for name, data := range appendTestFiles(t) {
		t.Run(name, func(t *testing.T) {
			sf, err := mp4.InitDecodeStream(bytes.NewReader(data),
				mp4.WithFragmentCallback(func(f *mp4.Fragment, sa mp4.SampleAccessor) error {
					for _, traf := range f.Moof.Trafs {
						trackID := traf.Tfhd.TrackID
						all, err := sa.GetSamples(trackID)
						if err != nil {
							return err
						}
						n := uint32(len(all))
						// Ranges starting and ending inside, at and across trun boundaries.
						for _, r := range [][2]uint32{{1, 1}, {1, n}, {2, 4}, {3, 4}, {n, n}, {n - 1, n + 5}} {
							want, err := sa.GetSampleRange(trackID, r[0], r[1])
							if err != nil {
								return err
							}
							prefix := []mp4.FullSample{{DecodeTime: 1}}
							got, buf, err := sa.AppendSampleRange(prefix, []byte{0xff}, trackID, r[0], r[1])
							if err != nil {
								t.Fatalf("range %v: %v", r, err)
							}
							if got[0].DecodeTime != 1 || buf[0] != 0xff {
								t.Fatalf("range %v: prefix of dst or buf overwritten", r)
							}
							compareFullSamples(t, got[1:], want)
						}
						for _, r := range [][2]uint32{{0, 1}, {3, 2}, {n + 1, n + 2}} {
							dst, buf := make([]mp4.FullSample, 2), make([]byte, 3)
							gotDst, gotBuf, err := sa.AppendSampleRange(dst, buf, trackID, r[0], r[1])
							if err == nil {
								t.Errorf("range %v: expected error", r)
							}
							if len(gotDst) != 2 || len(gotBuf) != 3 {
								t.Errorf("range %v: on error got lengths %d and %d, want 2 and 3", r,
									len(gotDst), len(gotBuf))
							}
						}
					}
					if _, _, err := sa.AppendSamples(nil, nil, 99); err == nil {
						t.Error("expected error for absent track")
					}
					return nil
				}))
			if err != nil {
				t.Fatal(err)
			}
			if err := sf.ProcessFragments(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// sliceWithin reports whether s is a subslice of the memory of whole.
func sliceWithin(s, whole []byte) bool {
	if len(s) == 0 {
		return true
	}
	whole = whole[:cap(whole)]
	for i := 0; i+len(s) <= len(whole); i++ {
		if &whole[i] == &s[0] {
			return true
		}
	}
	return false
}

// segmentBufPool holds the buffers that segments are read into. The samples returned by AppendFullSamples view
// that buffer, so a buffer may only go back to the pool once the samples extracted from it are done with.
var segmentBufPool = sync.Pool{New: func() any {
	b := make([]byte, 0, 64*1024)
	return &b
}}

// Extract the samples of many segments without allocating for sample data or sample slices once warmed up:
// segments are read into pooled buffers, decoding with DecodeFileSR makes mdat data a view into that buffer,
// and one sample slice is reused for every fragment.
func ExampleFragment_AppendFullSamples() {
	segment, err := os.ReadFile("testdata/1.m4s") // stands in for a segment arriving over the network
	if err != nil {
		panic(err)
	}
	var samples []mp4.FullSample
	var nrSamples, nrBytes int
	for i := 0; i < 3; i++ {
		bp := segmentBufPool.Get().(*[]byte)
		buf := append((*bp)[:0], segment...)
		f, err := mp4.DecodeFileSR(bits.NewFixedSliceReader(buf))
		if err != nil {
			panic(err)
		}
		for _, seg := range f.Segments {
			for _, frag := range seg.Fragments {
				samples, err = frag.AppendFullSamples(samples[:0], nil)
				if err != nil {
					panic(err)
				}
				for _, s := range samples {
					nrSamples++
					nrBytes += len(s.Data)
				}
			}
		}
		*bp = buf // keep the capacity if append had to grow it
		segmentBufPool.Put(bp)
	}
	fmt.Printf("%d samples, %d bytes\n", nrSamples, nrBytes)
	// Output: 180 samples, 73548 bytes
}

func BenchmarkStreamSamples(b *testing.B) {
	data, err := os.ReadFile("testdata/v300_multiple_segments.mp4")
	if err != nil {
		b.Fatal(err)
	}
	run := func(b *testing.B, get func(sa mp4.SampleAccessor, trackID uint32) error) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			sf, err := mp4.InitDecodeStream(bytes.NewReader(data),
				mp4.WithFragmentCallback(func(f *mp4.Fragment, sa mp4.SampleAccessor) error {
					for _, traf := range f.Moof.Trafs {
						if err := get(sa, traf.Tfhd.TrackID); err != nil {
							return err
						}
					}
					return nil
				}))
			if err != nil {
				b.Fatal(err)
			}
			if err := sf.ProcessFragments(); err != nil {
				b.Fatal(err)
			}
		}
	}
	b.Run("GetSamples", func(b *testing.B) {
		run(b, func(sa mp4.SampleAccessor, trackID uint32) error {
			_, err := sa.GetSamples(trackID)
			return err
		})
	})
	var samples []mp4.FullSample
	var buf []byte
	b.Run("AppendSamples", func(b *testing.B) {
		run(b, func(sa mp4.SampleAccessor, trackID uint32) (err error) {
			samples, buf, err = sa.AppendSamples(samples[:0], buf[:0], trackID)
			return err
		})
	})
}

func BenchmarkFragmentFullSamples(b *testing.B) {
	data, err := os.ReadFile("testdata/1.m4s")
	if err != nil {
		b.Fatal(err)
	}
	f, err := mp4.DecodeFileSR(bits.NewFixedSliceReader(data))
	if err != nil {
		b.Fatal(err)
	}
	frag := f.Segments[0].Fragments[0]
	b.Run("GetFullSamples", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := frag.GetFullSamples(nil); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("AppendFullSamples", func(b *testing.B) {
		b.ReportAllocs()
		var samples []mp4.FullSample
		for i := 0; i < b.N; i++ {
			if samples, err = frag.AppendFullSamples(samples[:0], nil); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("Samples", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			for _, err := range frag.Samples(nil) {
				if err != nil {
					b.Fatal(err)
				}
			}
		}
	})
}

// TestStreamAppendSamplesRefragment checks the zero-copy chain from a stream to a new fragment: the samples that
// AppendSamples reads into one buffer are adjacent there, so AddFullSamples adds them as a single data part that
// views that buffer.
func TestStreamAppendSamplesRefragment(t *testing.T) {
	for name, data := range appendTestFiles(t) {
		t.Run(name, func(t *testing.T) {
			sf, err := mp4.InitDecodeStream(bytes.NewReader(data),
				mp4.WithFragmentCallback(func(f *mp4.Fragment, sa mp4.SampleAccessor) error {
					for _, traf := range f.Moof.Trafs {
						trackID := traf.Tfhd.TrackID
						samples, buf, err := sa.AppendSamples(nil, nil, trackID)
						if err != nil {
							return err
						}
						out, err := mp4.CreateFragment(1, trackID)
						if err != nil {
							return err
						}
						out.AddFullSamples(samples)
						parts := out.Mdat.DataParts
						if len(parts) != 1 || len(parts[0]) != len(buf) || !sliceWithin(parts[0], buf) {
							t.Fatalf("track %d: got %d data parts, want 1 part viewing all of buf", trackID, len(parts))
						}
					}
					return nil
				}))
			if err != nil {
				t.Fatal(err)
			}
			if err := sf.ProcessFragments(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestFragmentSamples(t *testing.T) {
	for name, data := range appendTestFiles(t) {
		t.Run(name, func(t *testing.T) {
			f, err := mp4.DecodeFileSR(bits.NewFixedSliceReader(data))
			if err != nil {
				t.Fatal(err)
			}
			for _, seg := range f.Segments {
				for _, frag := range seg.Fragments {
					for _, traf := range frag.Moof.Trafs {
						trex := &mp4.TrexBox{TrackID: traf.Tfhd.TrackID}
						want, err := frag.GetFullSamples(trex)
						if err != nil {
							t.Fatal(err)
						}
						var got []mp4.FullSample
						for s, err := range frag.Samples(trex) {
							if err != nil {
								t.Fatal(err)
							}
							got = append(got, s)
						}
						compareFullSamples(t, got, want)

						// Collected samples are added to a new fragment as one data part per trun,
						// since only the data of a trun are adjacent in the mdat.
						out, err := mp4.CreateFragment(1, trex.TrackID)
						if err != nil {
							t.Fatal(err)
						}
						out.AddFullSamples(got)
						if len(out.Mdat.DataParts) != len(traf.Truns) {
							t.Errorf("got %d data parts, want %d", len(out.Mdat.DataParts), len(traf.Truns))
						}

						var nrBytes int
						allocs := testing.AllocsPerRun(10, func() {
							for s := range frag.Samples(trex) {
								nrBytes += len(s.Data)
							}
						})
						if allocs != 0 {
							t.Errorf("got %.0f allocations, want 0", allocs)
						}
					}
				}
			}
		})
	}
}

func TestFragmentSamplesEdgeCases(t *testing.T) {
	f, err := mp4.DecodeFileSR(bits.NewFixedSliceReader(interleavedSegment(t)))
	if err != nil {
		t.Fatal(err)
	}
	frag := f.Segments[0].Fragments[0]

	n := 0
	for range frag.Samples(nil) { // the first traf
		n++
		if n == 2 {
			break
		}
	}
	if n != 2 {
		t.Errorf("break: got %d iterations, want 2", n)
	}

	for s, err := range frag.Samples(&mp4.TrexBox{TrackID: 3}) {
		t.Errorf("absent track: got sample %+v and error %v, want nothing", s.Sample, err)
	}

	frag.Mdat.Data = frag.Mdat.Data[:500]
	var nrYields int
	for s, err := range frag.Samples(&mp4.TrexBox{TrackID: 2}) {
		nrYields++
		if err == nil {
			t.Fatalf("truncated mdat: got sample at %d without error", s.DecodeTime)
		}
		if s.Data != nil || s.DecodeTime != 0 {
			t.Errorf("truncated mdat: got non-zero sample with the error")
		}
	}
	if nrYields != 1 {
		t.Errorf("truncated mdat: got %d yields, want a single error", nrYields)
	}
}

func ExampleFragment_Samples() {
	segment, err := os.ReadFile("testdata/1.m4s")
	if err != nil {
		panic(err)
	}
	f, err := mp4.DecodeFileSR(bits.NewFixedSliceReader(segment))
	if err != nil {
		panic(err)
	}
	var nrSync, nrBytes int
	for _, seg := range f.Segments {
		for _, frag := range seg.Fragments {
			for s, err := range frag.Samples(nil) {
				if err != nil {
					panic(err)
				}
				if s.IsSync() {
					nrSync++
				}
				nrBytes += len(s.Data)
			}
		}
	}
	fmt.Printf("%d sync samples, %d bytes\n", nrSync, nrBytes)
	// Output: 2 sync samples, 24516 bytes
}

// hugeSizesStream returns a 170-byte fragment whose trun claims three samples of 1 GiB each, while its mdat holds
// only 30 bytes.
func hugeSizesStream(t *testing.T) []byte {
	t.Helper()
	frag, err := mp4.CreateFragment(1, 1)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		frag.AddFullSample(mp4.FullSample{
			Sample: mp4.Sample{Flags: mp4.SyncSampleFlags, Dur: 512, Size: 1 << 30},
			Data:   bytes.Repeat([]byte{byte(i)}, 10),
		})
	}
	var buf bytes.Buffer
	if err := frag.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestStreamSampleSizesBoundedByMdat checks that trun sample sizes reaching beyond the mdat give an error without
// first allocating buffers of the claimed size.
func TestStreamSampleSizesBoundedByMdat(t *testing.T) {
	calls := map[string]func(sa mp4.SampleAccessor) error{
		"AppendSamples": func(sa mp4.SampleAccessor) error {
			_, _, err := sa.AppendSamples(nil, nil, 1)
			return err
		},
		"AppendSampleRange": func(sa mp4.SampleAccessor) error {
			_, _, err := sa.AppendSampleRange(nil, nil, 1, 1, 3)
			return err
		},
	}
	data := hugeSizesStream(t)
	for name, call := range calls {
		t.Run(name, func(t *testing.T) {
			var callErr error
			var allocated uint64
			sf, err := mp4.InitDecodeStream(bytes.NewReader(data),
				mp4.WithFragmentCallback(func(f *mp4.Fragment, sa mp4.SampleAccessor) error {
					var before, after runtime.MemStats
					runtime.ReadMemStats(&before)
					callErr = call(sa)
					runtime.ReadMemStats(&after)
					allocated = after.TotalAlloc - before.TotalAlloc
					return nil
				}))
			if err != nil {
				t.Fatal(err)
			}
			if err := sf.ProcessFragments(); err != nil {
				t.Fatal(err)
			}
			if callErr == nil {
				t.Error("expected an error for sample data beyond the mdat")
			}
			if allocated > 1<<20 {
				t.Errorf("allocated %d bytes for a %d-byte stream", allocated, len(data))
			}
		})
	}
}
