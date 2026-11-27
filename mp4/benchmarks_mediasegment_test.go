package mp4_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

func BenchmarkFragmentify(b *testing.B) {
	data, err := os.ReadFile("testdata/1.m4s")
	if err != nil {
		b.Fatal(err)
	}
	f, err := mp4.DecodeFile(bytes.NewReader(data))
	if err != nil {
		b.Fatal(err)
	}
	trex := &mp4.TrexBox{TrackID: 2}
	for _, c := range []struct {
		name     string
		duration uint32
	}{
		{"single", 180000},
		{"split", 45000},
	} {
		b.Run(c.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := f.Segments[0].Fragmentify(90000, trex, c.duration); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
