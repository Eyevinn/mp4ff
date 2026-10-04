package mp4_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"testing"
	"testing/iotest"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

// decodeAllFragments decodes data fragment by fragment with one FragmentDecoder and calls fn for each.
func decodeAllFragments(t testing.TB, d *mp4.FragmentDecoder, data []byte, fn func(i int, f *mp4.Fragment)) {
	t.Helper()
	sr := bits.NewFixedSliceReader(data)
	for i := 0; ; i++ {
		f, err := d.DecodeSR(sr, uint64(sr.GetPos()))
		if errors.Is(err, io.EOF) {
			return
		}
		if err != nil {
			t.Fatalf("fragment %d: %v", i+1, err)
		}
		fn(i, f)
	}
}

func TestFragmentDecoderMatchesDecodeFileSR(t *testing.T) {
	for name, data := range fragmentFiles(t) {
		t.Run(name, func(t *testing.T) {
			ref, err := mp4.DecodeFileSR(bits.NewFixedSliceReader(data))
			if err != nil {
				t.Fatal(err)
			}
			var want []*mp4.Fragment
			for _, seg := range ref.Segments {
				want = append(want, seg.Fragments...)
			}
			var d mp4.FragmentDecoder
			n := 0
			decodeAllFragments(t, &d, data, func(i int, f *mp4.Fragment) {
				if i >= len(want) {
					t.Fatalf("decoded more fragments than DecodeFileSR (%d)", len(want))
				}
				w := want[i]
				if f.StartPos != w.StartPos || f.Moof.Mfhd.SequenceNumber != w.Moof.Mfhd.SequenceNumber {
					t.Fatalf("fragment %d: got start %d seq %d, want %d %d", i+1, f.StartPos,
						f.Moof.Mfhd.SequenceNumber, w.StartPos, w.Moof.Mfhd.SequenceNumber)
				}
				if len(f.Moof.Trafs) != len(w.Moof.Trafs) || len(f.Emsgs) != len(w.Emsgs) || len(f.Prfts) != len(w.Prfts) {
					t.Fatalf("fragment %d: different boxes", i+1)
				}
				for _, traf := range w.Moof.Trafs {
					trex := &mp4.TrexBox{TrackID: traf.Tfhd.TrackID}
					ws, err := w.GetFullSamples(trex)
					if err != nil {
						t.Fatal(err)
					}
					gs, err := f.GetFullSamples(trex)
					if err != nil {
						t.Fatal(err)
					}
					compareFullSamples(t, gs, ws)
				}
				var gotEnc, wantEnc bytes.Buffer
				if err := f.Encode(&gotEnc); err != nil {
					t.Fatal(err)
				}
				if err := w.Encode(&wantEnc); err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(gotEnc.Bytes(), wantEnc.Bytes()) {
					t.Fatalf("fragment %d encodes differently", i+1)
				}
				n++
			})
			if n != len(want) {
				t.Errorf("decoded %d fragments, want %d", n, len(want))
			}
		})
	}
}

