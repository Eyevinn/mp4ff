package mp4

import (
	"bytes"
	"testing"
)

func TestColrEncodeDecode(t *testing.T) {
	cases := []ColrBox{
		{
			ColorType:               onScreenColors,
			ColorPrimaries:          9,
			TransferCharacteristics: 16,
			MatrixCoefficients:      9,
			FullRangeFlag:           true,
		},
		{
			ColorType:               onScreenColors,
			ColorPrimaries:          9,
			TransferCharacteristics: 16,
			MatrixCoefficients:      9,
			FullRangeFlag:           false,
		},
		{
			ColorType:  restrictedICCType,
			ICCProfile: []byte{1, 2, 2, 43, 4},
		},
		{
			ColorType:  unrestrictedICCType,
			ICCProfile: []byte{1, 2, 2, 43, 4, 5},
		},
	}
	for _, tc := range cases {
		boxDiffAfterEncodeAndDecode(t, &tc)
	}
}

func TestColrInfo(t *testing.T) {
	cases := []struct {
		cb     ColrBox
		wanted string
	}{
		{
			cb: ColrBox{
				ColorType:               onScreenColors,
				ColorPrimaries:          9,
				TransferCharacteristics: 9,
				MatrixCoefficients:      16,
				FullRangeFlag:           true,
			},
			wanted: ("[colr] size=19\n - colorType: nclx\n - ColorPrimaries: 9, " +
				"TransferCharacteristics: 9, MatrixCoefficients: 16, FullRange: true\n"),
		},
		{
			cb: ColrBox{
				ColorType:  restrictedICCType,
				ICCProfile: []byte{0x02, 0x04},
			},
			wanted: "[colr] size=14\n - colorType: rICC\n - ICCProfile: 0204\n",
		},
	}
	for _, tc := range cases {
		b := bytes.Buffer{}
		err := tc.cb.Info(&b, "-1", "", "")
		if err != nil {
			t.Error(err)
		}
		gotStr := b.String()
		if gotStr != tc.wanted {
			t.Errorf("got %q, but wanted %q\n", gotStr, tc.wanted)
		}
	}
}
