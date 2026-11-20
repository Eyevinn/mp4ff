package mp4_test

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

func TestIpma(t *testing.T) {
	boxes := []*mp4.IpmaBox{
		{Entries: []mp4.IpmaEntry{
			{ItemID: 1, Associations: []mp4.IpmaAssociation{{Essential: true, PropertyIndex: 1}, {PropertyIndex: 127}}},
			{ItemID: 0xffff, Associations: []mp4.IpmaAssociation{{PropertyIndex: 0}}},
		}},
		{Flags: mp4.IpmaFlagLargeIndex, Entries: []mp4.IpmaEntry{
			{ItemID: 1, Associations: []mp4.IpmaAssociation{{Essential: true, PropertyIndex: 0x7fff}, {PropertyIndex: 128}}},
		}},
		{Version: 1, Entries: []mp4.IpmaEntry{
			{ItemID: 0x10000, Associations: make([]mp4.IpmaAssociation, 255)},
		}},
	}
	for _, ipma := range boxes {
		boxDiffAfterEncodeAndDecode(t, ipma)
	}

	var sb strings.Builder
	if err := boxes[0].Info(&sb, "ipma:1", "", "  "); err != nil {
		t.Fatal(err)
	}
	want := `[ipma] size=25 version=0 flags=000000
 - entryCount: 2
 - entry[1]: itemID=1 propertyIndexes=[1(essential) 127]
 - entry[2]: itemID=65535 propertyIndexes=[0]
`
	if sb.String() != want {
		t.Errorf("got info:\n%swant:\n%s", sb.String(), want)
	}
}

func TestIpmaErrors(t *testing.T) {
	decodeCases := []struct {
		desc   string
		hexBox string
		errMsg string
	}{
		{
			desc:   "two entries announced, one present",
			hexBox: "0000001369706d6100000000" + "00000002" + "000100",
			errMsg: "decode ipma pos 0: ipma: 2 entries do not fit in 3 bytes",
		},
		{
			desc:   "two associations announced, one present",
			hexBox: "0000001469706d6100000000" + "00000001" + "000102" + "81",
			errMsg: "decode ipma pos 0: ipma: 2 associations of item 1 do not fit in 1 bytes",
		},
		{
			desc:   "half of a 15-bit property index",
			hexBox: "0000001469706d6100000001" + "00000001" + "000101" + "81",
			errMsg: "decode ipma pos 0: ipma: 1 associations of item 1 do not fit in 1 bytes",
		},
		{
			desc:   "byte after the last entry",
			hexBox: "0000001569706d6100000000" + "00000001" + "000101" + "81" + "00",
			errMsg: "decode ipma pos 0: ipma: 1 bytes after the last entry",
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

	entry := func(e mp4.IpmaEntry) []mp4.IpmaEntry { return []mp4.IpmaEntry{e} }
	encodeCases := []struct {
		desc string
		ipma mp4.IpmaBox
	}{
		{"17-bit item ID in version 0", mp4.IpmaBox{Entries: entry(mp4.IpmaEntry{ItemID: 0x10000})}},
		{"256 associations", mp4.IpmaBox{Entries: entry(mp4.IpmaEntry{Associations: make([]mp4.IpmaAssociation, 256)})}},
		{"8-bit property index without the flag", mp4.IpmaBox{Entries: entry(mp4.IpmaEntry{
			Associations: []mp4.IpmaAssociation{{PropertyIndex: 128}}})}},
		{"16-bit property index", mp4.IpmaBox{Flags: mp4.IpmaFlagLargeIndex, Entries: entry(mp4.IpmaEntry{
			Associations: []mp4.IpmaAssociation{{PropertyIndex: 0x8000}}})}},
	}
	for _, c := range encodeCases {
		t.Run(c.desc, func(t *testing.T) {
			sw := bits.NewFixedSliceWriter(int(c.ipma.Size()))
			if err := c.ipma.EncodeSW(sw); err == nil {
				t.Error("no error from EncodeSW")
			}
		})
	}
}
