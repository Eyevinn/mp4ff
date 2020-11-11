package mp4

import "testing"

func TestSencDirectValues(t *testing.T) {
	iv8 := InitializationVector("12345678")
	iv16 := InitializationVector("0123456789abcdef")
	sencBoxes := []*SencBox{
		{
			Version: 0,
			Flags:   0,
		},
		{
			Version:     0,
			Flags:       0,
			SampleCount: 1,
			IVs:         []InitializationVector{iv8},
		},
		{
			Version:     0,
			Flags:       0,
			SampleCount: 1,
			SubSamples:  [][]SubSamplePattern{{{10, 1000}}},
		},
		{
			Version:     0,
			Flags:       0,
			SampleCount: 1,
			IVs:         []InitializationVector{iv8},
			SubSamples:  [][]SubSamplePattern{{{10, 1000}}},
		},
		{
			Version:     0,
			Flags:       0,
			SampleCount: 1,
			IVs:         []InitializationVector{iv16},
			SubSamples:  [][]SubSamplePattern{{{10, 1000}, {20, 2000}}},
		},
		{
			Version:     0,
			Flags:       0,
			SampleCount: 2,
			IVs:         []InitializationVector{iv16, iv16},
			SubSamples:  [][]SubSamplePattern{{{10, 1000}}, {{20, 2000}}},
		},
	}

	for _, senc := range sencBoxes {
		boxDiffAfterEncodeAndDecode(t, senc)
	}
}

func TestAddSamples(t *testing.T) {
	iv0 := InitializationVector("")
	iv8 := InitializationVector("01234567")
	iv16 := InitializationVector("0123456789abcdef")

	senc := CreateSencBox()
	err := senc.AddSample(SencSample{iv0, []SubSamplePattern{{10, 1000}}})
	assertNoError(t, err)
	boxDiffAfterEncodeAndDecode(t, senc)

	senc = CreateSencBox()
	err = senc.AddSample(SencSample{iv8, nil})
	assertNoError(t, err)
	boxDiffAfterEncodeAndDecode(t, senc)

	senc = CreateSencBox()
	err = senc.AddSample(SencSample{iv8, []SubSamplePattern{{10, 1000}}})
	assertNoError(t, err)
	boxDiffAfterEncodeAndDecode(t, senc)

	senc = CreateSencBox()
	err = senc.AddSample(SencSample{iv8, []SubSamplePattern{{10, 1000}}})
	assertNoError(t, err)
	boxDiffAfterEncodeAndDecode(t, senc)

	senc = CreateSencBox()
	err = senc.AddSample(SencSample{iv16, []SubSamplePattern{{10, 1000}, {20, 2000}}})
	assertNoError(t, err)
	boxDiffAfterEncodeAndDecode(t, senc)

	senc = CreateSencBox()
	err = senc.AddSample(SencSample{iv16, []SubSamplePattern{{10, 1000}}})
	assertNoError(t, err)
	err = senc.AddSample(SencSample{iv16, []SubSamplePattern{{20, 2000}}})
	assertNoError(t, err)
	boxDiffAfterEncodeAndDecode(t, senc)

	senc = CreateSencBox()
	err = senc.AddSample(SencSample{iv16, []SubSamplePattern{{10, 1000}}})
	assertNoError(t, err)
	err = senc.AddSample(SencSample{iv8, []SubSamplePattern{{20, 2000}}})
	assertError(t, err, "Should have got error due to different iv size")
}
