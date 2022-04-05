package hevc

import (
	"encoding/hex"
	"strings"
	"testing"
)

func TestCreateDecConfRec(t *testing.T) {
	testCases := []struct {
		vpsHex    string
		spsHex    string
		ppsHex    string
		complete  bool
		includePS bool
		errorMsg  string
	}{
		{
			vpsHex:    "",
			spsHex:    "",
			ppsHex:    "",
			complete:  false,
			includePS: false,
			errorMsg:  "no SPS NALU supported. Needed to extract fundamental information",
		},
		{
			vpsHex:    "0000000140010c01ffff016000000300900000030000030078959809",
			spsHex:    "00000001420101016000000300900000030000030078a00502016965959a4932bc05a80808082000000300200000030321",
			ppsHex:    "000000014401c172b46240",
			complete:  true,
			includePS: true,
			errorMsg:  "",
		},
	}
	for _, tc := range testCases {
		vpsNalus := createNalusFromHex(tc.vpsHex)
		spsNalus := createNalusFromHex(tc.spsHex)
		ppsNalus := createNalusFromHex(tc.ppsHex)
		dcr, err := CreateHEVCDecConfRec(vpsNalus, spsNalus, ppsNalus,
			tc.complete, tc.complete, tc.complete, tc.includePS)
		if tc.errorMsg != "" {
			if err.Error() != tc.errorMsg {
				t.Errorf("got error %q instead of %q", err.Error(), tc.errorMsg)
			}
		} else {
			if len(dcr.NaluArrays) != 3 {
				t.Errorf("got %d NALU arrays instead of 3", len(dcr.NaluArrays))
			}
		}
	}
}

func createNalusFromHex(hexStr string) [][]byte {
	if hexStr == "" {
		return nil
	}
	startCode := "00000001"
	parts := strings.Split(hexStr, startCode)
	nalusHex := parts[1:]
	nalus := make([][]byte, 0, len(nalusHex))
	for _, nx := range nalusHex {
		nalu, _ := hex.DecodeString(nx)
		nalus = append(nalus, nalu)
	}
	return nalus
}
