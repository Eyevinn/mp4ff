package mp4_test

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"slices"
	"testing"

	"github.com/Eyevinn/mp4ff/avc"
	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
	"github.com/go-test/deep"
)

func TestFindAVCSubsampleRanges(t *testing.T) {
	infile := "testdata/1.m4s"
	fmp4, err := mp4.ReadMP4File(infile)
	if err != nil {
		t.Error(err)
	}
	fss, err := fmp4.Segments[0].Fragments[0].GetFullSamples(nil)
	if err != nil {
		t.Error(err)
	}
	firstSampleData := fss[0].Data
	spss, ppss := avc.GetParameterSets(firstSampleData)
	spsMap := make(map[uint32]*avc.SPS)
	for _, spsNalu := range spss {
		fmt.Printf("hexSPS: %s\n", hex.EncodeToString(spsNalu))
		sps, err := avc.ParseSPSNALUnit(spsNalu, true)
		if err != nil {
			t.Error(err)
		}
		spsMap[sps.ParameterID] = sps
	}
	ppsMap := make(map[uint32]*avc.PPS)
	for _, ppsNalu := range ppss {
		fmt.Printf("hexPPS: %s\n", hex.EncodeToString(ppsNalu))
		pps, err := avc.ParsePPSNALUnit(ppsNalu, spsMap)
		if err != nil {
			t.Error(err)
		}
		ppsMap[pps.PicParameterSetID] = pps
	}
	testCases := []struct {
		scheme         string
		expectedRanges [][]mp4.SubSamplePattern
	}{
		{
			scheme: "cenc",
			expectedRanges: [][]mp4.SubSamplePattern{
				{{BytesOfClearData: 906, BytesOfProtectedData: 2224}},
				{{BytesOfClearData: 103, BytesOfProtectedData: 80}},
				{{BytesOfClearData: 104, BytesOfProtectedData: 64}},
				{{BytesOfClearData: 97, BytesOfProtectedData: 32}},
				{{BytesOfClearData: 102, BytesOfProtectedData: 0}},
				{{BytesOfClearData: 82, BytesOfProtectedData: 0}},
				{{BytesOfClearData: 97, BytesOfProtectedData: 0}},
				{{BytesOfClearData: 103, BytesOfProtectedData: 96}},
			},
		},
		{
			scheme: "cbcs",
			expectedRanges: [][]mp4.SubSamplePattern{
				{{BytesOfClearData: 805, BytesOfProtectedData: 2325}},
				{{BytesOfClearData: 10, BytesOfProtectedData: 173}},
				{{BytesOfClearData: 13, BytesOfProtectedData: 155}},
				{{BytesOfClearData: 15, BytesOfProtectedData: 114}},
			},
		},
	}

	for _, tc := range testCases {
		for i, fs := range fss {
			if i >= len(tc.expectedRanges) {
				break
			}
			data := fs.Data
			nalus, err := avc.GetNalusFromSample(data)
			if err != nil {
				t.Error(err)
			}
			for _, nalu := range nalus {
				t.Logf("Sample %d: NALU %d %dB\n", i+1, avc.GetNaluType(nalu[0]), len(nalu))
			}
			protectRanges, err := mp4.GetAVCProtectRanges(spsMap, ppsMap, data, tc.scheme)
			if err != nil {
				t.Error(err)
			}
			diff := deep.Equal(protectRanges, tc.expectedRanges[i])
			if diff != nil {
				t.Errorf("Mode %q sample %d: %v", tc.scheme, i+1, diff)
			}
		}
	}
}

