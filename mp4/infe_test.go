package mp4_test

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

func TestInfe(t *testing.T) {
	boxes := []*mp4.InfeBox{
		{Version: 0, ItemID: 1, ItemProtectionIndex: 2, ItemName: "name", ContentType: "text/xml", ContentEncoding: "gzip"},
		{Version: 1, ItemID: 1, ItemName: "name", ContentType: "text/xml"},
		{Version: 1, ItemID: 1, ItemName: "name", ContentType: "text/xml", ExtensionType: "fdel",
			ExtensionData: []byte{1, 2, 3}},
		{Version: 2, ItemID: 0xffff, ItemType: "av01"},
		{Version: 2, Flags: mp4.InfeFlagHidden, ItemID: 2, ItemType: "hvc1", ItemName: "tile"},
		{Version: 2, ItemID: 3, ItemType: "mime", ItemName: "XMP", ContentType: "application/rdf+xml"},
		{Version: 2, ItemID: 4, ItemType: "uri ", ItemURIType: "urn:example:type"},
		{Version: 3, ItemID: 0x10000, ItemProtectionIndex: 1, ItemType: "Exif", ItemName: "exif"},
	}
	for _, infe := range boxes {
		boxDiffAfterEncodeAndDecode(t, infe)
	}
	if boxes[3].IsHidden() || !boxes[4].IsHidden() {
		t.Error("hidden flag not detected")
	}
}

// TestInfeOptionalContentEncoding checks that the optional content encoding is kept as it is:
// absent, empty or with a value.
func TestInfeOptionalContentEncoding(t *testing.T) {
	const start = "696e666502000000" + "00030000" + "6d696d65" + "00" + "612f6200" // infe v2, item 3, mime, "", "a/b"
	cases := []struct {
		desc            string
		hexBox          string
		contentEncoding string
		infoLine        string
	}{
		{"absent", "00000019" + start, "", ""},
		{"empty", "0000001a" + start + "00", "", ` - contentEncoding: ""`},
		{"gzip", "0000001e" + start + "677a697000", "gzip", ` - contentEncoding: "gzip"`},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			data, err := hex.DecodeString(c.hexBox)
			if err != nil {
				t.Fatal(err)
			}
			cmpAfterDecodeEncodeBox(t, data)
			box, err := mp4.DecodeBoxSR(0, bits.NewFixedSliceReader(data))
			if err != nil {
				t.Fatal(err)
			}
			infe := box.(*mp4.InfeBox)
			if infe.ContentType != "a/b" || infe.ContentEncoding != c.contentEncoding {
				t.Errorf("got content type %q and encoding %q", infe.ContentType, infe.ContentEncoding)
			}
			var sb strings.Builder
			if err := infe.Info(&sb, "", "", "  "); err != nil {
				t.Fatal(err)
			}
			if got := strings.Contains(sb.String(), "contentEncoding"); got != (c.infoLine != "") ||
				!strings.Contains(sb.String(), c.infoLine) {
				t.Errorf("unexpected content encoding in info:\n%s", sb.String())
			}
		})
	}
}

func TestInfeErrors(t *testing.T) {
	decodeCases := []struct {
		desc   string
		hexBox string
		errMsg string
	}{
		{
			desc:   "version 4",
			hexBox: "00000015696e666504000000" + "00010000" + "6176303100",
			errMsg: "decode infe pos 0: infe: version 4 is not supported",
		},
		{
			desc:   "item name without terminator",
			hexBox: "00000015696e666502000000" + "00010000" + "6176303161",
			errMsg: "decode infe pos 0: infe: did not find terminating zero",
		},
		{
			desc:   "extension type cut short",
			hexBox: "00000015696e666501000000" + "00010000" + "000000" + "6664",
			errMsg: "decode infe pos 0: infe: 2 bytes are too few for an extension type",
		},
		{
			desc:   "bytes after the item name",
			hexBox: "00000016696e666502000000" + "00010000" + "6176303100" + "00",
			errMsg: "decode infe pos 0: infe: 1 bytes after the last field",
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
		desc string
		infe mp4.InfeBox
	}{
		{"version 4", mp4.InfeBox{Version: 4, ItemType: "av01"}},
		{"17-bit item ID in version 2", mp4.InfeBox{Version: 2, ItemID: 0x10000, ItemType: "av01"}},
		{"3-byte item type", mp4.InfeBox{Version: 2, ItemType: "av1"}},
		{"3-byte extension type", mp4.InfeBox{Version: 1, ExtensionType: "fde"}},
	}
	for _, c := range encodeCases {
		t.Run(c.desc, func(t *testing.T) {
			sw := bits.NewFixedSliceWriter(int(c.infe.Size()))
			if err := c.infe.EncodeSW(sw); err == nil {
				t.Error("no error from EncodeSW")
			}
		})
	}
}
