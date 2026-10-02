package mp4_test

import (
	"io"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

func BenchmarkEncodeFragments(b *testing.B) {
	f, err := mp4.DecodeFileSR(bits.NewFixedSliceReader(oneSampleFragments(b)))
	if err != nil {
		b.Fatal(err)
	}
	frags := f.Segments[0].Fragments
	b.Run("Encode", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			for _, frag := range frags {
				if err := frag.Encode(io.Discard); err != nil {
					b.Fatal(err)
				}
			}
		}
	})
	b.Run("EncodeSW", func(b *testing.B) {
		b.ReportAllocs()
		var buf []byte
		for i := 0; i < b.N; i++ {
			for _, frag := range frags {
				size := int(frag.Size())
				if cap(buf) < size {
					buf = make([]byte, size)
				}
				if err := frag.EncodeSW(bits.NewFixedSliceWriterFromSlice(buf[:size])); err != nil {
					b.Fatal(err)
				}
			}
		}
	})
}
