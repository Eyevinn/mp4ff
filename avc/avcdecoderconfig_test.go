package avc

import (
	"bytes"
	"encoding/hex"
	"os"
	"testing"

	"github.com/go-test/deep"
)

const avcDecoderConfigRecord = "0164001effe100196764001eacd940a02ff9610000030001000003003c8f162d9601000568ebecb22cfdf8f800"
const sps = "6764001eacd940a02ff9610000030001000003003c8f162d96"
const pps = "68ebecb22c"

func TestAvcDecoderConfigRecord(t *testing.T) {
	byteData, _ := hex.DecodeString(avcDecoderConfigRecord)
	spsBytes, _ := hex.DecodeString(sps)
	ppsBytes, _ := hex.DecodeString(pps)

	wanted := DecConfRec{
		AVCProfileIndication: 100,
		ProfileCompatibility: 0,
		AVCLevelIndication:   30,
		SPSnalus:             [][]byte{spsBytes},
		PPSnalus:             [][]byte{ppsBytes},
		ChromaFormat:         1,
		BitDepthLumaMinus1:   0,
		BitDepthChromaMinus1: 0,
		NumSPSExt:            0,
	}

	got, err := DecodeAVCDecConfRec(byteData)
	if err != nil {
		t.Error("Error parsing AVCDecoderConfigurationRecord")
	}
	if diff := deep.Equal(got, wanted); diff != nil {
		t.Error(diff)
	}

	enc := bytes.Buffer{}
	err = got.Encode(&enc)
	if err != nil {
		t.Error("Error encoding AVCDecoderConfigurationRecord")
	}
	if !bytes.Equal(enc.Bytes(), byteData) {
		t.Error("Error encoding AVCDecoderConfigurationRecord")
	}
}

func TestCreateAVCDecConfRec(t *testing.T) {
	data, err := os.ReadFile("testdata/blackframe.264")
	if err != nil {
		t.Error("Error reading file")
	}
	spss := ExtractNalusOfTypeFromByteStream(NALU_SPS, data, true)
	ppss := ExtractNalusOfTypeFromByteStream(NALU_PPS, data, true)
	if len(spss) != 1 || len(ppss) != 1 {
		t.Error("Error extracting SPS/PPS")
	}
	_, err = CreateAVCDecConfRec(spss, ppss, true)
	if err != nil {
		t.Error("Error creating AVCDecoderConfigurationRecord")
	}
}
