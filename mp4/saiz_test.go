package mp4_test

import (
	"bytes"
	"encoding/binary"
	"slices"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

func TestSaiz(t *testing.T) {
	cases := []struct {
		desc string
		saiz *mp4.SaizBox
		size uint64
	}{
		{"version 0 default size", &mp4.SaizBox{DefaultSampleInfoSize: 1}, 17},
		{"version 0 per-sample sizes", &mp4.SaizBox{SampleCount: 2, SampleInfo: []uint32{16, 255}}, 19},
		{"version 1 per-sample sizes", &mp4.SaizBox{Version: 1, Flags: 1, AuxInfoType: "cenc",
			SampleCount: 2, SampleInfo: []uint32{16, 65535}}, 30},
		{"version 1 default size", &mp4.SaizBox{Version: 1, DefaultSampleInfoSize: 310, SampleCount: 3}, 18},
		{"version 2 per-sample sizes", &mp4.SaizBox{Version: 2, SampleCount: 2, SampleInfo: []uint32{16, 70000}}, 28},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			if got := c.saiz.Size(); got != c.size {
				t.Errorf("size %d, want %d", got, c.size)
			}
			boxDiffAfterEncodeAndDecode(t, c.saiz)
		})
	}
}

func TestSaizEncodeErrors(t *testing.T) {
	cases := []struct {
		desc string
		saiz *mp4.SaizBox
	}{
		{"default size too big for version 0", &mp4.SaizBox{DefaultSampleInfoSize: 256, SampleCount: 1}},
		{"sample size too big for version 1", &mp4.SaizBox{Version: 1, SampleCount: 1, SampleInfo: []uint32{65536}}},
		{"fewer sizes than samples", &mp4.SaizBox{SampleCount: 2, SampleInfo: []uint32{16}}},
		{"version 3", &mp4.SaizBox{Version: 3, DefaultSampleInfoSize: 1}},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			sw := bits.NewFixedSliceWriter(64)
			if err := c.saiz.EncodeSW(sw); err == nil {
				t.Error("expected error")
			}
		})
	}
}

func TestSaizDecodeErrors(t *testing.T) {
	// makeSaiz returns a saiz box with no flags, the given version, size fields
	// of sizeBytes bytes (default size 0 and sampleCount per-sample sizes),
	// and declared size declared.
	makeSaiz := func(version byte, sizeBytes, sampleCount int, declared uint32) []byte {
		b := make([]byte, 12+sizeBytes+4+sizeBytes*sampleCount)
		binary.BigEndian.PutUint32(b[0:4], declared)
		copy(b[4:8], "saiz")
		b[8] = version
		binary.BigEndian.PutUint32(b[12+sizeBytes:], uint32(sampleCount))
		return b
	}
	cases := []struct {
		desc string
		data []byte
	}{
		{"version 3", makeSaiz(3, 4, 1, 24)},
		{"version 1 with version 0 layout", makeSaiz(1, 1, 2, 19)},
		{"version 2 with version 1 layout", makeSaiz(2, 2, 2, 22)},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			if _, err := mp4.DecodeBox(0, bytes.NewReader(c.data)); err == nil {
				t.Error("DecodeBox: expected error")
			}
			if _, err := mp4.DecodeBoxSR(0, bits.NewFixedSliceReader(c.data)); err == nil {
				t.Error("DecodeBoxSR: expected error")
			}
		})
	}
	// The correct layouts decode.
	for _, v := range []struct {
		version   byte
		sizeBytes int
	}{{0, 1}, {1, 2}, {2, 4}} {
		data := makeSaiz(v.version, v.sizeBytes, 2, uint32(12+v.sizeBytes+4+2*v.sizeBytes))
		box, err := mp4.DecodeBoxSR(0, bits.NewFixedSliceReader(data))
		if err != nil {
			t.Errorf("version %d: %v", v.version, err)
			continue
		}
		if saiz := box.(*mp4.SaizBox); saiz.Version != v.version || len(saiz.SampleInfo) != 2 {
			t.Errorf("version %d: got version %d with %d sizes", v.version, saiz.Version, len(saiz.SampleInfo))
		}
	}
}

