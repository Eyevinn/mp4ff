package sei

import (
	"bytes"
	"encoding/hex"
	"runtime"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/go-test/deep"
)

func TestHEVCSETI1PicTiming(t *testing.T) {

	cases := []struct {
		name           string
		naluPayloadHex string
		extParams      HEVCPicTimingParams
		expected       PicTimingHevcSEI
		expNonFatalErr error
	}{
		{
			name:           "HEVC_SETI1PicTiming",
			naluPayloadHex: "01071000001a0000030180",
			extParams: HEVCPicTimingParams{
				FrameFieldInfoPresentFlag:              true,
				CpbDpbDelaysPresentFlag:                true,
				SubPicHrdParamsPresentFlag:             false,
				SubPicCpbParamsInPicTimingSeiFlag:      false,
				AuCbpRemovalDelayLengthMinus1:          23,
				DpbOutputDelayLengthMinus1:             0,
				DpbOutputDelayDuLengthMinus1:           23,
				DuCpbRemovalDelayIncrementLengthMinus1: 0,
			},
			expected: PicTimingHevcSEI{
				FrameFieldInfo: &HEVCFrameFieldInfo{
					PicStruct:      1,
					SourceScanType: 0,
					DuplicateFlag:  false,
				},
			},
			expNonFatalErr: nil,
		},
	}

	for _, tc := range cases {
		seiNaluPayload, _ := hex.DecodeString(tc.naluPayloadHex)
		r := bytes.NewReader(seiNaluPayload)
		seis, err := ExtractSEIData(r)
		if err != nil && err != tc.expNonFatalErr {
			t.Error(err)
		}
		if len(seis) != 1 {
			t.Errorf("%s: Not %d but %d sei messages found", tc.name, 1, len(seis))
		}
		seiMessage, err := DecodePicTimingHevcSEI(&seis[0], tc.extParams)
		if err != nil {
			t.Error(err)
		}
		if seiMessage.Type() != SEIPicTimingType {
			t.Errorf("%s: got SEI type %d instead of %d", tc.name, seiMessage.Type(), SEIPicTimingType)
		}
		seiPT := seiMessage.(*PicTimingHevcSEI)
		diff := deep.Equal(seiPT.FrameFieldInfo, tc.expected.FrameFieldInfo)
		if diff != nil {
			t.Errorf("%s: %v %s", tc.name, diff, "frame field info mismatch")
		}
	}
}

// picTimingWithDecodingUnits returns the payload of a picture timing SEI with sub-picture
// CPB parameters, num_decoding_units_minus1 and then the given num_nalus_in_du_minus1 values
// with du_cpb_removal_delay_increment_minus1 values of 8 bits between them.
func picTimingWithDecodingUnits(numDecodingUnitsMinus1 uint, numNalusInDuMinus1 []uint) []byte {
	var buf bytes.Buffer
	w := bits.NewEBSPWriter(&buf)
	w.Write(1, 8) // au_cpb_removal_delay_minus1
	w.Write(2, 8) // pic_dpb_output_delay
	w.Write(3, 8) // pic_dpb_output_du_delay
	w.WriteExpGolomb(numDecodingUnitsMinus1)
	w.Write(0, 1) // du_common_cpb_removal_delay_flag
	for i, n := range numNalusInDuMinus1 {
		w.WriteExpGolomb(n)
		if i < len(numNalusInDuMinus1)-1 {
			w.Write(uint(10+i), 8) // du_cpb_removal_delay_increment_minus1
		}
	}
	w.WriteRbspTrailingBits()
	return buf.Bytes()
}

var subPicPicTimingParams = HEVCPicTimingParams{
	CpbDpbDelaysPresentFlag:                true,
	SubPicHrdParamsPresentFlag:             true,
	SubPicCpbParamsInPicTimingSeiFlag:      true,
	AuCbpRemovalDelayLengthMinus1:          7,
	DpbOutputDelayLengthMinus1:             7,
	DpbOutputDelayDuLengthMinus1:           7,
	DuCpbRemovalDelayIncrementLengthMinus1: 7,
}

func TestHEVCPicTimingDecodingUnits(t *testing.T) {
	payload := picTimingWithDecodingUnits(2, []uint{4, 5, 6})
	msg, err := DecodePicTimingHevcSEI(NewSEIData(SEIPicTimingType, payload), subPicPicTimingParams)
	if err != nil {
		t.Fatal(err)
	}
	pt := msg.(*PicTimingHevcSEI)
	if diff := deep.Equal(pt.NumNalusInDuMinus1, []uint32{4, 5, 6}); diff != nil {
		t.Errorf("num_nalus_in_du_minus1: %v", diff)
	}
	if diff := deep.Equal(pt.DuCpbRemovalDelayIncrementMinus1, []uint32{10, 11}); diff != nil {
		t.Errorf("du_cpb_removal_delay_increment_minus1: %v", diff)
	}
}

func TestHEVCPicTimingNumDecodingUnitsBeyondData(t *testing.T) {
	payload := picTimingWithDecodingUnits(1<<30, []uint{4})
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	_, err := DecodePicTimingHevcSEI(NewSEIData(SEIPicTimingType, payload), subPicPicTimingParams)
	runtime.ReadMemStats(&after)
	if err == nil {
		t.Error("expected an error")
	}
	if alloc := after.TotalAlloc - before.TotalAlloc; alloc > 64<<10 {
		t.Errorf("allocated %d bytes for a %d-byte payload", alloc, len(payload))
	}
}
