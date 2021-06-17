package avc

import (
	"encoding/hex"
	"testing"
)

const (
	// Slice Type Test Data
	videoNaluStart = "25888040ffde08e47a7bff05ab"
	// IDR Test Data
	videoSliceDataIDR = "6588840B5B07C341"
	SPSIDRTest        = "674d4028d900780227e59a808080a000000300c0000023c1e30649"
	PPSIDRTest        = "68ebc08cf2"
	// P Frame Test Data
	videoSliceDataPFrame = "419A384603FA42D6FFB5F01137F156003C"
	SPSPFrameTest        = "674d4028d900780227e59a808080a000000300c0000023c1e30649"
	PPSPFrameTest        = "68ebc08cf2"
	// P Frame Encrypted Slice Data Test - slice data (after slice header is encrypted)
	videoSliceDataPFrameEnc = "419A384603FA42D6FF62ADEB"
)

func TestSliceTypeParser(t *testing.T) {
	byteData, _ := hex.DecodeString(videoNaluStart)
	want := SLICE_I
	got, err := GetSliceTypeFromNALU(byteData)
	if err != nil {
		t.Error(err)
	}
	if got != want {
		t.Errorf("got %s want %s", got, want)
	}
}

func TestSliceHeaderParserIDR(t *testing.T) {
	// SPS needed to parse PPS and Slice Header
	spsData, _ := hex.DecodeString(SPSIDRTest)
	sps, err := ParseSPSNALUnit(spsData, false)
	if err != nil {
		t.Errorf("Parse IDR Failed to parse SPS")
	}
	// PPS needed to Parse Slice Header
	ppsData, _ := hex.DecodeString(PPSIDRTest)
	pps, err := ParsePPSNALUnit(ppsData, sps)
	if err != nil {
		t.Errorf("Parse IDR Failed to parse PPS")
	}

	byteData, _ := hex.DecodeString(videoSliceDataIDR) // Actual slice header data
	_, _, err = ParseSliceHeader(byteData, sps, pps)
	if err != nil {
		t.Error(err)
	}
}

func TestSliceHeaderParserPFrame(t *testing.T) {
	// SPS needed to parse PPS and Slice Header
	spsData, _ := hex.DecodeString(SPSPFrameTest)
	sps, err := ParseSPSNALUnit(spsData, false)
	if err != nil {
		t.Errorf("Parse PFrame Failed to parse SPS")
	}
	// PPS needed to Parse Slice Header
	ppsData, _ := hex.DecodeString(PPSPFrameTest)
	pps, err := ParsePPSNALUnit(ppsData, sps)
	if err != nil {
		t.Errorf("Parse PFrame Failed to parse PPS")
	}
	// Actual slice header data plus unencrypted slice data
	byteData, _ := hex.DecodeString(videoSliceDataPFrame)
	_, _, err = ParseSliceHeader(byteData, sps, pps)
	if err != nil {
		t.Error(err)
	}
}

func TestSliceHeaderParserPFrameEnc(t *testing.T) {
	// SPS needed to parse PPS and Slice Header
	spsData, _ := hex.DecodeString(SPSPFrameTest)
	sps, err := ParseSPSNALUnit(spsData, false)
	if err != nil {
		t.Errorf("Parse PFrame Failed to parse SPS")
	}
	// PPS needed to Parse Slice Header
	ppsData, _ := hex.DecodeString(PPSPFrameTest)
	pps, err := ParsePPSNALUnit(ppsData, sps)
	if err != nil {
		t.Errorf("Parse PFrame Failed to parse PPS")
	}
	// Actual slice header plus encrypted slice data
	byteData, _ := hex.DecodeString(videoSliceDataPFrameEnc)
	_, _, err = ParseSliceHeader(byteData, sps, pps)
	if err != nil {
		t.Error(err)
	}
}

// Test coverage that needs to be added - B slice, SP & SI slices, Ref Pic List modification
