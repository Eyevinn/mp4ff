package avc

import (
	"encoding/hex"
	"fmt"
	"os"
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

func TestSliceTypeStrings(t *testing.T) {
	cases := []struct {
		sliceType SliceType
		want      string
	}{
		{SLICE_P, "P"},
		{SLICE_B, "B"},
		{SLICE_I, "I"},
		{SLICE_SP, "SP"},
		{SLICE_SI, "SI"},
		{SliceType(12), ""},
	}
	for _, c := range cases {
		got := c.sliceType.String()
		if got != c.want {
			t.Errorf("got %s want %s", got, c.want)
		}
	}

}

func TestParseSliceHeader_BlackFrame(t *testing.T) {
	wantedHdr := SliceHeader{
		SliceType:              7,
		SliceQPDelta:           6,
		SliceAlphaC0OffsetDiv2: -3,
		SliceBetaOffsetDiv2:    -3,
		Size:                   7,
	}
	data, err := os.ReadFile("testdata/blackframe.264")
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
		t.Errorf("Got slice header %+v. Diff=%v", *gotHdr, diff)
	}
}

func TestParseSliceHeader_TwoFrames(t *testing.T) {
	wantedIdrHdr := SliceHeader{SliceType: SLICE_I, IDRPicID: 1, SliceQPDelta: 8, Size: 5}
	wantedNonIdrHdr := SliceHeader{
		SliceType: SLICE_P, FrameNum: 1, ModificationOfPicNumsIDC: 3, SliceQPDelta: 13,
		Size: 5, NumRefIdxActiveOverrideFlag: true, RefPicListModificationL0Flag: true,
	}

	data, err := os.ReadFile("testdata/two-frames.264")
	if err != nil {
		t.Error(err)
	}
	nalus, err := GetNalusFromSample(data)
	if err != nil {
		t.Error(err)
	}
	spsMap := make(map[uint32]*SPS, 1)
	ppsMap := make(map[uint32]*PPS, 1)
	var gotIdrHdr *SliceHeader
	var gotNonIdrHdr *SliceHeader
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
			gotIdrHdr, err = ParseSliceHeader(nalu, spsMap, ppsMap)
			if err != nil {
				t.Error(err)
			}
		case NALU_NON_IDR:
			gotNonIdrHdr, err = ParseSliceHeader(nalu, spsMap, ppsMap)
			if err != nil {
				t.Error(err)
			}
		}
	}
	if diff := deep.Equal(wantedIdrHdr, *gotIdrHdr); diff != nil {
		t.Errorf("Got IDR Slice Header: %+v\n Diff is: %v", *gotIdrHdr, diff)
	}
	if diff := deep.Equal(wantedNonIdrHdr, *gotNonIdrHdr); diff != nil {
		t.Errorf("Got NON_IDR Slice Header: %+v\n Diff is: %v", *gotNonIdrHdr, diff)
	}
}

func TestParseSliceHeaderLength(t *testing.T) {
	spsHex := "6764001eacd940a02ff9610000030001000003003c8f162d96"
	ppsHex := "68ebecb22c"
	naluStartHex := "419a6649e10f2653022fff8700000302c8a32d32"
	spsData, _ := hex.DecodeString(spsHex)
	sps, err := ParseSPSNALUnit(spsData, true)
	if err != nil {
		t.Error(err)
	}
	spsMap := make(map[uint32]*SPS, 1)
	spsMap[uint32(sps.ParameterID)] = sps
	ppsData, _ := hex.DecodeString(ppsHex)
	pps, err := ParsePPSNALUnit(ppsData, spsMap)
	if err != nil {
		t.Error(err)
	}
	ppsMap := make(map[uint32]*PPS, 1)
	ppsMap[uint32(pps.PicParameterSetID)] = pps
	naluStart, _ := hex.DecodeString(naluStartHex)
	sh, err := ParseSliceHeader(naluStart, spsMap, ppsMap)
	if err != nil {
		t.Error(err)
	}
	wantedSliceHeaderSize := uint32(11)
	if sh.Size != wantedSliceHeaderSize {
		t.Errorf("got %d want %d", sh.Size, wantedSliceHeaderSize)
	}
}

