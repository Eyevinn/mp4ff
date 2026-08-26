package mp4_test

import (
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

// Sample-table decode benchmarks. Long VoD assets carry sample tables with
// hundreds of thousands of entries (one stsz entry per sample, often one
// ctts entry per sample), so the per-entry decode cost dominates parsing a
// progressive file's moov. Sizes below mirror a real 2.5 h asset.
const (
	benchNrSamples = 120_000
	benchNrChunks  = 20_000
	benchNrSync    = 5_000
)

func encodeBoxToBytes(b *testing.B, box mp4.Box) []byte {
	b.Helper()
	sw := bits.NewFixedSliceWriter(int(box.Size()))
	if err := box.EncodeSW(sw); err != nil {
		b.Fatal(err)
	}
	return sw.Bytes()
}

func benchmarkDecodeBox(b *testing.B, raw []byte) {
	b.ReportAllocs()
	b.SetBytes(int64(len(raw)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sr := bits.NewFixedSliceReader(raw)
		if _, err := mp4.DecodeBoxSR(0, sr); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecodeStsz(b *testing.B) {
	sizes := make([]uint32, benchNrSamples)
	for i := range sizes {
		sizes[i] = uint32(1000 + i%7000)
	}
	stsz := &mp4.StszBox{SampleNumber: benchNrSamples, SampleSize: sizes}
	benchmarkDecodeBox(b, encodeBoxToBytes(b, stsz))
}

func BenchmarkDecodeCtts(b *testing.B) {
	endSampleNr := make([]uint32, benchNrSamples+1)
	offsets := make([]int32, benchNrSamples)
	for i := range offsets {
		endSampleNr[i+1] = uint32(i + 1)
		offsets[i] = int32(i%5) * 1024
	}
	ctts := &mp4.CttsBox{EndSampleNr: endSampleNr, SampleOffset: offsets}
	benchmarkDecodeBox(b, encodeBoxToBytes(b, ctts))
}

func BenchmarkDecodeStco(b *testing.B) {
	offsets := make([]uint32, benchNrChunks)
	for i := range offsets {
		offsets[i] = uint32(i * 40_000)
	}
	stco := &mp4.StcoBox{ChunkOffset: offsets}
	benchmarkDecodeBox(b, encodeBoxToBytes(b, stco))
}

func BenchmarkDecodeStss(b *testing.B) {
	syncs := make([]uint32, benchNrSync)
	for i := range syncs {
		syncs[i] = uint32(i*24 + 1)
	}
	stss := &mp4.StssBox{SampleNumber: syncs}
	benchmarkDecodeBox(b, encodeBoxToBytes(b, stss))
}
