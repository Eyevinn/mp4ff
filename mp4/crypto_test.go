package mp4

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
	"testing"

	"github.com/Eyevinn/mp4ff/avc"
	"github.com/Eyevinn/mp4ff/bits"
	"github.com/go-test/deep"
)

func TestFindAVCSubsampleRanges(t *testing.T) {
	infile := "testdata/1.m4s"
	fmp4, err := ReadMP4File(infile)
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
		expectedRanges [][]SubSamplePattern
	}{
		{
			scheme: "cenc",
			expectedRanges: [][]SubSamplePattern{
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
			expectedRanges: [][]SubSamplePattern{
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
			protectRanges, err := GetAVCProtectRanges(spsMap, ppsMap, data, tc.scheme)
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
	videoInit := "testdata/init.mp4"
	videoSeg := "testdata/1.m4s"
	audioInit := "testdata/aac_init.mp4"
	audioSeg := "testdata/aac_1.m4s"
	keyHex := "00112233445566778899aabbccddeeff"
	ivHex8 := "7766554433221100"
	ivHex16 := "ffeeddccbbaa99887766554433221100"
	kidHex := "11112222333344445555666677778888"
	key, _ := hex.DecodeString(keyHex)
	kidUUID, _ := NewUUIDFromHex(kidHex)

	testCases := []struct {
		desc   string
		init   string
		seg    string
		scheme string
		iv     string
	}{
		{desc: "video, cenc, iv8", init: videoInit, seg: videoSeg, scheme: "cenc", iv: ivHex8},
		{desc: "video, cbcs, iv8", init: videoInit, seg: videoSeg, scheme: "cbcs", iv: ivHex8},
		{desc: "video, cbcs, iv16", init: videoInit, seg: videoSeg, scheme: "cbcs", iv: ivHex16},
		{desc: "audio, cbcs, iv16", init: audioInit, seg: audioSeg, scheme: "cbcs", iv: ivHex16},
	}
	for _, c := range testCases {
		t.Run(c.desc, func(t *testing.T) {
			ifh, err := os.Open(c.init)
			if err != nil {
				t.Fatal(err)
			}
			init, err := DecodeFile(ifh)
			ifh.Close()
			if err != nil {
				t.Fatal(err)
			}
			iv, err := hex.DecodeString(c.iv)
			if err != nil {
				t.Fatal(err)
			}
			ipf, err := InitProtect(init.Init, key, iv, c.scheme, kidUUID, nil)
			if err != nil {
				t.Fatal(err)
			}
			// Write init segment with encyption info
			encInitBuf := bytes.Buffer{}
			err = init.Encode(&encInitBuf)
			if err != nil {
				t.Fatal(err)
			}

			// Check that one can extract the protection the InitProtectData from the init segment
			ipd, err := ExtractInitProtectData(init.Init)
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
			seg, err := DecodeFile(rs)
			if err != nil {
				t.Fatal(err)
			}
			for _, s := range seg.Segments {
				for _, f := range s.Fragments {
					err := EncryptFragment(f, key, iv, ipf)
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
			encInit, err := DecodeFileSR(sr)
			if err != nil {
				t.Error(err)
			}
			decInfo, err := DecryptInit(encInit.Init)
			if err != nil {
				t.Error(err)
			}

			// Decode and decrypt the written segment
			sr = bits.NewFixedSliceReader(outBuf.Bytes())
			decode, err := DecodeFileSR(sr)
			if err != nil {
				t.Error(err)
			}
			// Decrypt the segment
			for _, s := range decode.Segments {
				err := DecryptSegment(s, decInfo, key)
				if err != nil {
					t.Error(err)
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
		})
	}
}

func TestDecryptInit(t *testing.T) {
	encFile := "testdata/prog_8s_enc_dashinit.mp4"
	mp4f, err := ReadMP4File(encFile)
	if err != nil {
		t.Error(err)
	}
	init := mp4f.Init
	decInfo, err := DecryptInit(init)
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
