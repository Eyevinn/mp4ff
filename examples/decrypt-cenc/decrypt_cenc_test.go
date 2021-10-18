package main

import "testing"

func TestDecodeCenc(t *testing.T) {
	inFile := "testdata/prog_8s_enc_dashinit.mp4"
	outFile := "testdata/dec.mp4"
	hexString := "63cb5f7184dd4b689a5c5ff11ee6a328"
	err := start(inFile, outFile, hexString)
	if err != nil {
		t.Error(err)
	}
}
