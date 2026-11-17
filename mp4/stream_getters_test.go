package mp4_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"testing"
	"testing/iotest"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

type sampleReadCounter struct {
	*os.File
	reads int
}

func (r *sampleReadCounter) Read(p []byte) (int, error) {
	r.reads++
	return r.File.Read(p)
}

func TestStreamGettersReadRuns(t *testing.T) {
	const path = "testdata/v300_multiple_segments.mp4"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	file, err := mp4.DecodeFileSR(bits.NewFixedSliceReader(data))
	if err != nil {
		t.Fatal(err)
	}
	var fragments []*mp4.Fragment
	for _, seg := range file.Segments {
		fragments = append(fragments, seg.Fragments...)
	}
	for _, mode := range []string{"GetSamples", "GetSampleRange"} {
		t.Run(mode, func(t *testing.T) {
			f, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			r := &sampleReadCounter{File: f}
			index := 0
			var kept, expected []mp4.FullSample
			sf, err := mp4.InitDecodeStream(r, mp4.WithFragmentCallback(func(frag *mp4.Fragment, sa mp4.SampleAccessor) error {
				for _, traf := range frag.Moof.Trafs {
					id := traf.Tfhd.TrackID
					want, err := fragments[index].GetFullSamples(&mp4.TrexBox{TrackID: id})
					if err != nil {
						return err
					}
					before := r.reads
					var got []mp4.FullSample
					if mode == "GetSamples" {
						got, err = sa.GetSamples(id)
					} else {
						got, err = sa.GetSampleRange(id, 1, ^uint32(0))
					}
					if err != nil {
						return err
					}
					compareFullSamples(t, got, want)
					kept = append(kept, got...)
					expected = append(expected, want...)
					if reads := r.reads - before; reads > len(traf.Truns) {
						t.Errorf("got %d sample reads, want at most %d, one per trun", reads, len(traf.Truns))
					}
				}
				index++
				return nil
			}))
			if err != nil {
				t.Fatal(err)
			}
			if err := sf.ProcessFragments(); err != nil {
				t.Fatal(err)
			}
			compareFullSamples(t, kept, expected)
			if index != len(fragments) {
				t.Fatalf("got %d fragments, want %d", index, len(fragments))
			}
		})
	}
}

func TestStreamGetterReadErrors(t *testing.T) {
	frag, err := mp4.CreateFragment(1, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, size := range []int{3, 2, 4, 1} {
		frag.AddFullSample(mp4.FullSample{
			Sample: mp4.Sample{Dur: 10, Size: uint32(size)}, Data: make([]byte, size),
		})
	}
	var buf bytes.Buffer
	if err := frag.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	data := buf.Bytes()
	payload := bytes.Index(data, []byte("mdat")) + 4
	wrappedEOF := fmt.Errorf("wrapped: %w", io.EOF)
	for _, mode := range []string{"GetSamples", "GetSampleRange"} {
		for _, warm := range []bool{false, true} {
			for _, cut := range []int{3, 4} {
				for _, wrapped := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/warm=%t/cut=%d/wrapped=%t", mode, warm, cut, wrapped), func(t *testing.T) {
						r := iotest.DataErrReader(bytes.NewReader(data[:payload+cut]))
						if wrapped {
							r = io.MultiReader(bytes.NewReader(data[:payload+cut]), iotest.ErrReader(wrappedEOF))
						}
						sf, err := mp4.InitDecodeStream(r, mp4.WithFragmentCallback(
							func(f *mp4.Fragment, sa mp4.SampleAccessor) error {
								if warm {
									if _, err := sa.GetSample(1, 1); err != nil {
										t.Fatal(err)
									}
								}
								var got []mp4.FullSample
								var err error
								prefix := "read sample data: "
								if mode == "GetSamples" {
									got, err = sa.GetSamples(1)
								} else {
									got, err = sa.GetSampleRange(1, 1, 4)
									prefix = "read sample 2 data: "
								}
								want := io.EOF
								if cut == 4 {
									want = io.ErrUnexpectedEOF
								} else if wrapped {
									want = wrappedEOF
								}
								if got != nil || err == nil || err.Error() != prefix+want.Error() {
									t.Fatalf("got %v, %v; want nil, %s%v", got, err, prefix, want)
								}
								// A failed longer read leaves the earlier cached sample available.
								one, err := sa.GetSampleRange(1, 1, 1)
								if err != nil || len(one) != 1 || !bytes.Equal(one[0].Data, make([]byte, 3)) {
									t.Fatalf("cached sample after error: %v, %v", one, err)
								}
								return nil
							}))
						if err != nil {
							t.Fatal(err)
						}
						// The payload is truncated; ProcessFragments cannot finish skipping it.
						if err := sf.ProcessFragments(); err == nil {
							t.Fatal("expected truncated stream error")
						}
					})
				}
			}
		}
	}
}

