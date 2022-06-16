package main

import (
	"bytes"
	"io/ioutil"
	"testing"

	"github.com/go-test/deep"
)

func TestDecryptFiles(t *testing.T) {
	testCases := []struct {
		name            string
		inFile          string
		expectedOutFile string
		hexKey          string
	}{
		{
			name:            "cenc",
			inFile:          "../../mp4/testdata/prog_8s_enc_dashinit.mp4",
			expectedOutFile: "../../mp4/testdata/prog_8s_dec_dashinit.mp4",
			hexKey:          "63cb5f7184dd4b689a5c5ff11ee6a328",
		},
		{
			name:            "cbcs",
			inFile:          "../../mp4/testdata/cbcs.mp4",
			expectedOutFile: "../../mp4/testdata/cbcsdec.mp4",
			hexKey:          "22bdb0063805260307ee5045c0f3835a",
		},
		{
			name:            "cbcs audio",
			inFile:          "../../mp4/testdata/cbcs_audio.mp4",
			expectedOutFile: "../../mp4/testdata/cbcs_audiodec.mp4",
			hexKey:          "5ffd93861fa776e96cccd934898fc1c8",
		},
	}

	for _, tc := range testCases {
		raw, err := ioutil.ReadFile(tc.inFile)
		if err != nil {
			t.Error(err)
		}
		inBuf := bytes.NewBuffer(raw)
		buf := bytes.Buffer{}
		err = start(inBuf, &buf, tc.hexKey)
		if err != nil {
			t.Error(err)
		}
		expectedOut, err := ioutil.ReadFile(tc.expectedOutFile)
		if err != nil {
			t.Error(err)
		}
		gotOut := buf.Bytes()
		diff := deep.Equal(expectedOut, gotOut)
		if diff != nil {
			t.Errorf("Mismatch for case %s: %s", tc.name, diff)
		}
	}

}

func BenchmarkDecodeCenc(b *testing.B) {
	inFile := "../../mp4/testdata/prog_8s_enc_dashinit.mp4"
	hexKey := "63cb5f7184dd4b689a5c5ff11ee6a328"
	raw, err := ioutil.ReadFile(inFile)
	if err != nil {
		b.Error(err)
	}
	outData := make([]byte, 0, len(raw))
	outBuf := bytes.NewBuffer(outData)
	for i := 0; i < b.N; i++ {
		inBuf := bytes.NewBuffer(raw)
		outBuf.Reset()
		err = start(inBuf, outBuf, hexKey)
		if err != nil {
			b.Error(err)
		}
	}
}