func TestEncryptDecrypt(t *testing.T) {
	videoAVCInit := "testdata/init.mp4"
	videoAVCSeg := "testdata/1.m4s"
	videoHEVCInit := "testdata/hvc1_init.mp4"
	videoHEVCSeg := "testdata/hvc1_seg_1.m4s"
	videoAV1Init := "testdata/av1_multitile_init.mp4"
	videoAV1Seg := "testdata/av1_multitile_seg.m4s"
	audioInit := "testdata/aac_init.mp4"
	audioSeg := "testdata/aac_1.m4s"
	keyHex := "00112233445566778899aabbccddeeff"
	ivHex8 := "7766554433221100"
	ivHex16 := "ffeeddccbbaa99887766554433221100"
	kidHex := "11112222333344445555666677778888"
	key, _ := hex.DecodeString(keyHex)
	kidUUID, _ := mp4.NewUUIDFromString(kidHex)
	psshFile := "testdata/pssh.bin"
	psh, err := os.Open(psshFile)
	if err != nil {
		t.Fatal(err)
	}
	box, err := mp4.DecodeBox(0, psh)
	if err != nil {
		t.Fatal(err)
	}
	pssh := box.(*mp4.PsshBox)

	testCases := []struct {
		desc    string
		init    string
		seg     string
		scheme  string
		iv      string
		hasPssh bool
	}{
		{desc: "video AVC cenc iv8", init: videoAVCInit, seg: videoAVCSeg, scheme: "cenc", iv: ivHex8},
		{desc: "video AVC cbcs iv8", init: videoAVCInit, seg: videoAVCSeg, scheme: "cbcs", iv: ivHex8},
		{desc: "video AVC cbcs iv16", init: videoAVCInit, seg: videoAVCSeg, scheme: "cbcs", iv: ivHex16},
		{desc: "video HEVC cenc iv8", init: videoHEVCInit, seg: videoHEVCSeg, scheme: "cenc", iv: ivHex8},
		{desc: "video HEVC cbcs iv8", init: videoHEVCInit, seg: videoHEVCSeg, scheme: "cbcs", iv: ivHex8},
		{desc: "video HEVC cbcs iv16", init: videoHEVCInit, seg: videoHEVCSeg, scheme: "cbcs", iv: ivHex16},
		{desc: "video AV1 cenc iv8", init: videoAV1Init, seg: videoAV1Seg, scheme: "cenc", iv: ivHex8},
		{desc: "video AV1 cenc iv16", init: videoAV1Init, seg: videoAV1Seg, scheme: "cenc", iv: ivHex16},
		{desc: "video AV1 cbcs iv8", init: videoAV1Init, seg: videoAV1Seg, scheme: "cbcs", iv: ivHex8},
		{desc: "audio AAC cbcs iv16", init: audioInit, seg: audioSeg, scheme: "cbcs", iv: ivHex16, hasPssh: true},
	}
	for _, c := range testCases {
		t.Run(c.desc, func(t *testing.T) {
			ifh, err := os.Open(c.init)
			if err != nil {
				t.Fatal(err)
			}
			init, err := mp4.DecodeFile(ifh)
			ifh.Close()
			if err != nil {
				t.Fatal(err)
			}
			iv, err := hex.DecodeString(c.iv)
			if err != nil {
				t.Fatal(err)
			}

			var psshs []*mp4.PsshBox
			if c.hasPssh {
				psshs = []*mp4.PsshBox{pssh}
			}

			ipf, err := mp4.InitProtect(init.Init, key, iv, c.scheme, kidUUID, psshs)
			if err != nil {
				t.Fatal(err)
			}
			// Write init segment with encryption info
			encInitBuf := bytes.Buffer{}
			err = init.Encode(&encInitBuf)
			if err != nil {
				t.Fatal(err)
			}

			// Check that one can extract the protection the InitProtectData from the init segment
			ipd, err := mp4.ExtractInitProtectData(init.Init)
			if err != nil {
				t.Fatal(err)
			}
			diff := deep.Equal(ipd, ipf)
			if len(diff) > 0 {
				t.Errorf("InitProtectData not equal after extraction")
			}

			// Encrypt and write media segment
			rawSeg, err := os.ReadFile(c.seg)
			if err != nil {
				t.Fatal(err)
			}
			rs := bytes.NewBuffer(rawSeg)
			seg, err := mp4.DecodeFile(rs)
			if err != nil {
				t.Fatal(err)
			}
			fragIV := iv
			for _, s := range seg.Segments {
				for _, f := range s.Fragments {
					fragIV, err = mp4.EncryptFragment(f, key, fragIV, ipf)
					if err != nil {
						t.Error(err)
					}
				}
			}
			outBuf := bytes.Buffer{}
			err = seg.Encode(&outBuf)
			if err != nil {
				t.Error(err)
			}
			// Get decrypt info from init segment
			encInitRaw := encInitBuf.Bytes()
			sr := bits.NewFixedSliceReader(encInitRaw)
			encInit, err := mp4.DecodeFileSR(sr)
			if err != nil {
				t.Error(err)
			}
			decInfo, err := mp4.DecryptInit(encInit.Init)
			if err != nil {
				t.Error(err)
			}

			// Decode and decrypt the written segment
			sr = bits.NewFixedSliceReader(outBuf.Bytes())
			decode, err := mp4.DecodeFileSR(sr)
			if err != nil {
				t.Error(err)
			}
			// Decrypt the segment using DecryptFragment directly
			for _, s := range decode.Segments {
				for _, f := range s.Fragments {
					err := mp4.DecryptFragment(f, decInfo, key)
					if err != nil {
						t.Error(err)
					}
				}
			}

			decSegBuf := bytes.Buffer{}
			err = decode.Encode(&decSegBuf)
			if err != nil {
				t.Error(err)
			}

			if !bytes.Equal(rawSeg, decSegBuf.Bytes()) {
				t.Errorf("segment not equal after encryption+decryption")
			}

			// Make a new encryption to check that the decrypted segment is OK
			// for re-encryption (Issue #378).

			pd2, err := mp4.InitProtect(encInit.Init, key, iv, c.scheme, kidUUID, nil)
			if err != nil {
				t.Error(err)
			}
			fragIV = iv
			for _, s := range decode.Segments {
				for _, f := range s.Fragments {
					fragIV, err = mp4.EncryptFragment(f, key, fragIV, pd2)
					if err != nil {
						t.Errorf("Error re-encrypting fragment: %v\n", err)
					}
				}
			}
		})
	}
}

// TestEncryptFragmentIVChaining is a regression test for issue #499.
// EncryptFragment must return an updated IV so that callers encrypting
// multiple fragments with the same key advance the IV between fragments
// and avoid IV/keystream reuse.
func TestEncryptFragmentIVChaining(t *testing.T) {
	initFile := "testdata/init.mp4"
	segFile := "testdata/1.m4s"
	keyHex := "00112233445566778899aabbccddeeff"
	ivHex := "7766554433221100"
	kidHex := "11112222333344445555666677778888"
	key, _ := hex.DecodeString(keyHex)
	iv, _ := hex.DecodeString(ivHex)
	kidUUID, _ := mp4.NewUUIDFromString(kidHex)

	ifh, err := os.Open(initFile)
	if err != nil {
		t.Fatal(err)
	}
	initSeg, err := mp4.DecodeFile(ifh)
	ifh.Close()
	if err != nil {
		t.Fatal(err)
	}
	ipd, err := mp4.InitProtect(initSeg.Init, key, iv, "cenc", kidUUID, nil)
	if err != nil {
		t.Fatal(err)
	}

	rawSeg, err := os.ReadFile(segFile)
	if err != nil {
		t.Fatal(err)
	}

	loadFragment := func() *mp4.Fragment {
		seg, err := mp4.DecodeFile(bytes.NewBuffer(rawSeg))
		if err != nil {
			t.Fatal(err)
		}
		if len(seg.Segments) == 0 || len(seg.Segments[0].Fragments) == 0 {
			t.Fatal("expected at least one fragment in segment")
		}
		return seg.Segments[0].Fragments[0]
	}

	fragA := loadFragment()
	nextIV, err := mp4.EncryptFragment(fragA, key, iv, ipd)
	if err != nil {
		t.Fatalf("EncryptFragment fragA: %v", err)
	}
	if bytes.Equal(nextIV, iv) {
		t.Fatalf("expected returned IV to differ from input IV after cenc encryption")
	}

	fragB := loadFragment()
	_, err = mp4.EncryptFragment(fragB, key, nextIV, ipd)
	if err != nil {
		t.Fatalf("EncryptFragment fragB: %v", err)
	}

	sencA := fragA.Moof.Traf.Senc
	sencB := fragB.Moof.Traf.Senc
	if sencA == nil || sencB == nil {
		t.Fatal("missing senc box after encryption")
	}
	if len(sencA.IVs) == 0 || len(sencB.IVs) == 0 {
		t.Fatal("expected per-sample IVs in senc for cenc")
	}
	if bytes.Equal(sencA.IVs[0], sencB.IVs[0]) {
		t.Fatalf("first-sample IVs must differ between chained fragments; got %x for both",
			sencA.IVs[0])
	}
	if !bytes.Equal(sencB.IVs[0], nextIV) {
		t.Fatalf("fragB first-sample IV %x does not match returned next IV %x",
			sencB.IVs[0], nextIV)
	}
}

