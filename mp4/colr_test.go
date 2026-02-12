package mp4_test

import (
	"bytes"
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

func TestColrEncodeDecode(t *testing.T) {
	cases := []mp4.ColrBox{
		{
			ColorType:               mp4.ColorTypeOnScreenColors,
			ColorPrimaries:          9,
			TransferCharacteristics: 16,
			MatrixCoefficients:      9,
			FullRangeFlag:           true,
		},
		{
			ColorType:               mp4.ColorTypeOnScreenColors,
			ColorPrimaries:          9,
			TransferCharacteristics: 16,
			MatrixCoefficients:      9,
			FullRangeFlag:           false,
		},
		{
			ColorType:  mp4.ColorTypeRestrictedICCProfile,
			ICCProfile: []byte{1, 2, 2, 43, 4},
		},
		{
			ColorType:  mp4.ColorTypeUnrestrictedICCTProfile,
			ICCProfile: []byte{1, 2, 2, 43, 4, 5},
		},
		{
			ColorType:               mp4.QuickTimeColorParameters,
			ColorPrimaries:          1,
			TransferCharacteristics: 1,
			MatrixCoefficients:      1,
		},
		{
			ColorType:      "xyzd",
			UnknownPayload: []byte{0x0, 0x1, 0x0, 0x1},
		},
	}
	for _, tc := range cases {
		boxDiffAfterEncodeAndDecode(t, &tc)
	}
}

func TestColrInfo(t *testing.T) {
	cases := []struct {
		cb     mp4.ColrBox
		wanted string
	}{
		{
			cb: mp4.ColrBox{
				ColorType:               mp4.ColorTypeOnScreenColors,
				ColorPrimaries:          9,
				TransferCharacteristics: 9,
				MatrixCoefficients:      16,
				FullRangeFlag:           true,
			},
			wanted: ("[colr] size=19\n - colorType: nclx\n - ColorPrimaries: 9, " +
				"TransferCharacteristics: 9, MatrixCoefficients: 16, FullRange: true\n"),
		},
		{
			cb: mp4.ColrBox{
				ColorType:  mp4.ColorTypeRestrictedICCProfile,
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

func TestColrWithICCProfile(t *testing.T) {
	infile := "testdata/init_with_colr.mp4"
	fmp4, err := mp4.ReadMP4File(infile)
	if err != nil {
		t.Error(err)
	}
	if fmp4 == nil {
		t.Fatal("Failed to read mp4 file")
	}

	track := fmp4.Moov.Traks[0]
	avcx := track.Mdia.Minf.Stbl.Stsd.AvcX
	if avcx == nil {
		t.Fatal("No avcC box found")
	}

	colr := &mp4.ColrBox{}
	for _, ch := range avcx.Children {
		switch ch.Type() {
		case "colr":
			colr = ch.(*mp4.ColrBox)
		}
	}

	if colr == nil {
		t.Fatal("No colr box found")
	}

	if colr.ColorType != mp4.ColorTypeUnrestrictedICCTProfile {
		t.Errorf("Expected ColorType %s, got %s", mp4.ColorTypeUnrestrictedICCTProfile, colr.ColorType)
	}

	// check ICC profile size, [colr] size=8+460
	if len(colr.ICCProfile) != 460-4 {
		t.Errorf("Expected ICCProfile size")
	}
}
