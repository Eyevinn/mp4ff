package avc

import (
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/go-test/deep"
)

const videoNaluStart = "25888040ffde08e47a7bff05ab"

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

func TestParseSliceHeader(t *testing.T) {
	wantedHdr := SliceHeader{
		SliceType:              7,
		SliceQPDelta:           6,
		SliceAlphaC0OffsetDiv2: -3,
		SliceBetaOffsetDiv2:    -3,
		Size:                   8,
	}
	data, err := ioutil.ReadFile("testdata/blackframe.264")
	if err != nil {
		t.Error(err)
	}
	nalus := ExtractNalusFromByteStream(data)
	spsMap := make(map[uint32]*SPS, 1)
	ppsMap := make(map[uint32]*PPS, 1)
	var gotHdr *SliceHeader
	for _, nalu := range nalus {
		switch GetNaluType(nalu[0]) {
		case NALU_SPS:
			sps, err := ParseSPSNALUnit(nalu, true)
			if err != nil {
				t.Error(err)
			}
			spsMap[uint32(sps.ParameterID)] = sps
		case NALU_PPS:
			pps, err := ParsePPSNALUnit(nalu, spsMap)
			if err != nil {
				t.Error(err)
			}
			ppsMap[uint32(pps.PicParameterSetID)] = pps
		case NALU_IDR:
			gotHdr, err = ParseSliceHeader(nalu, spsMap, ppsMap)
			if err != nil {
				t.Error(err)
			}
		}
	}
	if diff := deep.Equal(wantedHdr, *gotHdr); diff != nil {
		fmt.Printf("Got Slice Header: %+v\n Diff is: ", *gotHdr)
		t.Error(diff)
	}
}