// TestEncryptBuiltFragment - a fragment built with AddFullSamples, whose sample data are in mdat data parts, is
// encrypted as one built with AddFullSample.
func TestEncryptBuiltFragment(t *testing.T) {
	key, _ := hex.DecodeString("00112233445566778899aabbccddeeff")
	iv, _ := hex.DecodeString("ffeeddccbbaa99887766554433221100")
	kid, _ := mp4.NewUUIDFromString("11112222333344445555666677778888")
	rawSeg, err := os.ReadFile("testdata/1.m4s")
	if err != nil {
		t.Fatal(err)
	}
	for _, scheme := range []string{"cenc", "cbcs"} {
		t.Run(scheme, func(t *testing.T) {
			init, err := mp4.ReadMP4File("testdata/init.mp4")
			if err != nil {
				t.Fatal(err)
			}
			ipd, err := mp4.InitProtect(init.Init, key, iv, scheme, kid, nil)
			if err != nil {
				t.Fatal(err)
			}
			seg, err := mp4.DecodeFile(bytes.NewReader(rawSeg))
			if err != nil {
				t.Fatal(err)
			}
			inFrag := seg.Segments[0].Fragments[0]
			samples, err := inFrag.GetFullSamples(ipd.Trex)
			if err != nil {
				t.Fatal(err)
			}
			trackID := inFrag.Moof.Traf.Tfhd.TrackID
			build := func(bulk bool) []byte {
				frag, err := mp4.CreateFragment(1, trackID)
				if err != nil {
					t.Fatal(err)
				}
				// Each sample in its own buffer, so that AddFullSamples makes one data part per sample.
				ss := make([]mp4.FullSample, len(samples))
				for i, s := range samples {
					ss[i] = s
					ss[i].Data = bytes.Clone(s.Data)
				}
				if bulk {
					frag.AddFullSamples(ss)
				} else {
					for _, s := range ss {
						frag.AddFullSample(s)
					}
				}
				if _, err := mp4.EncryptFragment(frag, key, iv, ipd); err != nil {
					t.Fatalf("EncryptFragment: %v", err)
				}
				var buf bytes.Buffer
				if err := frag.Encode(&buf); err != nil {
					t.Fatal(err)
				}
				return buf.Bytes()
			}
			if got, want := build(true), build(false); !bytes.Equal(got, want) {
				t.Error("fragment built with AddFullSamples is not encrypted as one built with AddFullSample")
			}
		})
	}
}

func TestDecryptInit(t *testing.T) {
	encFile := "testdata/prog_8s_enc_dashinit.mp4"
	mp4f, err := mp4.ReadMP4File(encFile)
	if err != nil {
		t.Error(err)
	}
	init := mp4f.Init
	decInfo, err := mp4.DecryptInit(init)
	if err != nil {
		t.Error(err)
	}
	if len(decInfo.Psshs) != 1 {
		t.Error("Pssh not extracted")
	}
	for _, tr := range decInfo.TrackInfos {
		schemeType := tr.Sinf.Schm.SchemeType
		if schemeType != "cenc" {
			t.Errorf("Expected cenc, got %s", schemeType)
		}
	}
}

func TestAppendProtectRange(t *testing.T) {
	testCases := []struct {
		name        string
		nrClear     uint32
		nrProtected uint32
		expected    []mp4.SubSamplePattern
	}{
		{
			name:        "clear data under limit",
			nrClear:     1000,
			nrProtected: 500,
			expected: []mp4.SubSamplePattern{
				{BytesOfClearData: 1000, BytesOfProtectedData: 500},
			},
		},
		{
			name:        "clear data at limit",
			nrClear:     65535,
			nrProtected: 500,
			expected: []mp4.SubSamplePattern{
				{BytesOfClearData: 65535, BytesOfProtectedData: 500},
			},
		},
		{
			name:        "clear data just over limit",
			nrClear:     65536,
			nrProtected: 500,
			expected: []mp4.SubSamplePattern{
				{BytesOfClearData: 65535, BytesOfProtectedData: 0},
				{BytesOfClearData: 1, BytesOfProtectedData: 500},
			},
		},
		{
			name:        "clear data well over limit",
			nrClear:     140000,
			nrProtected: 800,
			expected: []mp4.SubSamplePattern{
				{BytesOfClearData: 65535, BytesOfProtectedData: 0},
				{BytesOfClearData: 65535, BytesOfProtectedData: 0},
				{BytesOfClearData: 140000 - 65535 - 65535, BytesOfProtectedData: 800},
			},
		},
		{
			name:        "multiple of 65535 plus remainder",
			nrClear:     65535*3 + 42,
			nrProtected: 100,
			expected: []mp4.SubSamplePattern{
				{BytesOfClearData: 65535, BytesOfProtectedData: 0},
				{BytesOfClearData: 65535, BytesOfProtectedData: 0},
				{BytesOfClearData: 65535, BytesOfProtectedData: 0},
				{BytesOfClearData: 42, BytesOfProtectedData: 100},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := mp4.AppendProtectRange(nil, tc.nrClear, tc.nrProtected)
			if diff := deep.Equal(result, tc.expected); diff != nil {
				t.Errorf("AppendProtectRange(%d, %d) returned unexpected result: %v",
					tc.nrClear, tc.nrProtected, diff)
			}
		})
	}
}

