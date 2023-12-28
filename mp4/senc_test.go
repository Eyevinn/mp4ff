package mp4

import (
	"bytes"
	"testing"

	"github.com/go-test/deep"
)

func TestSencDirectValues(t *testing.T) {
	iv8 := InitializationVector("12345678")
	iv16 := InitializationVector("0123456789abcdef")
	sencBoxes := []*SencBox{
		{
			Version:         0,
			Flags:           0,
			SampleCount:     431, // No perSampleIVs
			perSampleIVSize: 0,
		},
		{
			Version:         0,
			Flags:           0,
			SampleCount:     1,
			perSampleIVSize: 8,
			IVs:             []InitializationVector{iv8},
			SubSamples:      [][]SubSamplePattern{{{10, 1000}}},
		},
		{
			Version:         0,
			Flags:           0,
			SampleCount:     1,
			perSampleIVSize: 16,
			IVs:             []InitializationVector{iv16},
			SubSamples:      [][]SubSamplePattern{{{10, 1000}, {20, 2000}}},
		},
		{
			Version:         0,
			Flags:           0,
			SampleCount:     2,
			perSampleIVSize: 16,
			IVs:             []InitializationVector{iv16, iv16},
			SubSamples:      [][]SubSamplePattern{{{10, 1000}}, {{20, 2000}}},
		},
	}

	for _, senc := range sencBoxes {
		sencDiffAfterEncodeAndDecode(t, senc, 0)
		sencDiffAfterEncodeAndDecode(t, senc, senc.perSampleIVSize)
	}
}

func sencDiffAfterEncodeAndDecode(t *testing.T, senc *SencBox, perSampleIVSize byte) {
	t.Helper()
	buf := bytes.Buffer{}
	err := senc.Encode(&buf)
	if err != nil {
		t.Error(err)
	}

	boxDec, err := DecodeBox(0, &buf)
	if err != nil {
		t.Error(err)
	}
	decSenc := boxDec.(*SencBox)
	var saizBox *SaizBox

	if decSenc.readButNotParsed {
		err = decSenc.ParseReadBox(perSampleIVSize, saizBox)
		if err != nil {
			t.Error(err)
		}
	}

	if diff := deep.Equal(decSenc, senc); diff != nil {
		t.Error(diff)
	}
}

func TestAddSamples(t *testing.T) {
	iv0 := InitializationVector("")
	iv8 := InitializationVector("01234567")
	iv16 := InitializationVector("0123456789abcdef")

	senc := CreateSencBox()
	err := senc.AddSample(SencSample{iv0, []SubSamplePattern{{10, 1000}}})
	assertNoError(t, err)
	sencDiffAfterEncodeAndDecode(t, senc, 0)

	senc = CreateSencBox()
	err = senc.AddSample(SencSample{iv8, nil})
	assertNoError(t, err)
	sencDiffAfterEncodeAndDecode(t, senc, 0)
	sencDiffAfterEncodeAndDecode(t, senc, 8)

	senc = CreateSencBox()
	err = senc.AddSample(SencSample{iv8, []SubSamplePattern{{10, 1000}}})
	assertNoError(t, err)
	sencDiffAfterEncodeAndDecode(t, senc, 0)
	sencDiffAfterEncodeAndDecode(t, senc, 8)

	senc = CreateSencBox()
	err = senc.AddSample(SencSample{iv8, []SubSamplePattern{{10, 1000}}})
	assertNoError(t, err)
	sencDiffAfterEncodeAndDecode(t, senc, 0)
	sencDiffAfterEncodeAndDecode(t, senc, 8)

	senc = CreateSencBox()
	err = senc.AddSample(SencSample{iv16, []SubSamplePattern{{10, 1000}, {20, 2000}}})
	assertNoError(t, err)
	sencDiffAfterEncodeAndDecode(t, senc, 0)
	sencDiffAfterEncodeAndDecode(t, senc, 16)

	senc = CreateSencBox()
	err = senc.AddSample(SencSample{iv16, []SubSamplePattern{{10, 1000}}})
	assertNoError(t, err)
	err = senc.AddSample(SencSample{iv16, []SubSamplePattern{{20, 2000}}})
	assertNoError(t, err)
	sencDiffAfterEncodeAndDecode(t, senc, 0)
	sencDiffAfterEncodeAndDecode(t, senc, 16)

	senc = CreateSencBox()
	err = senc.AddSample(SencSample{iv16, []SubSamplePattern{{10, 1000}}})
	assertNoError(t, err)
	err = senc.AddSample(SencSample{iv8, []SubSamplePattern{{20, 2000}}})
	assertError(t, err, "Should have got error due to different iv size")
}

// TestImplicitIVSize tests that the implicit IV size is correctly calculated (perSampleIVSize != 0)
func TestImplicitIVSize(t *testing.T) {
	testCases := []struct {
		inputFile        string
		expectedSencSize int
	}{
		{inputFile: "testdata/2xSencNoMdat.mp4", expectedSencSize: 2248},
	}

	for _, tc := range testCases {
		// Read the file
		m, err := ReadMP4File(tc.inputFile)
		if err != nil {
			t.Error(err)
		}
		frag := m.Segments[0].Fragments[0]
		senc := frag.Moof.Traf.Senc
		if int(senc.Size()) != tc.expectedSencSize {
			t.Errorf("Expected senc size %d, got %d", tc.expectedSencSize, senc.Size())
		}
	}
}
