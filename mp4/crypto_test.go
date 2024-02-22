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

func TestEncryptDecryptAVC(t *testing.T) {
	testInit := "testdata/init.mp4"
	testFile := "testdata/1.m4s"
	keyHex := "00112233445566778899aabbccddeeff"
	ivHex := "7766554433221100"
	kidHex := "11112222333344445555666677778888"
	key, _ := hex.DecodeString(keyHex)
	iv, _ := hex.DecodeString(ivHex)
	kidUUID, _ := NewUUIDFromHex(kidHex)

	if len(iv) == 8 {
		// Convert to 16 bytes
		iv8 := iv
		iv = make([]byte, 16)
		copy(iv, iv8)
	}

	testCases := []struct {
		scheme string
	}{
		{scheme: "cenc"},
		{scheme: "cbcs"},
	}

	for _, tc := range testCases {
		ifh, err := os.Open(testInit)
		if err != nil {
			t.Fatal(err)
		}
		init, err := DecodeFile(ifh)
		if err != nil {
			t.Fatal(err)
		}
		ifh.Close()
		ipf, err := InitProtect(init.Init, key, iv, tc.scheme, kidUUID, nil)
		if err != nil {
			t.Fatal(err)
		}
		ifh, err = os.Open(testFile)
		if err != nil {
			t.Fatal(err)
		}
		segFile, err := DecodeFile(ifh)
		if err != nil {
			t.Fatal(err)
		}
		dInfo, err := DecryptInit(init.Init)
		if err != nil {
			t.Fatal(err)
		}
		ifh.Close()
		for _, s := range segFile.Segments {
			for _, f := range s.Fragments {
				rawInput := make([]byte, len(f.Mdat.Data))
				copy(rawInput, f.Mdat.Data)
				err := EncryptFragment(f, key, iv, ipf)
				if err != nil {
					t.Error(err)
				}
				outBuf := bytes.Buffer{}
				err = f.Encode(&outBuf)
				if err != nil {
					t.Error(err)
				}
				sr := bits.NewFixedSliceReader(outBuf.Bytes())
				dff, err := DecodeFileSR(sr)
				if err != nil {
					t.Error(err)
				}
				if len(dff.Segments) != 1 {
					t.Errorf("Expected 1 segment, got %d", len(dff.Segments))
				}
				if len(dff.Segments[0].Fragments) != 1 {
					t.Errorf("Expected 1 fragment, got %d", len(dff.Segments[0].Fragments))
				}
				df := dff.Segments[0].Fragments[0]
				encData := make([]byte, len(df.Mdat.Data))
				copy(encData, df.Mdat.Data)
				if bytes.Equal(rawInput, encData) {
					t.Errorf("bytes equal after encryption")
				}
				err = DecryptFragment(df, dInfo, key)
				if err != nil {
					t.Error(err)
				}
				decData := make([]byte, len(df.Mdat.Data))
				copy(decData, df.Mdat.Data)
				if !bytes.Equal(rawInput, decData) {
					t.Errorf("bytes not equal after encryption+decryption")
				}
			}
		}
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