func TestStreamGetterKeepsTrailingBoxesAfterError(t *testing.T) {
	data := append(interleavedSegment(t), 0, 0, 0, 8, 'f', 'r', 'e', 'e')
	for _, mode := range []string{"GetSamples", "GetSampleRange"} {
		t.Run(mode, func(t *testing.T) {
			trailing := 0
			sf, err := mp4.InitDecodeStream(bytes.NewBuffer(data),
				mp4.WithFragmentCallback(func(f *mp4.Fragment, sa mp4.SampleAccessor) error {
					trun := f.Moof.Traf.Truns[len(f.Moof.Traf.Truns)-1]
					trun.Samples = append(trun.Samples, mp4.Sample{}, mp4.Sample{})
					last := &trun.Samples[len(trun.Samples)-3]
					size := last.Size
					mdatSize := f.Mdat.GetLazyDataSize()
					last.Size += 8
					f.Mdat.SetLazyDataSize(mdatSize + 8)
					var got []mp4.FullSample
					var err error
					if mode == "GetSamples" {
						got, err = sa.GetSamples(1)
					} else {
						got, err = sa.GetSampleRange(1, 1, ^uint32(0))
					}
					if err == nil || got != nil {
						t.Fatalf("expanded sample: got %v, %v; want nil and error", got, err)
					}
					last.Size = size
					f.Mdat.SetLazyDataSize(mdatSize)
					trun.Samples = trun.Samples[:len(trun.Samples)-2]
					if _, err := sa.GetSampleRange(1, 1, 1); err != nil {
						return err
					}
					return nil
				}), mp4.WithTrailingBoxesCallback(func(f *mp4.Fragment, boxes []mp4.Box) error {
					for _, box := range boxes {
						if box.Type() != "free" {
							t.Fatalf("trailing box %s, want free", box.Type())
						}
						trailing++
					}
					return nil
				}))
			if err != nil {
				t.Fatal(err)
			}
			if err := sf.ProcessFragments(); err != nil {
				t.Fatal(err)
			}
			if trailing != 1 {
				t.Fatalf("got %d trailing boxes, want 1", trailing)
			}
		})
	}
}

// sampleErrorReader reports an error once at a byte position, even when that
// position is the end of the caller's read buffer.
type sampleErrorReader struct {
	io.Reader
	remaining int
	err       error
}

func (r *sampleErrorReader) Read(p []byte) (int, error) {
	if r.err == nil {
		return r.Reader.Read(p)
	}
	n, err := r.Reader.Read(p[:min(len(p), r.remaining)])
	r.remaining -= n
	if r.remaining == 0 {
		err, r.err = r.err, nil
	}
	return n, err
}

