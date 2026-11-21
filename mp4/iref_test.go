package mp4_test

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

func TestIref(t *testing.T) {
	small := &mp4.IrefBox{}
	small.AddChild(&mp4.IrefTypeBox{Name: "dimg", FromItemID: 3, ToItemIDs: []uint32{1, 2}})
	small.AddChild(&mp4.IrefTypeBox{Name: "cdsc", FromItemID: 0xffff, ToItemIDs: []uint32{3}})
	small.AddChild(&mp4.IrefTypeBox{Name: "auxl", FromItemID: 4})
	large := &mp4.IrefBox{Version: 1}
	large.AddChild(&mp4.IrefTypeBox{Name: "thmb", FromItemID: 0x10000, ToItemIDs: []uint32{1, 0xffffffff}})
	for _, iref := range []*mp4.IrefBox{{}, small, large} {
		boxDiffAfterEncodeAndDecode(t, iref)
	}
	if small.Size() != 12+16+14+12 || large.Size() != 12+22 {
		t.Errorf("sizes %d and %d instead of 54 and 34", small.Size(), large.Size())
	}

	var sb strings.Builder
	if err := small.Info(&sb, "", "", "  "); err != nil {
		t.Fatal(err)
	}
	want := `[iref] size=54 version=0 flags=000000
  [dimg] size=16
   - fromItemID: 3
   - toItemIDs: [1 2]
  [cdsc] size=14
   - fromItemID: 65535
   - toItemIDs: [3]
  [auxl] size=12
   - fromItemID: 4
   - toItemIDs: []
`
	if sb.String() != want {
		t.Errorf("got info:\n%swant:\n%s", sb.String(), want)
	}

	// The version decides the size of the item IDs, also when it is set after the children were added.
	small.Version = 1
	if small.Size() != 12+22+18+14 {
		t.Errorf("size %d instead of 66 in version 1", small.Size())
	}
	boxDiffAfterEncodeAndDecode(t, small)
}

func TestIrefErrors(t *testing.T) {
	decodeCases := []struct {
		desc   string
		hexBox string
		errMsg string
	}{
		{
			desc:   "version 2",
			hexBox: "0000000c6972656602000000",
			errMsg: "decode iref pos 0: iref: version 2 is not supported",
		},
		{
			desc:   "reference beyond the end of iref",
			hexBox: "000000186972656600000000" + "0000000e64696d67" + "00030001",
			errMsg: "decode iref pos 0: iref: child dimg of size 14 goes beyond the end of iref",
		},
		{
			desc:   "bytes after the last reference",
			hexBox: "0000001d6972656600000000" + "0000000c64696d67" + "00030000" + "0000000000",
			errMsg: "decode iref pos 0: iref: 5 bytes after the last reference",
		},
		{
			desc:   "reference without reference count",
			hexBox: "000000166972656600000000" + "0000000a64696d67" + "0003",
			errMsg: "decode iref pos 0: iref: dimg: 2 bytes are too few for an item reference",
		},
		{
			desc:   "two references announced, one present",
			hexBox: "0000001a6972656600000000" + "0000000e64696d67" + "00030002" + "0001",
			errMsg: "decode iref pos 0: iref: dimg: 2 references do not match 2 bytes",
		},
		{
			desc:   "16-bit item IDs in version 1",
			hexBox: "0000001a6972656601000000" + "0000000e64696d67" + "00030001" + "0001",
			errMsg: "decode iref pos 0: iref: dimg: 1 references do not match 0 bytes",
		},
	}
	for _, c := range decodeCases {
		t.Run(c.desc, func(t *testing.T) {
			data, err := hex.DecodeString(c.hexBox)
			if err != nil {
				t.Fatal(err)
			}
			assertBoxDecodeError(t, data, 0, c.errMsg)
		})
	}

	encodeCases := []struct {
		desc    string
		version byte
		ref     mp4.IrefTypeBox
	}{
		{"version 2", 2, mp4.IrefTypeBox{Name: "dimg"}},
		{"17-bit from item ID in version 0", 0, mp4.IrefTypeBox{Name: "dimg", FromItemID: 0x10000}},
		{"17-bit to item ID in version 0", 0, mp4.IrefTypeBox{Name: "dimg", ToItemIDs: []uint32{0x10000}}},
		{"65536 references", 1, mp4.IrefTypeBox{Name: "dimg", ToItemIDs: make([]uint32, 0x10000)}},
		{"reference type of 3 bytes", 0, mp4.IrefTypeBox{Name: "dim"}},
	}
	for _, c := range encodeCases {
		t.Run(c.desc, func(t *testing.T) {
			iref := mp4.IrefBox{Version: c.version}
			iref.AddChild(&c.ref)
			sw := bits.NewFixedSliceWriter(int(iref.Size()))
			if err := iref.EncodeSW(sw); err == nil {
				t.Error("no error from EncodeSW")
			}
		})
	}
}