// TestSliceHeaderSize checks that SliceHeaderSize returns the Size and the error of ParseSliceHeader, for the
// NAL units of the test streams and for slices that are truncated, use weighted prediction or refer to an
// unknown PPS.
func TestSliceHeaderSize(t *testing.T) {
	type sliceCase struct {
		name   string
		nalu   []byte
		spsMap map[uint32]*SPS
		ppsMap map[uint32]*PPS
	}
	var cases []sliceCase
	for _, file := range []struct {
		name       string
		byteStream bool
	}{{"testdata/blackframe.264", true}, {"testdata/two-frames.264", false}} {
		data, err := os.ReadFile(file.name)
		if err != nil {
			t.Fatal(err)
		}
		var nalus [][]byte
		if file.byteStream {
			nalus = ExtractNalusFromByteStream(data)
		} else if nalus, err = GetNalusFromSample(data); err != nil {
			t.Fatal(err)
		}
		spsMap := map[uint32]*SPS{}
		ppsMap := map[uint32]*PPS{}
		for i, nalu := range nalus {
			switch GetNaluType(nalu[0]) {
			case NALU_SPS:
				sps, err := ParseSPSNALUnit(nalu, true)
				if err != nil {
					t.Fatal(err)
				}
				spsMap[sps.ParameterID] = sps
			case NALU_PPS:
				pps, err := ParsePPSNALUnit(nalu, spsMap)
				if err != nil {
					t.Fatal(err)
				}
				ppsMap[pps.PicParameterSetID] = pps
			default:
				cases = append(cases, sliceCase{fmt.Sprintf("%s NAL unit %d", file.name, i), nalu, spsMap, ppsMap})
			}
		}
	}
	spsMap, ppsMap := paramSetMaps(t, defaultSPSFields(), ppsFields{})
	idr := writeIDRSlice(t, 0)
	for n := 1; n <= len(idr); n++ {
		cases = append(cases, sliceCase{fmt.Sprintf("IDR slice, %d bytes", n), idr[:n], spsMap, ppsMap})
	}
	cases = append(cases, sliceCase{"IDR slice, unknown PPS", idr, spsMap, map[uint32]*PPS{}})
	wpSPSMap, wpPPSMap := paramSetMaps(t, defaultSPSFields(), ppsFields{weightedPredFlag: true})
	cases = append(cases, sliceCase{"P slice with pred_weight_table", writePSlice(t, 2, true), wpSPSMap, wpPPSMap})

	nrOK := 0
	for _, c := range cases {
		sh, err := ParseSliceHeader(c.nalu, c.spsMap, c.ppsMap)
		size, sizeErr := SliceHeaderSize(c.nalu, c.spsMap, c.ppsMap)
		if fmt.Sprint(sizeErr) != fmt.Sprint(err) {
			t.Errorf("%s: SliceHeaderSize error %v, ParseSliceHeader error %v", c.name, sizeErr, err)
			continue
		}
		if err != nil {
			continue
		}
		nrOK++
		if size != int(sh.Size) {
			t.Errorf("%s: SliceHeaderSize %d, ParseSliceHeader size %d", c.name, size, sh.Size)
		}
	}
	if nrOK < 4 {
		t.Errorf("only %d slices parsed", nrOK)
	}
}

func TestSliceHeaderSizeAllocations(t *testing.T) {
	spsMap, ppsMap := paramSetMaps(t, defaultSPSFields(), ppsFields{weightedPredFlag: true})
	nalu := writePSlice(t, 2, true)
	allocs := testing.AllocsPerRun(10, func() {
		if _, err := SliceHeaderSize(nalu, spsMap, ppsMap); err != nil {
			t.Fatal(err)
		}
	})
	if allocs != 0 {
		t.Errorf("got %.0f allocations, want 0", allocs)
	}
}

func BenchmarkParseNALUnits(b *testing.B) {
	spsData, _ := hex.DecodeString("6764001eacd940a02ff9610000030001000003003c8f162d96")
	sps, err := ParseSPSNALUnit(spsData, true)
	if err != nil {
		b.Fatal(err)
	}
	spsMap := map[uint32]*SPS{sps.ParameterID: sps}
	ppsData, _ := hex.DecodeString("68ebecb22c")
	pps, err := ParsePPSNALUnit(ppsData, spsMap)
	if err != nil {
		b.Fatal(err)
	}
	ppsMap := map[uint32]*PPS{pps.PicParameterSetID: pps}
	nalu, _ := hex.DecodeString("419a6649e10f2653022fff8700000302c8a32d32")
	b.Run("ParseSPSNALUnit", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := ParseSPSNALUnit(spsData, true); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("ParseSliceHeader", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := ParseSliceHeader(nalu, spsMap, ppsMap); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("SliceHeaderSize", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := SliceHeaderSize(nalu, spsMap, ppsMap); err != nil {
				b.Fatal(err)
			}
		}
	})
}