func TestDecryptSegmentTrackIDMismatch(t *testing.T) {
	initFile := "testdata/init.mp4"
	segFile := "testdata/1.m4s"
	keyHex := "00112233445566778899aabbccddeeff"
	ivHex := "7766554433221100"
	kidHex := "11112222333344445555666677778888"
	key, _ := hex.DecodeString(keyHex)
	iv, _ := hex.DecodeString(ivHex)
	kidUUID, _ := mp4.NewUUIDFromString(kidHex)

	ifh, err := os.Open(initFile)
	if err != nil {
		t.Fatal(err)
	}
	init, err := mp4.DecodeFile(ifh)
	ifh.Close()
	if err != nil {
		t.Fatal(err)
	}

	ipf, err := mp4.InitProtect(init.Init, key, iv, "cenc", kidUUID, nil)
	if err != nil {
		t.Fatal(err)
	}

	encInitBuf := bytes.Buffer{}
	err = init.Encode(&encInitBuf)
	if err != nil {
		t.Fatal(err)
	}

	rawSeg, err := os.ReadFile(segFile)
	if err != nil {
		t.Fatal(err)
	}
	seg, err := mp4.DecodeFile(bytes.NewBuffer(rawSeg))
	if err != nil {
		t.Fatal(err)
	}
	fragIV := iv
	for _, s := range seg.Segments {
		for _, f := range s.Fragments {
			fragIV, err = mp4.EncryptFragment(f, key, fragIV, ipf)
			if err != nil {
				t.Fatal(err)
			}
		}
	}

	encBuf := bytes.Buffer{}
	err = seg.Encode(&encBuf)
	if err != nil {
		t.Fatal(err)
	}

	sr := bits.NewFixedSliceReader(encInitBuf.Bytes())
	encInit, err := mp4.DecodeFileSR(sr)
	if err != nil {
		t.Fatal(err)
	}
	decInfo, err := mp4.DecryptInit(encInit.Init)
	if err != nil {
		t.Fatal(err)
	}

	sr = bits.NewFixedSliceReader(encBuf.Bytes())
	encSeg, err := mp4.DecodeFileSR(sr)
	if err != nil {
		t.Fatal(err)
	}

	// Change trackID in the segment's traf to create a mismatch
	for _, s := range encSeg.Segments {
		for _, f := range s.Fragments {
			for _, traf := range f.Moof.Trafs {
				traf.Tfhd.TrackID = 99999
			}
		}
	}

	for _, s := range encSeg.Segments {
		err = mp4.DecryptSegment(s, decInfo, key)
		if err == nil {
			t.Fatal("expected error for mismatched trackID")
		}
	}
}

