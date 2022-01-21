package main

import (
	"bytes"
	"io/ioutil"
	"testing"

	"github.com/go-test/deep"
)

func TestDecodeCenc(t *testing.T) {
	inFile := "../../mp4/testdata/prog_8s_enc_dashinit.mp4"
	expectedOutFile := "../../mp4/testdata/prog_8s_dec_dashinit.mp4"
	hexString := "63cb5f7184dd4b689a5c5ff11ee6a328"
	raw, err := ioutil.ReadFile(inFile)
	if err != nil {
		t.Error(err)
	}
	inBuf := bytes.NewBuffer(raw)
	buf := bytes.Buffer{}
	err = start(inBuf, &buf, hexString)
	if err != nil {
		t.Error(err)
	}
	expectedOut, err := ioutil.ReadFile(expectedOutFile)
	if err != nil {
		t.Error(err)
	}
	gotOut := buf.Bytes()
	diff := deep.Equal(expectedOut, gotOut)
	if diff != nil {
		t.Errorf("Mismatch: %s", diff)
	}
}

func BenchmarkDecodeCenc(b *testing.B) {
	inFile := "../../mp4/testdata/prog_8s_enc_dashinit.mp4"
	hexString := "63cb5f7184dd4b689a5c5ff11ee6a328"
	raw, err := ioutil.ReadFile(inFile)
	if err != nil {
		b.Error(err)
	}
	for i := 0; i < b.N; i++ {
		inBuf := bytes.NewBuffer(raw)
		outBuf := bytes.Buffer{}
		err = start(inBuf, &outBuf, hexString)
		if err != nil {
			b.Error(err)
		}
	}
}
