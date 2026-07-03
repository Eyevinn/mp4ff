package mp4_test

import (
	"bytes"
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

func TestSaiz(t *testing.T) {
	saiz := &mp4.SaizBox{DefaultSampleInfoSize: 1}
	boxDiffAfterEncodeAndDecode(t, saiz)
}

func TestSaizAddSampleInfo(t *testing.T) {
	iv8 := make([]byte, 8)
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
		if saiz.SampleCount != 2 || len(saiz.SampleInfo) != 0 {
			t.Errorf("got sampleCount %d, %d per-sample sizes; expected 2 and 0",
				saiz.SampleCount, len(saiz.SampleInfo))
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
		wanted := []byte{16, 22, 16}
		if saiz.SampleCount != 3 || !bytes.Equal(saiz.SampleInfo, wanted) {
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

	t.Run("size above 255 is an error", func(t *testing.T) {
		saiz := mp4.NewSaizBox(1)
		manySubs := make([]mp4.SubSamplePattern, 43) // 8 + 2 + 43*6 = 268
		err := saiz.AddSampleInfo(iv8, manySubs)
		if err == nil {
			t.Error("expected error for sample info size above 255")
		}
	})
}