// TestFragmentDecoderAllocations - once the decoder has seen the layout, decoding fragments of the same layout and
// iterating over their samples allocates nothing.
func TestFragmentDecoderAllocations(t *testing.T) {
	data := oneSampleFragments(t)
	var d mp4.FragmentDecoder
	sr := bits.NewFixedSliceReader(data)
	decodeAll := func() {
		sr.SetPos(0)
		for {
			f, err := d.DecodeSR(sr, uint64(sr.GetPos()))
			if errors.Is(err, io.EOF) {
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			for _, err := range f.Samples(nil) {
				if err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	decodeAll() // the first fragment allocates the boxes
	if allocs := testing.AllocsPerRun(10, decodeAll); allocs != 0 {
		t.Errorf("decoding %d bytes of fragments made %.0f allocations, want none", len(data), allocs)
	}
}

// TestFragmentDecoderTruncated - a fragment that is cut off, inside a box header, between boxes or inside the mdat,
// gives io.ErrUnexpectedEOF, as does a box after the last mdat.
func TestFragmentDecoderTruncated(t *testing.T) {
	first, err := mp4.ReadFragmentBytes(bytes.NewReader(oneSampleFragments(t)), nil)
	if err != nil {
		t.Fatal(err)
	}
	large := largeSizeFragment(t)
	cuts := map[string][]byte{
		"inStypHeader":     first[:7],
		"afterStypHeader":  first[:8],
		"inMoof":           first[:30],
		"beforeMdat":       first[:bytes.Index(first, []byte("mdat"))-4],
		"inMdat":           first[:len(first)-1],
		"inLargeSizeField": large[:bytes.Index(large, []byte("mdat"))+8],
		"inLargeSizeMdat":  large[:len(large)-1],
	}
	var d mp4.FragmentDecoder
	for name, data := range cuts {
		if _, err := d.DecodeSR(bits.NewFixedSliceReader(data), 0); !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Errorf("%s: got error %v, want io.ErrUnexpectedEOF", name, err)
		}
	}
	// After the errors, the decoder decodes a whole fragment, and a box after its mdat starts a fragment that the
	// stream does not finish.
	sr := bits.NewFixedSliceReader(append(bytes.Clone(first), 0, 0, 0, 8, 'f', 'r', 'e', 'e'))
	f, err := d.DecodeSR(sr, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Mdat.Data) == 0 {
		t.Error("no mdat data")
	}
	if _, err := d.DecodeSR(sr, uint64(sr.GetPos())); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Errorf("box after the last mdat: got error %v, want io.ErrUnexpectedEOF", err)
	}
}

// fragmentFiles are the test segments, with the 60 samples of 1.m4s cut into fragments of one sample each.
func fragmentFiles(t *testing.T) map[string][]byte {
	t.Helper()
	files := map[string][]byte{"oneSampleFragments": oneSampleFragments(t)}
	for _, name := range []string{"1.m4s", "multi_sidx_segment.m4s", "interleaved_sidxs_segment.m4s",
		"hvc1_seg_1.m4s", "aac_1.m4s", "av1_multitile_seg.m4s"} {
		data, err := os.ReadFile("testdata/" + name)
		if err != nil {
			t.Fatal(err)
		}
		files[name] = data
	}
	return files
}

// TestReadFragmentBytes - streaming a segment fragment by fragment with ReadFragmentBytes, through readers that
// return little data per call, gives the bytes of the fragments, which decode as with DecodeFileSR.
func TestReadFragmentBytes(t *testing.T) {
	for name, data := range fragmentFiles(t) {
		for rdName, rd := range map[string]func(io.Reader) io.Reader{
			"oneByte": iotest.OneByteReader, "half": iotest.HalfReader} {
			t.Run(name+"/"+rdName, func(t *testing.T) {
				ref, err := mp4.DecodeFileSR(bits.NewFixedSliceReader(data))
				if err != nil {
					t.Fatal(err)
				}
				var want []*mp4.Fragment
				for _, seg := range ref.Segments {
					want = append(want, seg.Fragments...)
				}
				r := rd(bytes.NewReader(data))
				var d mp4.FragmentDecoder
				sr := bits.NewFixedSliceReader(nil)
				var buf []byte
				pos := 0
				for i := 0; ; i++ {
					buf, err = mp4.ReadFragmentBytes(r, buf[:0])
					if errors.Is(err, io.EOF) {
						if i != len(want) {
							t.Fatalf("read %d fragments, want %d", i, len(want))
						}
						break
					}
					if err != nil {
						t.Fatalf("fragment %d: %v", i+1, err)
					}
					if !bytes.Equal(buf, data[pos:pos+len(buf)]) {
						t.Fatalf("fragment %d: bytes differ from the input", i+1)
					}
					sr.Reset(buf)
					f, err := d.DecodeSR(sr, uint64(pos))
					if err != nil {
						t.Fatalf("fragment %d: %v", i+1, err)
					}
					if sr.NrRemainingBytes() != 0 {
						t.Fatalf("fragment %d: %d bytes after the mdat", i+1, sr.NrRemainingBytes())
					}
					var gotEnc, wantEnc bytes.Buffer
					if err := f.Encode(&gotEnc); err != nil {
						t.Fatal(err)
					}
					if err := want[i].Encode(&wantEnc); err != nil {
						t.Fatal(err)
					}
					if !bytes.Equal(gotEnc.Bytes(), wantEnc.Bytes()) {
						t.Fatalf("fragment %d encodes differently from DecodeFileSR", i+1)
					}
					pos += len(buf)
				}
			})
		}
	}
}

func TestReadFragmentBytesErrors(t *testing.T) {
	data := oneSampleFragments(t)
	first, err := mp4.ReadFragmentBytes(bytes.NewReader(data), nil)
	if err != nil {
		t.Fatal(err)
	}
	prefix := []byte{1, 2, 3}

	t.Run("empty", func(t *testing.T) {
		got, err := mp4.ReadFragmentBytes(bytes.NewReader(nil), prefix)
		if !errors.Is(err, io.EOF) || len(got) != len(prefix) {
			t.Errorf("got %d bytes and error %v, want %d bytes and io.EOF", len(got), err, len(prefix))
		}
	})
	t.Run("truncated", func(t *testing.T) {
		for _, n := range []int{1, 7, 8, 30, len(first) - 1} {
			got, err := mp4.ReadFragmentBytes(bytes.NewReader(first[:n]), prefix)
			if !errors.Is(err, io.ErrUnexpectedEOF) || len(got) != len(prefix) {
				t.Errorf("cut at %d: got %d bytes and error %v, want %d bytes and io.ErrUnexpectedEOF",
					n, len(got), err, len(prefix))
			}
		}
	})
	t.Run("mdatFirst", func(t *testing.T) {
		mdat := []byte{0, 0, 0, 9, 'm', 'd', 'a', 't', 0}
		if _, err := mp4.ReadFragmentBytes(bytes.NewReader(mdat), nil); err == nil {
			t.Error("expected an error for an mdat without moof")
		}
	})
	t.Run("hugeSize", func(t *testing.T) {
		// A moof claiming 1 GiB, backed by a few bytes: the buffer grows with the data read, not with the size.
		huge := []byte{0x40, 0, 0, 0, 'm', 'o', 'o', 'f', 1, 2, 3}
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		_, err := mp4.ReadFragmentBytes(bytes.NewReader(huge), nil)
		runtime.ReadMemStats(&after)
		if !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Errorf("got error %v, want io.ErrUnexpectedEOF", err)
		}
		if allocated := after.TotalAlloc - before.TotalAlloc; allocated > 4<<20 {
			t.Errorf("allocated %d bytes for a stream of %d bytes", allocated, len(huge))
		}
	})
}

// largeSizeFragment returns an encoded fragment whose mdat has a 64-bit size field.
func largeSizeFragment(t *testing.T) []byte {
	t.Helper()
	frag, err := mp4.CreateFragment(1, 1)
	if err != nil {
		t.Fatal(err)
	}
	frag.AddFullSample(mp4.FullSample{Sample: mp4.Sample{Flags: mp4.SyncSampleFlags, Dur: 1, Size: 3},
		Data: []byte{7, 8, 9}})
	frag.Mdat.LargeSize = true
	var enc bytes.Buffer
	if err := frag.Encode(&enc); err != nil {
		t.Fatal(err)
	}
	return enc.Bytes()
}

// TestReadFragmentBytesLargeSize - an mdat with a 64-bit size field is read and decoded.
func TestReadFragmentBytesLargeSize(t *testing.T) {
	enc := largeSizeFragment(t)
	buf, err := mp4.ReadFragmentBytes(bytes.NewReader(enc), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buf, enc) {
		t.Fatal("bytes differ from the input")
	}
	var d mp4.FragmentDecoder
	f, err := d.DecodeSR(bits.NewFixedSliceReader(buf), 0)
	if err != nil {
		t.Fatal(err)
	}
	if !f.Mdat.LargeSize || !bytes.Equal(f.Mdat.Data, []byte{7, 8, 9}) {
		t.Errorf("got mdat with large size %t and data %v", f.Mdat.LargeSize, f.Mdat.Data)
	}
}

// TestReadFragmentBytesAllocations - streaming fragment after fragment into one buffer and decoding them with one
// FragmentDecoder and one reader allocates nothing once the buffer and the boxes exist.
func TestReadFragmentBytesAllocations(t *testing.T) {
	data := oneSampleFragments(t)
	rd := bytes.NewReader(data)
	var d mp4.FragmentDecoder
	sr := bits.NewFixedSliceReader(nil)
	var buf []byte
	stream := func() {
		rd.Reset(data)
		pos := 0
		for {
			var err error
			buf, err = mp4.ReadFragmentBytes(rd, buf[:0])
			if errors.Is(err, io.EOF) {
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			sr.Reset(buf)
			f, err := d.DecodeSR(sr, uint64(pos))
			if err != nil {
				t.Fatal(err)
			}
			for _, err := range f.Samples(nil) {
				if err != nil {
					t.Fatal(err)
				}
			}
			pos += len(buf)
		}
	}
	stream()
	if allocs := testing.AllocsPerRun(10, stream); allocs != 0 {
		t.Errorf("streaming %d bytes of fragments made %.0f allocations, want none", len(data), allocs)
	}
}

// Read a fragmented stream fragment by fragment, each into a buffer that is reused for the next, and iterate over
// the samples. Once the decoder has seen the layout of the fragments, this allocates nothing.
func ExampleFragmentDecoder() {
	data, err := os.ReadFile("testdata/interleaved_sidxs_segment.m4s")
	if err != nil {
		panic(err)
	}
	r := bytes.NewReader(data) // stands in for a network stream
	var d mp4.FragmentDecoder
	sr := bits.NewFixedSliceReader(nil)
	var buf []byte
	pos := 0
	var nrFragments, nrSamples, nrBytes int
	for {
		buf, err = mp4.ReadFragmentBytes(r, buf[:0])
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			panic(err)
		}
		sr.Reset(buf)
		f, err := d.DecodeSR(sr, uint64(pos))
		if err != nil {
			panic(err)
		}
		for s, err := range f.Samples(nil) {
			if err != nil {
				panic(err)
			}
			nrSamples++
			nrBytes += len(s.Data)
		}
		nrFragments++
		pos += len(buf)
	}
	fmt.Printf("%d fragments, %d samples, %d bytes\n", nrFragments, nrSamples, nrBytes)
	// Output: 3 fragments, 3 samples, 26610 bytes
}
