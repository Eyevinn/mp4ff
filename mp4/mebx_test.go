package mp4_test

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/go-test/deep"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

// The sample entry of the video orientation track of an iPhone movie.
const iPhoneMebx = "000000756d656278" + "0000000000000001" + mebxKeys

func TestMebxOfIPhone(t *testing.T) {
	box, _ := decodeHex(t, iPhoneMebx)
	mebx, ok := box.(*mp4.MebxBox)
	if !ok {
		t.Fatalf("mebx decoded as %T", box)
	}
	if mebx.DataReferenceIndex != 1 || mebx.Keys == nil || len(mebx.Keys.Keys) != 1 || mebx.Btrt != nil {
		t.Fatalf("mebx: %+v", mebx)
	}
	key := mebx.Keys.Keys[0]
	if key.LocalKeyID != 1 || key.Type() != "\x00\x00\x00\x01" || len(key.Children) != 3 {
		t.Errorf("key: %+v", key)
	}
	if key.Keyd == nil || key.Keyd.Key() != (mp4.MetadataKey{Namespace: "mdta", Value: "com.apple.quicktime.video-orientation"}) {
		t.Errorf("keyd: %+v", key.Keyd)
	}
	if dataType, ok := key.Dtyp.WellKnownType(); !ok || dataType != 66 {
		t.Errorf("dtyp: %+v", key.Dtyp)
	}
	if _, ok := key.Children[2].(*mp4.UnknownBox); !ok || key.Children[2].Type() != "sdpd" {
		t.Errorf("third child of the key is %T %s", key.Children[2], key.Children[2].Type())
	}
	if key.Loca != nil || key.Setu != nil {
		t.Errorf("key has loca %v and setu %v", key.Loca, key.Setu)
	}

	var info strings.Builder
	if err := mebx.Info(&info, "", "", "  "); err != nil {
		t.Fatal(err)
	}
	want := `[mebx] size=117
 - dataReferenceIndex: 1
  [keys] size=101
    [\x00\x00\x00\x01] size=93
      [keyd] size=49
       - key: mdta "com.apple.quicktime.video-orientation"
      [dtyp] size=16
       - namespace: 0
       - dataType: 66 (BE 16-bit Signed Integer)
      [sdpd] size=20
       - not implemented or unknown box
`
	if info.String() != want {
		t.Errorf("got info:\n%swant:\n%s", info.String(), want)
	}
}

// testMebx returns a mebx box with two keys, a bit rate box and a box of an unknown type.
func testMebx() *mp4.MebxBox {
	key1 := &mp4.MetadataKeyBox{LocalKeyID: 1}
	key1.AddChild(&mp4.KeydBox{Namespace: "mdta", Value: "com.example.speed"})
	key1.AddChild(&mp4.DtypBox{Namespace: mp4.DtypNamespaceWellKnown, Value: []byte{0, 0, 0, 23}})
	key2 := &mp4.MetadataKeyBox{LocalKeyID: binary.BigEndian.Uint32([]byte("titl"))}
	key2.AddChild(&mp4.KeydBox{Namespace: "uiso", Value: "titl"})
	key2.AddChild(&mp4.LocaBox{Locale: "en-US"})
	key2.AddChild(&mp4.SetuBox{Data: []byte{1, 2, 3}})
	key2.AddChild(mp4.CreateUnknownBox("xtra", 12, []byte{0, 0, 0, 7}))
	mebx := &mp4.MebxBox{DataReferenceIndex: 1}
	mebx.AddChild(&mp4.MetadataKeyTableBox{Keys: []*mp4.MetadataKeyBox{key1, key2}})
	mebx.AddChild(&mp4.BtrtBox{BufferSizeDB: 100, MaxBitrate: 2000, AvgBitrate: 1000})
	mebx.AddChild(mp4.CreateUnknownBox("lidp", 12, []byte{0, 0, 0, 1}))
	return mebx
}

func TestMebxRoundTrip(t *testing.T) {
	mebx := testMebx()
	if mebx.Keys == nil || mebx.Btrt == nil || len(mebx.Children) != 3 {
		t.Fatalf("mebx after AddChild: %+v", mebx)
	}
	boxDiffAfterEncodeAndDecode(t, mebx)

	// A sample entry without children, which is the shortest one.
	boxDiffAfterEncodeAndDecode(t, &mp4.MebxBox{DataReferenceIndex: 2})

	keys := mebx.Keys
	if k := keys.Key(1); k != keys.Keys[0] {
		t.Errorf("key 1 is %+v", k)
	}
	if k := keys.Key(2); k != nil {
		t.Errorf("key 2 is %+v", k)
	}
	if k := keys.FindKey("uiso", "titl"); k != keys.Keys[1] || k.Type() != "titl" {
		t.Errorf("key uiso titl is %+v", k)
	}
	if k := keys.FindKey("mdta", "titl"); k != nil {
		t.Errorf("key mdta titl is %+v", k)
	}
	if k := keys.Keys[1]; k.Loca.Locale != "en-US" || !bytes.Equal(k.Setu.Data, []byte{1, 2, 3}) || k.Dtyp != nil {
		t.Errorf("key 2: %+v", k)
	}
}

