package mp4_test

import (
	"bytes"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

// oneSampleFragments returns testdata/1.m4s (60 video samples) re-fragmented into 60 fragments of
// one sample each, the layout of low-latency live streams, where the fixed cost of decoding a
// fragment dominates.
func oneSampleFragments(tb testing.TB) []byte {
	tb.Helper()
	initData, err := os.ReadFile("testdata/init.mp4")
	if err != nil {
		tb.Fatal(err)
	}
	init, err := mp4.DecodeFile(bytes.NewReader(initData))
	if err != nil {
		tb.Fatal(err)
	}
	trex := init.Init.Moov.Mvex.Trex
	data, err := os.ReadFile("testdata/1.m4s")
	if err != nil {
		tb.Fatal(err)
	}
	f, err := mp4.DecodeFileSR(bits.NewFixedSliceReader(data))
	if err != nil {
		tb.Fatal(err)
	}
	samples, err := f.Segments[0].Fragments[0].GetFullSamples(trex)
	if err != nil {
		tb.Fatal(err)
	}
	var buf bytes.Buffer
	if err := f.Segments[0].Styp.Encode(&buf); err != nil {
		tb.Fatal(err)
	}
	for i := range samples {
		frag, err := mp4.CreateFragment(uint32(i+1), trex.TrackID)
		if err != nil {
			tb.Fatal(err)
		}
		frag.AddFullSamples(samples[i : i+1])
		if err := frag.Encode(&buf); err != nil {
			tb.Fatal(err)
		}
	}
	return buf.Bytes()
}

func BenchmarkDecodeFragments(b *testing.B) {
	data := oneSampleFragments(b)
	b.Run("DecodeFileSR", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := mp4.DecodeFileSR(bits.NewFixedSliceReader(data)); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("DecodeFile", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := mp4.DecodeFile(bytes.NewReader(data)); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("FragmentDecoder", func(b *testing.B) {
		b.ReportAllocs()
		var d mp4.FragmentDecoder
		sr := bits.NewFixedSliceReader(data)
		for i := 0; i < b.N; i++ {
			sr.SetPos(0)
			for {
				if _, err := d.DecodeSR(sr, uint64(sr.GetPos())); err != nil {
					if errors.Is(err, io.EOF) {
						break
					}
					b.Fatal(err)
				}
			}
		}
	})
	b.Run("ReadFragmentBytes+FragmentDecoder", func(b *testing.B) {
		b.ReportAllocs()
		var d mp4.FragmentDecoder
		sr := bits.NewFixedSliceReader(nil)
		rd := bytes.NewReader(nil)
		var buf []byte
		for i := 0; i < b.N; i++ {
			rd.Reset(data)
			pos := 0
			for {
				var err error
				if buf, err = mp4.ReadFragmentBytes(rd, buf[:0]); err != nil {
					if errors.Is(err, io.EOF) {
						break
					}
					b.Fatal(err)
				}
				sr.Reset(buf)
				if _, err := d.DecodeSR(sr, uint64(pos)); err != nil {
					b.Fatal(err)
				}
				pos += len(buf)
			}
		}
	})
	b.Run("StreamFile", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			sf, err := mp4.InitDecodeStream(bytes.NewReader(data), mp4.WithMaxFragments(1))
			if err != nil {
				b.Fatal(err)
			}
			if err := sf.ProcessFragments(); err != nil {
				b.Fatal(err)
			}
		}
	})
}
