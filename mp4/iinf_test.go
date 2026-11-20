package mp4_test

import (
	"encoding/hex"
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

func TestIinf(t *testing.T) {
	for _, version := range []byte{0, 1} {
		iinf := &mp4.IinfBox{Version: version}
		iinf.AddChild(&mp4.InfeBox{Version: 2, ItemID: 1, ItemType: "av01"})
		iinf.AddChild(&mp4.InfeBox{Version: 2, ItemID: 2, ItemType: "Exif", ItemName: "exif"})
		boxDiffAfterEncodeAndDecode(t, iinf)
		if infe := iinf.GetInfe(2); infe == nil || infe.ItemType != "Exif" {
			t.Errorf("version %d: item 2 not found", version)
		}
		if infe := iinf.GetInfe(3); infe != nil {
			t.Errorf("version %d: found item 3", version)
		}
	}
}

func TestIinfDecodeErrors(t *testing.T) {
	const infe = "00000015696e666502000000" + "00010000" + "6176303100"
	cases := []struct {
		desc   string
		hexBox string
		errMsg string
	}{
		{
			desc:   "two entries announced, one present",
			hexBox: "0000002369696e6600000000" + "0002" + infe,
			errMsg: "decode iinf pos 0: iinf: entry count 2 but 1 entries",
		},
		{
			desc:   "no entry count",
			hexBox: "0000000d69696e6600000000" + "00",
			errMsg: "decode iinf pos 0: iinf: read too far in SliceReader",
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			data, err := hex.DecodeString(c.hexBox)
			if err != nil {
				t.Fatal(err)
			}
			assertBoxDecodeError(t, data, 0, c.errMsg)
		})
	}
}
