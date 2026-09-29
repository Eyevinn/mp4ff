package mp4_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

// oneSampleFragments returns testdata/1.m4s (60 video samples) re-fragmented into 60 fragments of
// one sample each, the layout of low-latency live streams, where the fixed cost of decoding a
// fragment dominates.
func oneSampleFragments(b *testing.B) []byte {
	b.Helper()
	initData, err := os.ReadFile("testdata/init.mp4")
	if err != nil {
		b.Fatal(err)
	}
	init, err := mp4.DecodeFile(bytes.NewReader(initData))
	if err != nil {
		b.Fatal(err)
	}
	trex := init.Init.Moov.Mvex.Trex
	data, err := os.ReadFile("testdata/1.m4s")
	if err != nil {
		b.Fatal(err)
	}
	f, err := mp4.DecodeFileSR(bits.NewFixedSliceReader(data))
	if err != nil {
		b.Fatal(err)
	}
	samples, err := f.Segments[0].Fragments[0].GetFullSamples(trex)
	if err != nil {
		b.Fatal(err)
	}
	var buf bytes.Buffer
	if err := f.Segments[0].Styp.Encode(&buf); err != nil {
		b.Fatal(err)
	}
	for i := range samples {
		frag, err := mp4.CreateFragment(uint32(i+1), trex.TrackID)
		if err != nil {
			b.Fatal(err)
		}
		frag.AddFullSamples(samples[i : i+1])
		if err := frag.Encode(&buf); err != nil {
			b.Fatal(err)
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
