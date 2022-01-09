package avc

import (
	"encoding/hex"
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
}
