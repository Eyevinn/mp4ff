package mp4_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
	"github.com/go-test/deep"
)

func TestSencDirectValues(t *testing.T) {
	iv8 := mp4.InitializationVector("12345678")
	iv16 := mp4.InitializationVector("0123456789abcdef")
	cases := []struct {
		desc            string
		senc            *mp4.SencBox
		perSampleIVSize byte
	}{
		{
			desc: "No perSampleIVs",
			senc: &mp4.SencBox{
				Version:     0,
				Flags:       0,
				SampleCount: 431, // No perSampleIVs
			},
			perSampleIVSize: 0,
		},
		{
			desc: "perSampleIVSize 8",
			senc: &mp4.SencBox{
				Version:     0,
				Flags:       0,
				SampleCount: 1,
				IVs:         []mp4.InitializationVector{iv8},
				SubSamples:  [][]mp4.SubSamplePattern{{{10, 1000}}},
			},
			perSampleIVSize: 8,
		},
		{
			desc: "perSampleIVSize 16",
			senc: &mp4.SencBox{
				Version:     0,
				Flags:       0,
				SampleCount: 1,
				IVs:         []mp4.InitializationVector{iv16},
				SubSamples:  [][]mp4.SubSamplePattern{{{10, 1000}, {20, 2000}}},
			},
			perSampleIVSize: 16,
		},
		{
			desc: "perSampleIVSize 16, 2 subsamples",
			senc: &mp4.SencBox{
				Version:     0,
				Flags:       0,
				SampleCount: 2,
				IVs:         []mp4.InitializationVector{iv16, iv16},
				SubSamples:  [][]mp4.SubSamplePattern{{{10, 1000}}, {{20, 2000}}},
			},
			perSampleIVSize: 16,
		},
	}

	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			c.senc.SetPerSampleIVSize(c.perSampleIVSize)
			sencDiffAfterEncodeAndDecode(t, c.senc, 0)
			sencDiffAfterEncodeAndDecode(t, c.senc, c.perSampleIVSize)
		})
	}
}

