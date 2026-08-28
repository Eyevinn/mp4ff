package mp4_test

import (
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

// Fragment-building benchmarks on a realistic video fragment: 96 samples of
// ~7 KB (4 s at 25 fps, ~3.5 Mbps), in two layouts. "contiguous" samples are
// subslices of one buffer, which is what GetFullSamples returns and what a
// re-fragmenting pipeline holds. "scattered" samples are each their own
// allocation, as when reading a lazy mdat sample by sample.

const benchNrFragSamples = 96

func benchSampleSizes() (sizes []int, total int) {
	sizes = make([]int, benchNrFragSamples)
	for i := range sizes {
		sizes[i] = 7000 + i
		total += sizes[i]
	}
	return sizes, total
}

func benchFullSamples(datas [][]byte) []mp4.FullSample {
	samples := make([]mp4.FullSample, 0, len(datas))
	decTime := uint64(0)
	for _, d := range datas {
		samples = append(samples, mp4.FullSample{
			Sample: mp4.Sample{
				Flags: mp4.SyncSampleFlags,
				Dur:   1024,
				Size:  uint32(len(d)),
			},
			DecodeTime: decTime,
			Data:       d,
		})
		decTime += 1024
	}
	return samples
}

// benchContiguousSamples returns the samples and the single buffer holding
// all their data.
func benchContiguousSamples() ([]mp4.FullSample, []byte) {
	sizes, total := benchSampleSizes()
	buf := make([]byte, total)
	datas := make([][]byte, 0, len(sizes))
	pos := 0
	for _, size := range sizes {
		datas = append(datas, buf[pos:pos+size])
		pos += size
	}
	return benchFullSamples(datas), buf
}

func benchScatteredSamples() []mp4.FullSample {
	sizes, _ := benchSampleSizes()
	datas := make([][]byte, 0, len(sizes))
	for _, size := range sizes {
		datas = append(datas, make([]byte, size))
	}
	return benchFullSamples(datas)
}

func benchLayouts() map[string][]mp4.FullSample {
	contiguous, _ := benchContiguousSamples()
	return map[string][]mp4.FullSample{
		"contiguous": contiguous,
		"scattered":  benchScatteredSamples(),
	}
}

func newBenchFragment(b *testing.B) *mp4.Fragment {
	frag, err := mp4.CreateFragment(1, 1)
	if err != nil {
		b.Fatal(err)
	}
	return frag
}

// BenchmarkBuildFragment compares the ways of building a fragment from
// samples held in memory. AddSamples+SetData and AddSampleInterval apply to
// contiguous data only and take their []Sample and payload prepared up front,
// which is the shape their callers hold.
func BenchmarkBuildFragment(b *testing.B) {
	for _, layout := range []string{"contiguous", "scattered"} {
		samples := benchLayouts()[layout]
		b.Run(layout+"/AddFullSample-loop", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				frag := newBenchFragment(b)
				for _, s := range samples {
					frag.AddFullSample(s)
				}
			}
		})
		b.Run(layout+"/AddFullSamples", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				frag := newBenchFragment(b)
				frag.AddFullSamples(samples)
			}
		})
	}

	contiguous, payload := benchContiguousSamples()
	plainSamples := make([]mp4.Sample, len(contiguous))
	for i := range contiguous {
		plainSamples[i] = contiguous[i].Sample
	}
	b.Run("contiguous/AddSamples+SetData", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			frag := newBenchFragment(b)
			frag.AddSamples(plainSamples, contiguous[0].DecodeTime)
			frag.Mdat.SetData(payload)
		}
	})
	itvl := mp4.SampleInterval{
		FirstDecodeTime: contiguous[0].DecodeTime,
		Samples:         plainSamples,
		Size:            uint32(len(payload)),
		Data:            payload,
	}
	b.Run("contiguous/AddSampleInterval", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			frag := newBenchFragment(b)
			if err := frag.AddSampleInterval(itvl); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkBuildAndEncodeFragment includes the encode into a preallocated
// slice writer, since a parts-based mdat defers its one payload copy from
// build time to encode time.
func BenchmarkBuildAndEncodeFragment(b *testing.B) {
	for _, layout := range []string{"contiguous", "scattered"} {
		samples := benchLayouts()[layout]
		sized := newBenchFragment(b)
		sized.AddFullSamples(samples)
		out := make([]byte, sized.Size())
		b.Run(layout+"/AddFullSample-loop", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				frag := newBenchFragment(b)
				for _, s := range samples {
					frag.AddFullSample(s)
				}
				if err := frag.EncodeSW(bits.NewFixedSliceWriterFromSlice(out)); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(layout+"/AddFullSamples", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				frag := newBenchFragment(b)
				frag.AddFullSamples(samples)
				if err := frag.EncodeSW(bits.NewFixedSliceWriterFromSlice(out)); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
