package mp4_test

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

func TestIloc(t *testing.T) {
	boxes := []*mp4.IlocBox{
		{Version: 0, OffsetSize: 4, LengthSize: 4, BaseOffsetSize: 4, Items: []mp4.IlocItem{
			{ItemID: 1, BaseOffset: 250, Extents: []mp4.IlocExtent{{Offset: 0, Length: 10235}}},
		}},
		{Version: 0, OffsetSize: 4, LengthSize: 4, BaseOffsetSize: 0, Items: []mp4.IlocItem{
			{ItemID: 1, Extents: []mp4.IlocExtent{{Offset: 0xffffffff, Length: 0xffffffff}}},
			{ItemID: 0xffff, DataReferenceIndex: 1, Extents: []mp4.IlocExtent{{Offset: 1, Length: 2}, {Offset: 3, Length: 4}}},
		}},
		{Version: 0, Items: []mp4.IlocItem{ // the whole referenced data, without any extent field
			{ItemID: 1, DataReferenceIndex: 1, Extents: []mp4.IlocExtent{{}}},
		}},
		{Version: 1, OffsetSize: 8, LengthSize: 8, BaseOffsetSize: 8, IndexSize: 4, Items: []mp4.IlocItem{
			{ItemID: 2, ConstructionMethod: mp4.IlocConstructionIdat, BaseOffset: 1 << 40,
				Extents: []mp4.IlocExtent{{Offset: 1 << 33, Length: 1 << 34}}},
			{ItemID: 3, ConstructionMethod: mp4.IlocConstructionItem,
				Extents: []mp4.IlocExtent{{ItemReferenceIndex: 0xffffffff, Offset: 5, Length: 6}}},
		}},
		{Version: 2, OffsetSize: 4, LengthSize: 4, Items: []mp4.IlocItem{
			{ItemID: 0x10000, ConstructionMethod: mp4.IlocConstructionFile, Extents: []mp4.IlocExtent{{Offset: 7, Length: 8}}},
		}},
		{Version: 1, OffsetSize: 2, LengthSize: 3, BaseOffsetSize: 1, IndexSize: 1, Items: []mp4.IlocItem{ // unusual sizes
			{ItemID: 4, BaseOffset: 0xff, Extents: []mp4.IlocExtent{{ItemReferenceIndex: 1, Offset: 0xffff, Length: 0xffffff}}},
		}},
	}
	for _, iloc := range boxes {
		boxDiffAfterEncodeAndDecode(t, iloc)
	}
}

func TestIlocInfo(t *testing.T) {
	iloc := &mp4.IlocBox{Version: 1, OffsetSize: 4, LengthSize: 4, IndexSize: 4, Items: []mp4.IlocItem{
		{ItemID: 3, ConstructionMethod: mp4.IlocConstructionItem,
			Extents: []mp4.IlocExtent{{ItemReferenceIndex: 1, Offset: 5, Length: 6}}},
	}}
	var sb strings.Builder
	if err := iloc.Info(&sb, "iloc:1", "", "  "); err != nil {
		t.Fatal(err)
	}
	want := `[iloc] size=36 version=1 flags=000000
 - offsetSize: 4
 - lengthSize: 4
 - baseOffsetSize: 0
 - indexSize: 4
 - itemCount: 1
 - item[1]: itemID=3 constructionMethod=2 dataReferenceIndex=0 baseOffset=0 extentCount=1
   - extent[1]: itemReferenceIndex=1 offset=5 length=6
`
	if sb.String() != want {
		t.Errorf("got info:\n%swant:\n%s", sb.String(), want)
	}
}

func TestIlocDecodeErrors(t *testing.T) {
	cases := []struct {
		desc   string
		hexBox string
		errMsg string
	}{
		{
			desc:   "version 3",
			hexBox: "00000010696c6f6303000000" + "4400" + "0000",
			errMsg: "decode iloc pos 0: iloc: version 3 is not supported",
		},
		{
			desc:   "offset size 9",
			hexBox: "00000010696c6f6300000000" + "9400" + "0000",
			errMsg: "decode iloc pos 0: iloc: field size 9 is more than 8 bytes",
		},
		{
			desc:   "two items announced, one present",
			hexBox: "00000016696c6f6300000000" + "0000" + "0002" + "000100000000",
			errMsg: "decode iloc pos 0: iloc: 2 items do not fit in 6 bytes",
		},
		{
			desc:   "two extents announced, one present",
			hexBox: "0000001e696c6f6300000000" + "4400" + "0001" + "000100000002" + "0000000100000002",
			errMsg: "decode iloc pos 0: iloc: 2 extents of item 1 do not fit in 8 bytes",
		},
		{
			desc:   "two extents without fields",
			hexBox: "00000016696c6f6300000000" + "0000" + "0001" + "000100000002",
			errMsg: "decode iloc pos 0: iloc: 2 extents of item 1 without any field",
		},
		{
			desc:   "item cut short",
			hexBox: "00000013696c6f6300000000" + "0000" + "0000" + "000100",
			errMsg: "decode iloc pos 0: iloc: 3 bytes after the last item",
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

func TestIlocEncodeErrors(t *testing.T) {
	item := func(i mp4.IlocItem) []mp4.IlocItem { return []mp4.IlocItem{i} }
	extent := func(e mp4.IlocExtent) []mp4.IlocItem {
		return item(mp4.IlocItem{ItemID: 1, Extents: []mp4.IlocExtent{e}})
	}
	cases := []struct {
		desc string
		iloc mp4.IlocBox
	}{
		{"version 3", mp4.IlocBox{Version: 3}},
		{"offset size 9", mp4.IlocBox{OffsetSize: 9}},
		{"17-bit item ID in version 1", mp4.IlocBox{Version: 1, Items: item(mp4.IlocItem{ItemID: 0x10000})}},
		{"construction method in version 0", mp4.IlocBox{Items: item(mp4.IlocItem{ConstructionMethod: 1})}},
		{"5-bit construction method", mp4.IlocBox{Version: 1, Items: item(mp4.IlocItem{ConstructionMethod: 16})}},
		{"base offset without field", mp4.IlocBox{Items: item(mp4.IlocItem{BaseOffset: 1})}},
		{"33-bit offset in 4 bytes", mp4.IlocBox{OffsetSize: 4, Items: extent(mp4.IlocExtent{Offset: 1 << 32})}},
		{"33-bit length in 4 bytes", mp4.IlocBox{LengthSize: 4, Items: extent(mp4.IlocExtent{Length: 1 << 32})}},
		{"index in version 0", mp4.IlocBox{IndexSize: 4, Items: extent(mp4.IlocExtent{ItemReferenceIndex: 1})}},
		{"index without field", mp4.IlocBox{Version: 1, Items: extent(mp4.IlocExtent{ItemReferenceIndex: 1})}},
		{"two extents without fields", mp4.IlocBox{Items: item(mp4.IlocItem{Extents: make([]mp4.IlocExtent, 2)})}},
		{"65536 extents", mp4.IlocBox{OffsetSize: 4, Items: item(mp4.IlocItem{Extents: make([]mp4.IlocExtent, 0x10000)})}},
		{"65536 items in version 1", mp4.IlocBox{Version: 1, Items: make([]mp4.IlocItem, 0x10000)}},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			sw := bits.NewFixedSliceWriter(int(c.iloc.Size()))
			if err := c.iloc.EncodeSW(sw); err == nil {
				t.Error("no error from EncodeSW")
			}
			if err := c.iloc.Encode(&strings.Builder{}); err == nil {
				t.Error("no error from Encode")
			}
		})
	}
}