func TestDecryptSegmentWithKeys(t *testing.T) {
	// Encrypt a segment, then decrypt using KID-based key selection
	initFile := "testdata/init.mp4"
	segFile := "testdata/1.m4s"
	keyHex := "00112233445566778899aabbccddeeff"
	ivHex := "7766554433221100"
	kidHex := "11112222333344445555666677778888"
	key, _ := hex.DecodeString(keyHex)
	iv, _ := hex.DecodeString(ivHex)
	kidUUID, _ := mp4.NewUUIDFromString(kidHex)

	// Read and encrypt
	ifh, err := os.Open(initFile)
	if err != nil {
		t.Fatal(err)
	}
	initDec, err := mp4.DecodeFile(ifh)
	ifh.Close()
	if err != nil {
		t.Fatal(err)
	}
	_, err = mp4.InitProtect(initDec.Init, key, iv, "cenc", kidUUID, nil)
	if err != nil {
		t.Fatal(err)
	}
	encInitBuf := bytes.Buffer{}
	err = initDec.Encode(&encInitBuf)
	if err != nil {
		t.Fatal(err)
	}

	rawSeg, err := os.ReadFile(segFile)
	if err != nil {
		t.Fatal(err)
	}

	encryptSeg := func(t *testing.T) *mp4.File {
		t.Helper()
		seg, err := mp4.DecodeFile(bytes.NewBuffer(rawSeg))
		if err != nil {
			t.Fatal(err)
		}
		ipf, err := mp4.ExtractInitProtectData(initDec.Init)
		if err != nil {
			t.Fatal(err)
		}
		fragIV := iv
		for _, s := range seg.Segments {
			for _, f := range s.Fragments {
				fragIV, err = mp4.EncryptFragment(f, key, fragIV, ipf)
				if err != nil {
					t.Fatal(err)
				}
			}
		}
		encBuf := bytes.Buffer{}
		err = seg.Encode(&encBuf)
		if err != nil {
			t.Fatal(err)
		}
		encSeg, err := mp4.DecodeFile(&encBuf)
		if err != nil {
			t.Fatal(err)
		}
		return encSeg
	}

	sr := bits.NewFixedSliceReader(encInitBuf.Bytes())
	encInit, err := mp4.DecodeFileSR(sr)
	if err != nil {
		t.Fatal(err)
	}
	decInfo, err := mp4.DecryptInit(encInit.Init)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("matching kid decrypts via fragment", func(t *testing.T) {
		encSeg := encryptSeg(t)
		keysByKID := map[string][]byte{kidHex: key}
		for _, s := range encSeg.Segments {
			for _, f := range s.Fragments {
				err := mp4.DecryptFragmentWithKeys(f, decInfo, nil, keysByKID, true)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		}
		// Verify decrypted data matches original
		decBuf := bytes.Buffer{}
		err = encSeg.Encode(&decBuf)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(rawSeg, decBuf.Bytes()) {
			t.Error("segment not equal after KID-based decrypt")
		}
	})

	t.Run("missing kid strict mode fails", func(t *testing.T) {
		encSeg := encryptSeg(t)
		wrongKID := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		keysByKID := map[string][]byte{wrongKID: key}
		for _, s := range encSeg.Segments {
			err := mp4.DecryptSegmentWithKeys(s, decInfo, nil, keysByKID, true)
			if err == nil {
				t.Fatal("expected error for missing kid in strict mode")
			}
		}
	})

	t.Run("missing kid non-strict falls back to legacy key", func(t *testing.T) {
		encSeg := encryptSeg(t)
		wrongKID := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		keysByKID := map[string][]byte{wrongKID: key}
		for _, s := range encSeg.Segments {
			err := mp4.DecryptSegmentWithKeys(s, decInfo, key, keysByKID, false)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		}
		decBuf := bytes.Buffer{}
		err = encSeg.Encode(&decBuf)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(rawSeg, decBuf.Bytes()) {
			t.Error("segment not equal after fallback decrypt")
		}
	})
}

// TestEncryptFragmentCenc8ByteIV verifies the CMAF-conformant cenc mode:
// an 8-byte input IV gives tenc.DefaultPerSampleIVSize = 8, 8-byte
// per-sample IVs in senc, and an IV that increments by one per sample
// (ISO/IEC 23001-7 Section 9.2, ISO/IEC 23000-19 Section 8.2.3.1).
func TestEncryptFragmentCenc8ByteIV(t *testing.T) {
	key, _ := hex.DecodeString("00112233445566778899aabbccddeeff")
	iv, _ := hex.DecodeString("7766554433221100")
	kidUUID, _ := mp4.NewUUIDFromString("11112222333344445555666677778888")

	ifh, err := os.Open("testdata/init.mp4")
	if err != nil {
		t.Fatal(err)
	}
	initSeg, err := mp4.DecodeFile(ifh)
	ifh.Close()
	if err != nil {
		t.Fatal(err)
	}
	ipd, err := mp4.InitProtect(initSeg.Init, key, iv, "cenc", kidUUID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if ipd.Tenc.DefaultPerSampleIVSize != 8 {
		t.Errorf("got DefaultPerSampleIVSize %d, expected 8", ipd.Tenc.DefaultPerSampleIVSize)
	}

	rawSeg, err := os.ReadFile("testdata/1.m4s")
	if err != nil {
		t.Fatal(err)
	}
	seg, err := mp4.DecodeFile(bytes.NewBuffer(rawSeg))
	if err != nil {
		t.Fatal(err)
	}
	frag := seg.Segments[0].Fragments[0]
	nextIV, err := mp4.EncryptFragment(frag, key, iv, ipd)
	if err != nil {
		t.Fatal(err)
	}
	senc := frag.Moof.Traf.Senc
	if senc == nil || len(senc.IVs) == 0 {
		t.Fatal("missing senc IVs after encryption")
	}
	prev := binary.BigEndian.Uint64(iv)
	for i, sIV := range senc.IVs {
		if len(sIV) != 8 {
			t.Fatalf("sample %d: got %d-byte IV, expected 8", i, len(sIV))
		}
		wanted := prev + uint64(i)
		if got := binary.BigEndian.Uint64(sIV); got != wanted {
			t.Errorf("sample %d: got IV %#x, expected %#x", i, got, wanted)
		}
	}
	if len(nextIV) != 8 {
		t.Fatalf("got %d-byte next IV, expected 8", len(nextIV))
	}
	wantedNext := prev + uint64(len(senc.IVs))
	if got := binary.BigEndian.Uint64(nextIV); got != wantedNext {
		t.Errorf("got next IV %#x, expected %#x", got, wantedNext)
	}
}

// TestProtectRangesDegenerateSample verifies that a degenerate video
// sample (only a NALU length field) still yields one all-clear subsample
// entry, so that senc and saiz stay consistent within the fragment.
func TestProtectRangesDegenerateSample(t *testing.T) {
	sample := []byte{0, 0, 0, 0}
	for _, tc := range []string{"avc", "hevc"} {
		var ssps []mp4.SubSamplePattern
		var err error
		switch tc {
		case "avc":
			ssps, err = mp4.GetAVCProtectRanges(nil, nil, sample, "cenc")
		case "hevc":
			ssps, err = mp4.GetHEVCProtectRanges(nil, nil, sample, "cenc")
		}
		if err != nil {
			t.Fatalf("%s: %v", tc, err)
		}
		if len(ssps) != 1 || ssps[0].BytesOfClearData != 4 || ssps[0].BytesOfProtectedData != 0 {
			t.Errorf("%s: got %+v, expected one all-clear range of 4 bytes", tc, ssps)
		}
	}
}

// TestEncryptFragmentNoAuxInfoOmitsBoxes verifies that full-sample
// encryption with a constant IV (cbcs audio) omits the senc, saiz, and
// saio boxes, as CMAF (ISO/IEC 23000-19 Section 8.2.2.1) recommends, and
// that such fragments still decrypt.
func TestEncryptFragmentNoAuxInfoOmitsBoxes(t *testing.T) {
	key, _ := hex.DecodeString("00112233445566778899aabbccddeeff")
	iv, _ := hex.DecodeString("ffeeddccbbaa99887766554433221100")
	kidUUID, _ := mp4.NewUUIDFromString("11112222333344445555666677778888")

	ifh, err := os.Open("testdata/aac_init.mp4")
	if err != nil {
		t.Fatal(err)
	}
	initSeg, err := mp4.DecodeFile(ifh)
	ifh.Close()
	if err != nil {
		t.Fatal(err)
	}
	ipd, err := mp4.InitProtect(initSeg.Init, key, iv, "cbcs", kidUUID, nil)
	if err != nil {
		t.Fatal(err)
	}
	rawSeg, err := os.ReadFile("testdata/aac_1.m4s")
	if err != nil {
		t.Fatal(err)
	}
	seg, err := mp4.DecodeFile(bytes.NewBuffer(rawSeg))
	if err != nil {
		t.Fatal(err)
	}
	frag := seg.Segments[0].Fragments[0]
	_, err = mp4.EncryptFragment(frag, key, iv, ipd)
	if err != nil {
		t.Fatal(err)
	}
	traf := frag.Moof.Traf
	if traf.Senc != nil || traf.Saiz != nil || traf.Saio != nil {
		t.Errorf("expected senc, saiz, and saio to be omitted, got senc=%v saiz=%v saio=%v",
			traf.Senc != nil, traf.Saiz != nil, traf.Saio != nil)
	}
}

// TestCryptSampleBadSubSamplePatterns checks that subsample byte counts
// reaching outside the sample give an error instead of a panic. The counts
// come from a senc box, which nothing ties to the actual sample sizes.
func TestCryptSampleBadSubSamplePatterns(t *testing.T) {
	key := make([]byte, 16)
	iv := make([]byte, 16)
	tenc := &mp4.TencBox{DefaultCryptByteBlock: 1, DefaultSkipByteBlock: 9}
	cases := []struct {
		desc     string
		patterns []mp4.SubSamplePattern
		wantErr  bool
	}{
		{
			desc:     "protected data past end",
			patterns: []mp4.SubSamplePattern{{BytesOfClearData: 2, BytesOfProtectedData: 1000}},
			wantErr:  true,
		},
		{
			desc:     "clear data past end",
			patterns: []mp4.SubSamplePattern{{BytesOfClearData: 1000, BytesOfProtectedData: 1}},
			wantErr:  true,
		},
		{
			desc:     "byte counts wrapping uint32",
			patterns: []mp4.SubSamplePattern{{BytesOfClearData: 4, BytesOfProtectedData: 0xfffffffc}},
			wantErr:  true,
		},
		{
			desc: "second pattern past end",
			patterns: []mp4.SubSamplePattern{
				{BytesOfClearData: 2, BytesOfProtectedData: 8},
				{BytesOfClearData: 0, BytesOfProtectedData: 16},
			},
			wantErr: true,
		},
		{
			desc:     "exactly filling the sample",
			patterns: []mp4.SubSamplePattern{{BytesOfClearData: 2, BytesOfProtectedData: 8}},
			wantErr:  false,
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			for _, f := range []struct {
				name string
				fn   func(sample []byte) error
			}{
				{"CryptSampleCenc", func(s []byte) error { return mp4.CryptSampleCenc(s, key, iv, c.patterns) }},
				{"DecryptSampleCbcs", func(s []byte) error { return mp4.DecryptSampleCbcs(s, key, iv, c.patterns, tenc) }},
				{"EncryptSampleCbcs", func(s []byte) error { return mp4.EncryptSampleCbcs(s, key, iv, c.patterns, tenc) }},
			} {
				err := f.fn(make([]byte, 10))
				if c.wantErr && err == nil {
					t.Errorf("%s: expected error, got nil", f.name)
				}
				if !c.wantErr && err != nil {
					t.Errorf("%s: unexpected error: %s", f.name, err)
				}
			}
		})
	}
}

// TestDecryptFragmentSencSampleCountMismatch checks that a senc box with
// subsample information for a different number of samples than the trun
// declares gives an error instead of a panic.
func TestDecryptFragmentSencSampleCountMismatch(t *testing.T) {
	frag, err := mp4.CreateFragment(1, 1)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		frag.AddFullSample(mp4.FullSample{
			Sample:     mp4.Sample{Flags: mp4.SyncSampleFlags, Dur: 1024, Size: 16},
			DecodeTime: uint64(i * 1024),
			Data:       make([]byte, 16),
		})
	}
	// senc for one sample only, while the trun has three
	senc := mp4.CreateSencBox()
	err = senc.AddSample(mp4.SencSample{
		IV:         make([]byte, 16),
		SubSamples: []mp4.SubSamplePattern{{BytesOfClearData: 0, BytesOfProtectedData: 16}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := frag.Moof.Traf.AddChild(senc); err != nil {
		t.Fatal(err)
	}

	di := mp4.DecryptInfo{
		TrackInfos: []mp4.DecryptTrackInfo{{
			TrackID: 1,
			Sinf: &mp4.SinfBox{
				Schm: &mp4.SchmBox{SchemeType: "cenc"},
				Schi: &mp4.SchiBox{Tenc: &mp4.TencBox{
					DefaultPerSampleIVSize: 16,
					DefaultKID:             mp4.UUID(make([]byte, 16)),
				}},
			},
		}},
	}
	if err := mp4.DecryptFragment(frag, di, make([]byte, 16)); err == nil {
		t.Error("expected error for senc subsample count mismatch, got nil")
	}
}

// TestProtectRangesBadNaluLength checks that a nalu length field that wraps in
// uint32 is reported instead of slicing outside the sample.
func TestProtectRangesBadNaluLength(t *testing.T) {
	samples := map[string][]byte{
		"length wrapping uint32": {0xff, 0xff, 0xff, 0xff, 0x65, 0, 0, 0},
		"length beyond sample":   {0, 0, 0x10, 0, 0x65, 0, 0, 0},
	}
	for desc, sample := range samples {
		t.Run(desc, func(t *testing.T) {
			if _, err := mp4.GetAVCProtectRanges(nil, nil, sample, "cenc"); err == nil {
				t.Error("GetAVCProtectRanges: expected error, got nil")
			}
			if _, err := mp4.GetHEVCProtectRanges(nil, nil, sample, "cenc"); err == nil {
				t.Error("GetHEVCProtectRanges: expected error, got nil")
			}
		})
	}
}

// TestEncryptDecryptManySubsamples encrypts a sample with so many subsamples
// that its sample auxiliary information exceeds 255 bytes, which needs a
// version 1 saiz, and checks that it decrypts after being written and read.
func TestEncryptDecryptManySubsamples(t *testing.T) {
	key, _ := hex.DecodeString("00112233445566778899aabbccddeeff")
	iv, _ := hex.DecodeString("7766554433221100")
	kidUUID, _ := mp4.NewUUIDFromString("11112222333344445555666677778888")

	ifh, err := os.Open("testdata/init.mp4")
	if err != nil {
		t.Fatal(err)
	}
	initFile, err := mp4.DecodeFile(ifh)
	ifh.Close()
	if err != nil {
		t.Fatal(err)
	}
	init := initFile.Init
	ipd, err := mp4.InitProtect(init, key, iv, "cenc", kidUUID, nil)
	if err != nil {
		t.Fatal(err)
	}

	// 50 IDR slice NAL units, each large enough to get its own protected range.
	const nrNalus, naluSize = 50, 300
	var sample []byte
	for i := range nrNalus {
		nalu := make([]byte, 4+naluSize)
		binary.BigEndian.PutUint32(nalu, naluSize)
		nalu[4] = 0x65 // IDR slice
		for j := 5; j < len(nalu); j++ {
			nalu[j] = byte(i + j)
		}
		sample = append(sample, nalu...)
	}
	plain := slices.Clone(sample)

	seg := mp4.NewMediaSegment()
	frag, err := mp4.CreateFragment(1, init.Moov.Trak.Tkhd.TrackID)
	if err != nil {
		t.Fatal(err)
	}
	seg.AddFragment(frag)
	frag.AddFullSample(mp4.FullSample{
		Sample: mp4.Sample{Flags: mp4.SyncSampleFlags, Dur: 1000, Size: uint32(len(sample))}, Data: sample,
	})
	if _, err := mp4.EncryptFragment(frag, key, iv, ipd); err != nil {
		t.Fatal(err)
	}
	saiz := frag.Moof.Traf.Saiz
	wantSize := uint32(8 + 2 + nrNalus*6)
	if saiz.Version != 1 || saiz.DefaultSampleInfoSize != wantSize {
		t.Fatalf("got saiz version %d with size %d, want version 1 with size %d",
			saiz.Version, saiz.DefaultSampleInfoSize, wantSize)
	}

	var initBuf, segBuf bytes.Buffer
	if err := init.Encode(&initBuf); err != nil {
		t.Fatal(err)
	}
	if err := seg.Encode(&segBuf); err != nil {
		t.Fatal(err)
	}
	encInit, err := mp4.DecodeFileSR(bits.NewFixedSliceReader(initBuf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	decInfo, err := mp4.DecryptInit(encInit.Init)
	if err != nil {
		t.Fatal(err)
	}
	encSeg, err := mp4.DecodeFileSR(bits.NewFixedSliceReader(segBuf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	decFrag := encSeg.Segments[0].Fragments[0]
	if v := decFrag.Moof.Traf.Saiz.Version; v != 1 {
		t.Errorf("decoded saiz version %d, want 1", v)
	}
	if err := mp4.DecryptFragment(decFrag, decInfo, key); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decFrag.Mdat.Data, plain) {
		t.Error("decrypted sample differs from the original")
	}
}

// clearSequence returns the AVC init segment of the test data and its 60 samples re-fragmented into fragments of
// samplesPerFrag samples, with a copy of the clear sample data.
func clearSequence(t *testing.T, samplesPerFrag int) (*mp4.InitSegment, []*mp4.Fragment, [][]byte) {
	t.Helper()
	initFile, err := mp4.ReadMP4File("testdata/init.mp4")
	if err != nil {
		t.Fatal(err)
	}
	segFile, err := mp4.ReadMP4File("testdata/1.m4s")
	if err != nil {
		t.Fatal(err)
	}
	trex := initFile.Init.Moov.Mvex.Trex
	fss, err := segFile.Segments[0].Fragments[0].GetFullSamples(trex)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	var clear [][]byte
	for start := 0; start < len(fss); start += samplesPerFrag {
		end := min(start+samplesPerFrag, len(fss))
		frag, err := mp4.CreateFragment(uint32(start/samplesPerFrag+1), trex.TrackID)
		if err != nil {
			t.Fatal(err)
		}
		frag.AddFullSamples(fss[start:end])
		if err := frag.Encode(&buf); err != nil {
			t.Fatal(err)
		}
		for _, fs := range fss[start:end] {
			clear = append(clear, bytes.Clone(fs.Data))
		}
	}
	// Decode the fragments, since the encryptor works on decoded fragments
	f, err := mp4.DecodeFileSR(bits.NewFixedSliceReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	return initFile.Init, f.Segments[0].Fragments, clear
}

// addToIV returns iv plus n as a big-endian integer, wrapping around.
func addToIV(iv []byte, n uint64) []byte {
	out := bytes.Clone(iv)
	for i := len(out) - 1; i >= 0 && n > 0; i-- {
		sum := uint64(out[i]) + n&0xff
		out[i] = byte(sum)
		n = n>>8 + sum>>8
	}
	return out
}

// TestFragmentEncryptorMatchesSampleFunctions encrypts a sequence of fragments with one FragmentEncryptor and
// then checks every sample against CryptSampleCenc or EncryptSampleCbcs of the clear sample with the IV and
// subsamples in senc. It also checks that the cenc IVs follow on from each other across fragments. Since the
// encryptor reuses its storage from fragment to fragment, the checks start only when all fragments are encrypted.
func TestFragmentEncryptorMatchesSampleFunctions(t *testing.T) {
	key, _ := hex.DecodeString("00112233445566778899aabbccddeeff")
	kid, _ := mp4.NewUUIDFromString("11112222333344445555666677778888")
	iv8, _ := hex.DecodeString("77665544332211fe") // the low byte overflows after two samples
	iv16, _ := hex.DecodeString("ffeeddccbbaa99887766554433221100")
	for _, sc := range []struct {
		scheme string
		iv     []byte
	}{{"cenc", iv8}, {"cenc", iv16}, {"cbcs", iv16}} {
		for _, samplesPerFrag := range []int{1, 7, 60} {
			name := fmt.Sprintf("%s %d-byte IV, %d samples per fragment", sc.scheme, len(sc.iv), samplesPerFrag)
			t.Run(name, func(t *testing.T) {
				init, frags, clear := clearSequence(t, samplesPerFrag)
				ipd, err := mp4.InitProtect(init, key, sc.iv, sc.scheme, kid, nil)
				if err != nil {
					t.Fatal(err)
				}
				e, err := ipd.NewFragmentEncryptor(key, sc.iv)
				if err != nil {
					t.Fatal(err)
				}
				for _, frag := range frags {
					if err := e.EncryptFragment(frag); err != nil {
						t.Fatal(err)
					}
				}
				nr := 0
				expIV := sc.iv
				for _, frag := range frags {
					fss, err := frag.GetFullSamples(ipd.Trex)
					if err != nil {
						t.Fatal(err)
					}
					senc := frag.Moof.Traf.Senc
					if len(senc.SubSamples) != len(fss) {
						t.Fatalf("senc has %d subsample entries for %d samples", len(senc.SubSamples), len(fss))
					}
					for i, fs := range fss {
						want := bytes.Clone(clear[nr])
						subSamples := senc.SubSamples[i]
						switch sc.scheme {
						case "cenc":
							iv := []byte(senc.IVs[i])
							if !bytes.Equal(iv, expIV) {
								t.Fatalf("sample %d: IV %x, want %x", nr+1, iv, expIV)
							}
							ctrIV := make([]byte, 16)
							copy(ctrIV, iv)
							if err := mp4.CryptSampleCenc(want, key, ctrIV, subSamples); err != nil {
								t.Fatal(err)
							}
							if len(iv) == 8 {
								expIV = addToIV(iv, 1)
							} else {
								nrBlocks := 0
								for _, ss := range subSamples {
									nrBlocks += int(ss.BytesOfProtectedData / 16)
								}
								expIV = addToIV(iv, uint64(nrBlocks))
							}
						case "cbcs":
							if len(senc.IVs) != 0 {
								t.Fatalf("cbcs senc has %d IVs, want none", len(senc.IVs))
							}
							if err := mp4.EncryptSampleCbcs(want, key, sc.iv, subSamples, ipd.Tenc); err != nil {
								t.Fatal(err)
							}
						}
						if !bytes.Equal(fs.Data, want) {
							t.Fatalf("sample %d is not encrypted as its senc entry says", nr+1)
						}
						nr++
					}
				}
				if nr != len(clear) {
					t.Fatalf("checked %d samples, want %d", nr, len(clear))
				}
				if sc.scheme == "cenc" && !bytes.Equal(e.IV(), expIV) {
					t.Errorf("next IV %x, want %x", e.IV(), expIV)
				}
			})
		}
	}
}

// TestFragmentEncryptorIV checks that IV returns a copy, which neither changes when the encryptor encrypts
// more fragments nor changes the encryptor when the caller changes it.
func TestFragmentEncryptorIV(t *testing.T) {
	key, _ := hex.DecodeString("00112233445566778899aabbccddeeff")
	kid, _ := mp4.NewUUIDFromString("11112222333344445555666677778888")
	iv, _ := hex.DecodeString("7766554433221100")
	init, frags, _ := clearSequence(t, 30)
	ipd, err := mp4.InitProtect(init, key, iv, "cenc", kid, nil)
	if err != nil {
		t.Fatal(err)
	}
	e, err := ipd.NewFragmentEncryptor(key, iv)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.EncryptFragment(frags[0]); err != nil {
		t.Fatal(err)
	}
	ivAfterFirst := e.IV()
	saved := bytes.Clone(ivAfterFirst)
	ivAfterFirst[0] ^= 0xff
	if err := e.EncryptFragment(frags[1]); err != nil {
		t.Fatal(err)
	}
	if got := []byte(frags[1].Moof.Traf.Senc.IVs[0]); !bytes.Equal(got, saved) {
		t.Errorf("second fragment starts with IV %x, want %x", got, saved)
	}
	ivAfterFirst[0] ^= 0xff
	if !bytes.Equal(ivAfterFirst, saved) {
		t.Errorf("IV returned after the first fragment changed to %x from %x", ivAfterFirst, saved)
	}
	if bytes.Equal(e.IV(), saved) {
		t.Error("IV did not advance in the second fragment")
	}
}

// TestFragmentEncryptorAllocations checks that cbcs encryption of a fragment allocates less than once per sample.
// It used to allocate three times per sample, and about ten times before avc.SliceHeaderSize.
func TestFragmentEncryptorAllocations(t *testing.T) {
	key, _ := hex.DecodeString("00112233445566778899aabbccddeeff")
	kid, _ := mp4.NewUUIDFromString("11112222333344445555666677778888")
	iv, _ := hex.DecodeString("ffeeddccbbaa99887766554433221100")
	init, err := mp4.ReadMP4File("testdata/init.mp4")
	if err != nil {
		t.Fatal(err)
	}
	ipd, err := mp4.InitProtect(init.Init, key, iv, "cbcs", kid, nil)
	if err != nil {
		t.Fatal(err)
	}
	e, err := ipd.NewFragmentEncryptor(key, iv)
	if err != nil {
		t.Fatal(err)
	}
	seg, err := os.ReadFile("testdata/1.m4s")
	if err != nil {
		t.Fatal(err)
	}
	work := make([]byte, len(seg))
	decode := func() *mp4.Fragment {
		copy(work, seg) // encryption is in place
		f, err := mp4.DecodeFileSR(bits.NewFixedSliceReader(work))
		if err != nil {
			t.Fatal(err)
		}
		return f.Segments[0].Fragments[0]
	}
	nrSamples := int(decode().Moof.Traf.Trun.SampleCount())
	decodeAllocs := testing.AllocsPerRun(10, func() { decode() })
	allocs := testing.AllocsPerRun(10, func() {
		if err := e.EncryptFragment(decode()); err != nil {
			t.Fatal(err)
		}
	})
	if encAllocs := allocs - decodeAllocs; encAllocs >= float64(nrSamples) {
		t.Errorf("encrypting %d samples made %.0f allocations, want fewer than one per sample", nrSamples, encAllocs)
	}
}