func TestMebxInStsd(t *testing.T) {
	stsd := mp4.NewStsdBox()
	stsd.AddChild(testMebx())
	if stsd.Mebx == nil {
		t.Fatal("stsd.Mebx is not set by AddChild")
	}
	dec, ok := boxAfterEncodeAndDecode(t, stsd).(*mp4.StsdBox)
	if !ok || dec.Mebx == nil {
		t.Fatalf("stsd.Mebx is not set by decoding: %+v", dec)
	}
	if diff := deep.Equal(dec.Mebx, stsd.Mebx); diff != nil {
		t.Error(diff)
	}
}

// decodesAsUnknown checks that data decodes as an UnknownBox with both decoders, and encodes to data again.
func decodesAsUnknown(t *testing.T, data []byte) {
	t.Helper()
	box, err := mp4.DecodeBox(0, bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := box.(*mp4.UnknownBox); !ok {
		t.Errorf("DecodeBox gives %T", box)
	}
	box, err = mp4.DecodeBoxSR(0, bits.NewFixedSliceReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := box.(*mp4.UnknownBox); !ok {
		t.Errorf("DecodeBoxSR gives %T", box)
	}
	cmpAfterDecodeEncodeBox(t, data)
}

func TestMebxWithOtherLayoutIsUnknown(t *testing.T) {
	valid, err := hex.DecodeString(iPhoneMebx)
	if err != nil {
		t.Fatal(err)
	}
	// The positions of the size fields of the nested boxes: keys, the key box and keyd.
	const keysPos, keyPos, keydPos = 16, 24, 32
	cases := []struct {
		desc   string
		change func(d []byte) []byte
	}{
		{"payload of 7 bytes", func(d []byte) []byte {
			d = d[:15]
			d[3] = 15
			return d
		}},
		{"keys goes beyond the end of mebx", func(d []byte) []byte {
			d[keysPos+3]++
			return d
		}},
		{"key goes beyond the end of keys", func(d []byte) []byte {
			d[keyPos+3]++
			return d
		}},
		{"children of the key do not fill it", func(d []byte) []byte {
			d[keydPos+3]++
			return d
		}},
		{"7 bytes after the last child", func(d []byte) []byte {
			d = append(d, make([]byte, 7)...)
			d[3] += 7
			return d
		}},
		{"7 bytes after the last key", func(d []byte) []byte {
			d = append(d, make([]byte, 7)...)
			d[3] += 7
			d[keysPos+3] += 7
			return d
		}},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			decodesAsUnknown(t, c.change(append([]byte(nil), valid...)))
		})
	}
}

func TestDecodeMebxSample(t *testing.T) {
	sample := []byte{
		0, 0, 0, 10, 0, 0, 0, 1, 0, 6, // key 1: 2 bytes
		0, 0, 0, 8, 0, 0, 0, 0, // padding
		0, 0, 0, 12, 't', 'i', 't', 'l', 'a', 'b', 'c', 'd', // key titl: 4 bytes
	}
	values, err := mp4.DecodeMebxSample(sample)
	if err != nil {
		t.Fatal(err)
	}
	want := []mp4.MebxValue{
		{LocalKeyID: 1, Data: []byte{0, 6}},
		{LocalKeyID: 0, Data: []byte{}},
		{LocalKeyID: binary.BigEndian.Uint32([]byte("titl")), Data: []byte("abcd")},
	}
	if diff := deep.Equal(values, want); diff != nil {
		t.Error(diff)
	}

	values, err = mp4.DecodeMebxSample(nil)
	if err != nil || values != nil {
		t.Errorf("empty sample: %v, %v", values, err)
	}

	errCases := []struct {
		desc   string
		sample []byte
		want   string
	}{
		{"7 bytes", sample[:7], "mebx sample: 7 bytes after the last value"},
		{"7 bytes after a value", sample[:17], "mebx sample: 7 bytes after the last value"},
		{"size 7", []byte{0, 0, 0, 7, 0, 0, 0, 1}, "mebx sample: value of size 7 at 0 does not fit"},
		{"value one byte too long", sample[:len(sample)-1], "mebx sample: value of size 12 at 18 does not fit"},
		{"size above 2 GB", []byte{0xff, 0xff, 0xff, 0xff, 0, 0, 0, 1}, "mebx sample: value of size 4294967295 at 0 does not fit"},
	}
	for _, c := range errCases {
		if _, err := mp4.DecodeMebxSample(c.sample); err == nil || err.Error() != c.want {
			t.Errorf("%s: got error %v, want %q", c.desc, err, c.want)
		}
	}
}