func TestStreamGetterRetriesAfterReadError(t *testing.T) {
	sizes := []int{3, 4, 5, 6}
	for _, mode := range []string{"GetSamples", "GetSampleRange"} {
		t.Run(mode, func(t *testing.T) {
			frag, err := mp4.CreateFragment(1, 1)
			if err != nil {
				t.Fatal(err)
			}
			var want []mp4.FullSample
			for i, size := range sizes {
				s := mp4.FullSample{Sample: mp4.Sample{Dur: 10, Size: uint32(size)},
					DecodeTime: uint64(i * 10), Data: bytes.Repeat([]byte{byte(i + 1)}, size)}
				frag.AddFullSample(s)
				want = append(want, s)
			}
			var buf bytes.Buffer
			if err := frag.Encode(&buf); err != nil {
				t.Fatal(err)
			}
			r := &sampleErrorReader{Reader: bytes.NewReader(buf.Bytes())}
			sf, err := mp4.InitDecodeStream(r, mp4.WithFragmentCallback(func(f *mp4.Fragment, sa mp4.SampleAccessor) error {
				// The reader fails once in the middle of sample 2, and then recovers.
				r.remaining, r.err = sizes[0]+2, fmt.Errorf("temporary read error")
				var got []mp4.FullSample
				var err error
				if mode == "GetSamples" {
					got, err = sa.GetSamples(1)
				} else {
					got, err = sa.GetSampleRange(1, 1, ^uint32(0))
				}
				if err != nil {
					t.Fatal(err)
				}
				compareFullSamples(t, got, want)
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

func TestStreamGetterChunkErrors(t *testing.T) {
	const chunk = 1 << 20
	for _, sizes := range [][]int{{chunk + 1, 2, 2, 2}, {3, chunk + 1, 2, 2, 2}} {
		for _, mode := range []string{"GetSamples", "GetSampleRange", "PartialRange"} {
			if mode == "PartialRange" && len(sizes) == 4 {
				continue
			}
			t.Run(fmt.Sprintf("%v/%s", sizes, mode), func(t *testing.T) {
				frag, err := mp4.CreateFragment(1, 1)
				if err != nil {
					t.Fatal(err)
				}
				var want []mp4.FullSample
				for i, size := range sizes {
					s := mp4.FullSample{Sample: mp4.Sample{Dur: 10, Size: uint32(size)},
						DecodeTime: uint64(i * 10), Data: bytes.Repeat([]byte{byte(i + 1)}, size)}
					frag.AddFullSample(s)
					want = append(want, s)
				}
				var buf bytes.Buffer
				if err := frag.Encode(&buf); err != nil {
					t.Fatal(err)
				}
				r := &sampleErrorReader{Reader: bytes.NewReader(buf.Bytes())}
				sf, err := mp4.InitDecodeStream(r, mp4.WithFragmentCallback(func(f *mp4.Fragment, sa mp4.SampleAccessor) error {
					r.remaining, r.err = chunk, fmt.Errorf("temporary read error")
					if len(sizes) == 5 {
						r.remaining += sizes[0]
					}
					var got []mp4.FullSample
					var err error
					start := uint32(1)
					if mode == "GetSamples" {
						got, err = sa.GetSamples(1)
					} else {
						if mode == "PartialRange" {
							start = 2
						}
						got, err = sa.GetSampleRange(1, start, ^uint32(0))
					}
					if err != nil {
						t.Fatal(err)
					}
					compareFullSamples(t, got, want[start-1:])
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
}

func TestStreamGetterOversizedSample(t *testing.T) {
	const size = uint32(1 << 31)
	for _, mode := range []string{"GetSamples", "GetSampleRange", "PartialRange"} {
		for _, preceding := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/preceding=%t", mode, preceding), func(t *testing.T) {
				frag, err := mp4.CreateFragment(1, 1)
				if err != nil {
					t.Fatal(err)
				}
				start, largeNr := uint32(1), uint32(1)
				if mode == "PartialRange" {
					start++
					largeNr++
				}
				if preceding {
					largeNr++
				}
				for i := uint32(1); i < largeNr; i++ {
					frag.AddFullSample(mp4.FullSample{Sample: mp4.Sample{Size: 1, Dur: 1}})
				}
				for _, n := range []uint32{size, 1, 1, 1} {
					frag.AddFullSample(mp4.FullSample{Sample: mp4.Sample{Size: n, Dur: 1}})
				}
				frag.Mdat.SetLazyDataSize(uint64(size) + uint64(largeNr-1) + 3)
				var buf bytes.Buffer
				if err := frag.Encode(&buf); err != nil {
					t.Fatal(err)
				}
				readErr, stop := errors.New("payload unavailable"), errors.New("callback complete")
				r := io.MultiReader(&buf, bytes.NewReader(make([]byte, largeNr-1)), iotest.ErrReader(readErr))
				sf, err := mp4.InitDecodeStream(r, mp4.WithFragmentCallback(func(f *mp4.Fragment, sa mp4.SampleAccessor) error {
					var got []mp4.FullSample
					var err error
					prefix := "read sample data: "
					if mode == "GetSamples" {
						got, err = sa.GetSamples(1)
					} else {
						got, err = sa.GetSampleRange(1, start, ^uint32(0))
						prefix = fmt.Sprintf("read sample %d data: ", largeNr)
					}
					want := prefix + readErr.Error()
					if uint64(size) > uint64(math.MaxInt) {
						want = fmt.Sprintf("%slazy mdat mode - invalid size %d", prefix, size)
					}
					if got != nil || err == nil || err.Error() != want {
						t.Fatalf("got %v, %v; want nil, %s", got, err, want)
					}
					return stop
				}))
				if err == nil {
					err = sf.ProcessFragments()
				}
				if !errors.Is(err, stop) {
					t.Fatalf("callback: %v", err)
				}
			})
		}
	}
}