func TestSaizAddSampleInfo(t *testing.T) {
	iv8 := make([]byte, 8)
	iv16 := make([]byte, 16)
	subs1 := []mp4.SubSamplePattern{{BytesOfClearData: 10, BytesOfProtectedData: 1000}}
	subs2 := []mp4.SubSamplePattern{{BytesOfClearData: 10, BytesOfProtectedData: 1000},
		{BytesOfClearData: 20, BytesOfProtectedData: 2000}}

	t.Run("uniform sizes collapse to default", func(t *testing.T) {
		saiz := mp4.NewSaizBox(2)
		assertNoError(t, saiz.AddSampleInfo(iv8, subs1))
		assertNoError(t, saiz.AddSampleInfo(iv8, subs1))
		if saiz.DefaultSampleInfoSize != 16 { // 8 + 2 + 6
			t.Errorf("got default size %d, expected 16", saiz.DefaultSampleInfoSize)
		}
		if saiz.SampleCount != 2 || len(saiz.SampleInfo) != 0 || saiz.Version != 0 {
			t.Errorf("got sampleCount %d, %d per-sample sizes, version %d; expected 2, 0 and 0",
				saiz.SampleCount, len(saiz.SampleInfo), saiz.Version)
		}
	})

	t.Run("differing size switches to per-sample sizes", func(t *testing.T) {
		saiz := mp4.NewSaizBox(3)
		assertNoError(t, saiz.AddSampleInfo(iv8, subs1))
		assertNoError(t, saiz.AddSampleInfo(iv8, subs2))
		assertNoError(t, saiz.AddSampleInfo(iv8, subs1))
		if saiz.DefaultSampleInfoSize != 0 {
			t.Errorf("got default size %d, expected 0", saiz.DefaultSampleInfoSize)
		}
		wanted := []uint32{16, 22, 16}
		if saiz.SampleCount != 3 || !slices.Equal(saiz.SampleInfo, wanted) {
			t.Errorf("got sampleCount %d, sizes %v; expected 3 and %v",
				saiz.SampleCount, saiz.SampleInfo, wanted)
		}
	})

	t.Run("zero size records nothing", func(t *testing.T) {
		saiz := mp4.NewSaizBox(1)
		assertNoError(t, saiz.AddSampleInfo(nil, nil))
		if saiz.SampleCount != 0 || saiz.DefaultSampleInfoSize != 0 || len(saiz.SampleInfo) != 0 {
			t.Errorf("expected empty saiz, got %+v", saiz)
		}
	})

	t.Run("more than 39 subsamples with 16-byte IVs need version 1", func(t *testing.T) {
		saiz := mp4.NewSaizBox(2)
		assertNoError(t, saiz.AddSampleInfo(iv16, make([]mp4.SubSamplePattern, 39))) // 16 + 2 + 39*6 = 252
		if saiz.Version != 0 {
			t.Errorf("got version %d after size 252, expected 0", saiz.Version)
		}
		assertNoError(t, saiz.AddSampleInfo(iv16, make([]mp4.SubSamplePattern, 40))) // 258
		if saiz.Version != 1 || !slices.Equal(saiz.SampleInfo, []uint32{252, 258}) {
			t.Errorf("got version %d, sizes %v; expected 1 and [252 258]", saiz.Version, saiz.SampleInfo)
		}
		boxDiffAfterEncodeAndDecode(t, saiz)
	})

	t.Run("sizes above 65535 need version 2", func(t *testing.T) {
		saiz := mp4.NewSaizBox(1)
		assertNoError(t, saiz.AddSampleInfo(iv8, make([]mp4.SubSamplePattern, 11000))) // 8 + 2 + 66000
		if saiz.Version != 2 || saiz.DefaultSampleInfoSize != 66010 {
			t.Errorf("got version %d, default size %d; expected 2 and 66010", saiz.Version, saiz.DefaultSampleInfoSize)
		}
		saiz.SampleInfo = nil // preallocated by NewSaizBox, but not set when decoding a default size
		boxDiffAfterEncodeAndDecode(t, saiz)
	})
}
