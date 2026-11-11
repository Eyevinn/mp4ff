package mp4_test

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

// The QuickTime metadata of an iPhone movie: the keys atom of moov/meta, and its ilst atom.
const iPhoneKeys = "0000009d6b6579730000000000000004000000206d647461636f6d2e6170706c652e717569636b74696d652e6d616b65" +
	"000000216d647461636f6d2e6170706c652e717569636b74696d652e6d6f64656c000000246d647461636f6d2e617070" +
	"6c652e717569636b74696d652e736f667477617265000000286d647461636f6d2e6170706c652e717569636b74696d65" +
	"2e6372656174696f6e64617465"

const iPhoneIlst = "00000099696c73740000001d00000001000000156461746100000001000000004170706c650000002600000002000000" +
	"1e6461746100000001000000006950686f6e65203132206d696e690000001e0000000300000016646174610000000100" +
	"00000031342e342e31000000300000000400000028646174610000000100000000323032312d30352d32395431353a31" +
	"343a31392d30343030"

// The keys atom of a timed metadata sample entry mebx in the same movie, which holds boxes instead.
const mebxKeys = "000000656b6579730000005d00000001000000316b6579646d647461636f6d2e6170706c652e717569636b74696d652e" +
	"766964656f2d6f7269656e746174696f6e0000001064747970000000000000004200000014736470640000000c736470" +
	"6900000000"

func decodeHex(t *testing.T, h string) (mp4.Box, []byte) {
	t.Helper()
	data, err := hex.DecodeString(h)
	if err != nil {
		t.Fatal(err)
	}
	cmpAfterDecodeEncodeBox(t, data)
	box, err := mp4.DecodeBox(0, bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return box, data
}

func TestKeysAndItems(t *testing.T) {
	box, _ := decodeHex(t, iPhoneKeys)
	keys, ok := box.(*mp4.KeysBox)
	if !ok || len(keys.Keys) != 4 {
		t.Fatalf("keys decoded as %T %+v", box, box)
	}
	if k, ok := keys.Key(1); !ok || k != (mp4.MetadataKey{Namespace: "mdta", Value: "com.apple.quicktime.make"}) {
		t.Errorf("key 1: %+v", k)
	}
	if _, ok := keys.Key(5); ok {
		t.Error("key 5 found")
	}

	box, _ = decodeHex(t, iPhoneIlst)
	ilst := box.(*mp4.IlstBox)
	if len(ilst.Children) != 4 {
		t.Fatalf("%d items", len(ilst.Children))
	}
	item, ok := ilst.Children[1].(*mp4.GenericContainerBox)
	if !ok || item.Type() != "\x00\x00\x00\x02" {
		t.Fatalf("item 2 decoded as %T", ilst.Children[1])
	}
	if k, _ := keys.KeyOfItem(item.Type()); k.Value != "com.apple.quicktime.model" {
		t.Errorf("key of item 2: %+v", k)
	}
	if data, ok := item.Children[0].(*mp4.DataBox); !ok || string(data.Data) != "iPhone 12 mini" {
		t.Errorf("value of item 2: %+v", item.Children[0])
	}
}

func TestKeysOfMebxIsUnknown(t *testing.T) {
	box, _ := decodeHex(t, mebxKeys)
	if _, ok := box.(*mp4.UnknownBox); !ok {
		t.Errorf("mebx keys decoded as %T", box)
	}
}

func TestKeysRoundTrip(t *testing.T) {
	keys := &mp4.KeysBox{Keys: []mp4.MetadataKey{{Namespace: "mdta", Value: "com.example.a"}, {Namespace: "udta", Value: "\xa9cpy"}}}
	boxDiffAfterEncodeAndDecode(t, keys)
	bad := &mp4.KeysBox{Keys: []mp4.MetadataKey{{Namespace: "md", Value: "x"}}}
	if err := bad.Encode(&bytes.Buffer{}); err == nil {
		t.Error("no error for a namespace of 2 bytes")
	}
}

func TestItunesItemsAreContainers(t *testing.T) {
	ilst := &mp4.IlstBox{}
	trkn := mp4.NewGenericContainerBox("trkn")
	trkn.AddChild(&mp4.DataBox{DataType: 0, Data: []byte{0, 0, 0, 3, 0, 12, 0, 0}})
	ilst.AddChild(trkn)
	boxDiffAfterEncodeAndDecode(t, ilst)
}

func TestItemKeysInInfo(t *testing.T) {
	hdlr := "0000002168646c7200000000000000006d647461" + strings.Repeat("00", 13)
	// The fourth item refers to key 5, which does not exist.
	ilst := strings.Replace(iPhoneIlst, "0000003000000004", "0000003000000005", 1)
	payload := hdlr + iPhoneKeys + ilst
	box, _ := decodeHex(t, fmt.Sprintf("%08x6d657461", 8+len(payload)/2)+payload)
	var info bytes.Buffer
	if err := box.Info(&info, "", "", "  "); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"\n    [\\x00\\x00\\x00\\x01] size=29\n     - key: mdta \"com.apple.quicktime.make\"\n      [data] size=21\n",
		"\n    [\\x00\\x00\\x00\\x05] size=48\n      [data] size=40\n",
	} {
		if !strings.Contains(info.String(), want) {
			t.Errorf("Info lacks %q:\n%s", want, info.String())
		}
	}

	// Without a keys box, the items have no keys.
	box, _ = decodeHex(t, iPhoneIlst)
	info.Reset()
	if err := box.Info(&info, "", "", "  "); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(info.String(), "key:") {
		t.Errorf("Info of ilst without keys has keys:\n%s", info.String())
	}
}