func sencDiffAfterEncodeAndDecode(t *testing.T, senc *mp4.SencBox, perSampleIVSize byte) {
	t.Helper()
	buf := bytes.Buffer{}
	err := senc.Encode(&buf)
	if err != nil {
		t.Error(err)
	}

	boxDec, err := mp4.DecodeBox(0, &buf)
	if err != nil {
		t.Error(err)
	}
	decSenc := boxDec.(*mp4.SencBox)
	var saizBox *mp4.SaizBox

	if decSenc.ReadButNotParsed() {
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
	iv0 := mp4.InitializationVector("")
	iv8 := mp4.InitializationVector("01234567")
	iv16 := mp4.InitializationVector("0123456789abcdef")

	senc := mp4.CreateSencBox()
	err := senc.AddSample(mp4.SencSample{iv0, []mp4.SubSamplePattern{{10, 1000}}})
	assertNoError(t, err)
	sencDiffAfterEncodeAndDecode(t, senc, 0)

	senc = mp4.CreateSencBox()
	err = senc.AddSample(mp4.SencSample{iv8, nil})
	assertNoError(t, err)
	sencDiffAfterEncodeAndDecode(t, senc, 0)
	sencDiffAfterEncodeAndDecode(t, senc, 8)

	senc = mp4.CreateSencBox()
	err = senc.AddSample(mp4.SencSample{iv8, []mp4.SubSamplePattern{{10, 1000}}})
	assertNoError(t, err)
	sencDiffAfterEncodeAndDecode(t, senc, 0)
	sencDiffAfterEncodeAndDecode(t, senc, 8)

	senc = mp4.CreateSencBox()
	err = senc.AddSample(mp4.SencSample{iv8, []mp4.SubSamplePattern{{10, 1000}}})
	assertNoError(t, err)
	sencDiffAfterEncodeAndDecode(t, senc, 0)
	sencDiffAfterEncodeAndDecode(t, senc, 8)

	senc = mp4.CreateSencBox()
	err = senc.AddSample(mp4.SencSample{iv16, []mp4.SubSamplePattern{{10, 1000}, {20, 2000}}})
	assertNoError(t, err)
	sencDiffAfterEncodeAndDecode(t, senc, 0)
	sencDiffAfterEncodeAndDecode(t, senc, 16)

	senc = mp4.CreateSencBox()
	err = senc.AddSample(mp4.SencSample{iv16, []mp4.SubSamplePattern{{10, 1000}}})
	assertNoError(t, err)
	err = senc.AddSample(mp4.SencSample{iv16, []mp4.SubSamplePattern{{20, 2000}}})
	assertNoError(t, err)
	sencDiffAfterEncodeAndDecode(t, senc, 0)
	sencDiffAfterEncodeAndDecode(t, senc, 16)

	senc = mp4.CreateSencBox()
	err = senc.AddSample(mp4.SencSample{iv16, []mp4.SubSamplePattern{{10, 1000}}})
	assertNoError(t, err)
	err = senc.AddSample(mp4.SencSample{iv8, []mp4.SubSamplePattern{{20, 2000}}})
	assertError(t, err, "Should have got error due to different iv size")
}

// TestParseSencWithSeparateInit tests parsing senc when init segment is in a separate file.
// The media segment (seg_cenc_no_seig.m4v) has no seig sample group, so the perSampleIVSize
// must come from the tenc box in the init segment (init_cenc_test.m4i).
func TestParseSencWithSeparateInit(t *testing.T) {
	// Decode init segment
	initData, err := os.ReadFile("testdata/init_cenc_test.m4i")
	if err != nil {
		t.Fatal(err)
	}
	initFile, err := mp4.DecodeFile(bytes.NewBuffer(initData))
	if err != nil {
		t.Fatal(err)
	}
	init := initFile.Init

	// Decode media segment without init (no moov available)
	segData, err := os.ReadFile("testdata/seg_cenc_no_seig.m4v")
	if err != nil {
		t.Fatal(err)
	}
	segFile, err := mp4.DecodeFile(bytes.NewBuffer(segData))
	if err != nil {
		t.Fatal(err)
	}

	// Heuristic with saiz should have parsed senc during decode
	seg := segFile.Segments[0]
	frag := seg.Fragments[0]
	senc := frag.Moof.Traf.Senc
	if senc == nil {
		t.Fatal("expected senc box in traf")
	}
	if senc.ReadButNotParsed() {
		t.Error("expected senc to be parsed by heuristic")
	}
	if !senc.IsParsedByGuess() {
		t.Error("expected senc to be marked as parsed by guess")
	}
	if senc.PerSampleIVSize() != 8 {
		t.Errorf("expected heuristic perSampleIVSize=8, got %d", senc.PerSampleIVSize())
	}

	// Now re-parse with authoritative init segment info
	err = seg.ParseSenc(init)
	if err != nil {
		t.Fatal(err)
	}
	if senc.IsParsedByGuess() {
		t.Error("expected senc to no longer be marked as parsed by guess after ParseSenc")
	}
	if senc.PerSampleIVSize() != 8 {
		t.Errorf("expected perSampleIVSize=8 after ParseSenc, got %d", senc.PerSampleIVSize())
	}
}

// TestParseSencWithSeig tests that senc is parsed using seig when moov is not available.
// moof_enc.m4s has a seig sample group with perSampleIVSize=8.
func TestParseSencWithSeig(t *testing.T) {
	segData, err := os.ReadFile("testdata/moof_enc.m4s")
	if err != nil {
		t.Fatal(err)
	}
	segFile, err := mp4.DecodeFile(bytes.NewBuffer(segData))
	if err != nil {
		t.Fatal(err)
	}
	seg := segFile.Segments[0]
	frag := seg.Fragments[0]
	senc := frag.Moof.Traf.Senc
	if senc == nil {
		t.Fatal("expected senc box in traf")
	}
	if senc.ReadButNotParsed() {
		t.Error("expected senc to be parsed")
	}
	// seig provides the authoritative value, not a guess
	if senc.IsParsedByGuess() {
		t.Error("expected senc parsed via seig, not by guess")
	}
	if senc.PerSampleIVSize() != 8 {
		t.Errorf("expected perSampleIVSize=8, got %d", senc.PerSampleIVSize())
	}
}

// TestParseSencGuessThenReparse tests that a heuristic-parsed senc can be re-parsed
// with an authoritative value, and that re-parsing with 0 is a no-op.
func TestParseSencGuessThenReparse(t *testing.T) {
	// Decode media segment without init — senc will be parsed by heuristic
	segData, err := os.ReadFile("testdata/seg_cenc_no_seig.m4v")
	if err != nil {
		t.Fatal(err)
	}
	segFile, err := mp4.DecodeFile(bytes.NewBuffer(segData))
	if err != nil {
		t.Fatal(err)
	}
	senc := segFile.Segments[0].Fragments[0].Moof.Traf.Senc
	if !senc.IsParsedByGuess() {
		t.Fatal("expected senc to be parsed by guess")
	}

	// Re-parse with 0 should be a no-op (no better info)
	err = senc.ParseReadBox(0, nil)
	if err != nil {
		t.Fatalf("re-parse with 0 should succeed as no-op, got: %v", err)
	}
	if !senc.IsParsedByGuess() {
		t.Error("should still be marked as guess after re-parse with 0")
	}

	// Re-parse with authoritative value should replace the guess
	err = senc.ParseReadBox(8, nil)
	if err != nil {
		t.Fatalf("re-parse with 8 should succeed, got: %v", err)
	}
	if senc.IsParsedByGuess() {
		t.Error("should no longer be marked as guess after authoritative re-parse")
	}
	if senc.PerSampleIVSize() != 8 {
		t.Errorf("expected perSampleIVSize=8, got %d", senc.PerSampleIVSize())
	}
}

func TestParseSencAlreadyParsed(t *testing.T) {
	// Create a senc box, encode it, decode it, parse it, then try to parse again
	senc := mp4.CreateSencBox()
	iv8 := mp4.InitializationVector("01234567")
	err := senc.AddSample(mp4.SencSample{IV: iv8, SubSamples: []mp4.SubSamplePattern{{10, 1000}}})
	if err != nil {
		t.Fatal(err)
	}

	buf := bytes.Buffer{}
	err = senc.Encode(&buf)
	if err != nil {
		t.Fatal(err)
	}

	boxDec, err := mp4.DecodeBox(0, &buf)
	if err != nil {
		t.Fatal(err)
	}
	decSenc := boxDec.(*mp4.SencBox)

	// First parse should succeed
	err = decSenc.ParseReadBox(8, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Second parse with same authoritative value should fail (already parsed, not by guess)
	err = decSenc.ParseReadBox(8, nil)
	if err == nil {
		t.Error("expected error when parsing already-parsed senc box")
	}
}

func TestParseSencNilMoof(t *testing.T) {
	// ParseSenc on a fragment with nil moof should be a no-op
	frag := mp4.NewFragment()
	initData, err := os.ReadFile("testdata/init_cenc_test.m4i")
	if err != nil {
		t.Fatal(err)
	}
	initFile, err := mp4.DecodeFile(bytes.NewBuffer(initData))
	if err != nil {
		t.Fatal(err)
	}
	err = frag.ParseSenc(initFile.Init)
	if err != nil {
		t.Errorf("ParseSenc on nil moof should not error, got: %v", err)
	}
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
		m, err := mp4.ReadMP4File(tc.inputFile)
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

func TestBadSencData(t *testing.T) {
	// raw senc box with version > 2 */
	cases := []struct {
		desc string
		raw  []byte
		err  string
	}{
		{
			desc: "too short",
			raw:  []byte{0x00, 0x00, 0x00, 0x0f, 's', 'e', 'n', 'c', 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
			err:  "decode senc pos 0: payload size 7 less than min size 8",
		},
		{
			desc: "v1 not supported",
			raw:  []byte{0x00, 0x00, 0x00, 0x10, 's', 'e', 'n', 'c', 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
			err:  "decode senc pos 0: version 1 not supported",
		},
		{
			desc: "too short for subsample encryption",
			raw:  []byte{0x00, 0x00, 0x00, 0x10, 's', 'e', 'n', 'c', 0x00, 0x00, 0x00, 0x02, 0x00, 0x00, 0x00, 0xff},
			err:  "decode senc pos 0: payload size 8 too small for 255 samples and subSampleEncryption",
		},
		{
			desc: "extended size rejected for senc (issue 479)",
			raw:  []byte{0x00, 0x00, 0x00, 0x01, 's', 'e', 'n', 'c', 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x10},
			err:  "extended size not supported for box type senc",
		},
	}

	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			buf := bytes.NewBuffer(c.raw)
			_, err := mp4.DecodeBox(0, buf)
			if err == nil {
				t.Errorf("expected error %q, but got nil", c.err)
			}
			if err.Error() != c.err {
				t.Errorf("expected error %q, got %q", c.err, err.Error())
			}

			sr := bits.NewFixedSliceReader(c.raw)
			_, err = mp4.DecodeBoxSR(0, sr)
			if err == nil {
				t.Errorf("expected error %q, but got nil", c.err)
			}
			if err.Error() != c.err {
				t.Errorf("expected error %q, got %q", c.err, err.Error())
			}
		})
	}
}
