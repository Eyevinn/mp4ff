package mp4_test

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
	"github.com/go-test/deep"
)

func TestEncodeData(t *testing.T) {
	boxDiffAfterEncodeAndDecode(t, &mp4.DataBox{Data: []byte("dummy")})
	boxDiffAfterEncodeAndDecode(t, &mp4.DataBox{DataType: mp4.DataTypeUTF8, Country: 0x5345, Language: 0x15c7,
		Data: []byte("dummy")})
}

func TestDecodeData(t *testing.T) {
	cases := []struct {
		desc     string
		hex      string
		want     mp4.DataBox
		wantInfo string
	}{
		{
			desc:     "ffmpeg ©nam text",
			hex:      "0000001364617461000000010000000048656a",
			want:     mp4.DataBox{DataType: mp4.DataTypeUTF8, Data: []byte("Hej")},
			wantInfo: "[data] size=19\n - dataType: 1 (UTF-8)\n - value: \"Hej\"\n",
		},
		{
			desc:     "ffmpeg trkn track 3 of 12",
			hex:      "0000001864617461000000000000000000000003000c0000",
			want:     mp4.DataBox{DataType: mp4.DataTypeImplicit, Data: []byte{0, 0, 0, 3, 0, 0x0c, 0, 0}},
			wantInfo: "[data] size=24\n - dataType: 0 (implicit)\n - value: 00000003000c0000\n",
		},
		{
			desc:     "ffmpeg covr JPEG",
			hex:      "00000014646174610000000d00000000ffd8ffe0",
			want:     mp4.DataBox{DataType: mp4.DataTypeJPEG, Data: []byte{0xff, 0xd8, 0xff, 0xe0}},
			wantInfo: "[data] size=20\n - dataType: 13 (JPEG)\n - value: JPEG image, 4 bytes\n",
		},
		{
			desc:     "iTunes tmpo 190",
			hex:      "000000126461746100000015000000000" + "0be",
			want:     mp4.DataBox{DataType: mp4.DataTypeBESignedInt, Data: []byte{0, 0xbe}},
			wantInfo: "[data] size=18\n - dataType: 21 (BE Signed Integer)\n - value: 00be (190)\n",
		},
		{
			desc: "iPhone make with country DE and language deu",
			hex:  "000000156461746100000001444510b54170706c65",
			want: mp4.DataBox{DataType: mp4.DataTypeUTF8, Country: 0x4445, Language: 0x10b5, Data: []byte("Apple")},
			wantInfo: "[data] size=21\n - dataType: 1 (UTF-8)\n - locale: country=DE language=deu\n" +
				" - value: \"Apple\"\n",
		},
		{
			desc: "iPhone camera identifier with language eng",
			hex:  "000000146461746100000001000015c74261636b",
			want: mp4.DataBox{DataType: mp4.DataTypeUTF8, Language: 0x15c7, Data: []byte("Back")},
			wantInfo: "[data] size=20\n - dataType: 1 (UTF-8)\n - locale: country=0 language=eng\n" +
				" - value: \"Back\"\n",
		},
		{
			desc: "UTF-16 text with list indices as locale",
			hex:  "0000001664617461000000020002000300480065006a",
			want: mp4.DataBox{DataType: mp4.DataTypeUTF16, Country: 2, Language: 3, Data: []byte{0, 'H', 0, 'e', 0, 'j'}},
			wantInfo: "[data] size=22\n - dataType: 2 (UTF-16)\n" +
				" - locale: country=2 (list index) language=3 (list index)\n - value: \"Hej\"\n",
		},
		{
			desc:     "type outside the well-known list",
			hex:      "000000116461746100000063000000002a",
			want:     mp4.DataBox{DataType: 99, Data: []byte{0x2a}},
			wantInfo: "[data] size=17\n - dataType: 99\n - value: 2a\n",
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			data, err := hex.DecodeString(c.hex)
			if err != nil {
				t.Fatal(err)
			}
			decoders := []struct {
				name   string
				decode func() (mp4.Box, error)
			}{
				{"DecodeBox", func() (mp4.Box, error) { return mp4.DecodeBox(0, bytes.NewReader(data)) }},
				{"DecodeBoxSR", func() (mp4.Box, error) { return mp4.DecodeBoxSR(0, bits.NewFixedSliceReader(data)) }},
			}
			for _, d := range decoders {
				box, err := d.decode()
				if err != nil {
					t.Fatalf("%s: %v", d.name, err)
				}
				db, ok := box.(*mp4.DataBox)
				if !ok {
					t.Fatalf("%s: got %T, want *mp4.DataBox", d.name, box)
				}
				if diff := deep.Equal(*db, c.want); diff != nil {
					t.Errorf("%s: %v", d.name, diff)
				}
				var enc bytes.Buffer
				if err := db.Encode(&enc); err != nil {
					t.Fatalf("%s: %v", d.name, err)
				}
				if !bytes.Equal(enc.Bytes(), data) {
					t.Errorf("%s: encoded %x, want %x", d.name, enc.Bytes(), data)
				}
				sw := bits.NewFixedSliceWriter(int(db.Size()))
				if err := db.EncodeSW(sw); err != nil {
					t.Fatalf("%s: %v", d.name, err)
				}
				if !bytes.Equal(sw.Bytes(), data) {
					t.Errorf("%s: EncodeSW gave %x, want %x", d.name, sw.Bytes(), data)
				}
				var info bytes.Buffer
				if err := db.Info(&info, "", "", "  "); err != nil {
					t.Fatalf("%s: %v", d.name, err)
				}
				if info.String() != c.wantInfo {
					t.Errorf("%s: Info gave\n%s\nwant\n%s", d.name, info.String(), c.wantInfo)
				}
			}
		})
	}
}

func TestDecodeShortData(t *testing.T) {
	data, err := hex.DecodeString("0000000c6461746100000001")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mp4.DecodeBox(0, bytes.NewReader(data)); err == nil {
		t.Error("DecodeBox: no error for a data box without locale")
	}
	if _, err := mp4.DecodeBoxSR(0, bits.NewFixedSliceReader(data)); err == nil {
		t.Error("DecodeBoxSR: no error for a data box without locale")
	}
}

func TestDataMeaning(t *testing.T) {
	smpb := " 00000000 00000840 000001FD 000000000013B5C3 00000000 00000000"
	cases := []struct {
		item, name string
		box        mp4.DataBox
		want       string
	}{
		{"trkn", "", mp4.DataBox{Data: []byte{0, 0, 0, 2, 0, 21, 0, 0}}, "track 2 of 21"},
		{"disk", "", mp4.DataBox{Data: []byte{0, 0, 0, 1, 0, 2}}, "disc 1 of 2"},
		{"gnre", "", mp4.DataBox{Data: []byte{0, 0x12}}, "ID3v1 genre 17"},
		{"----", "iTunSMPB", mp4.DataBox{DataType: mp4.DataTypeUTF8, Data: []byte(smpb)},
			"encoder delay 2112, padding 509, 1291715 samples"},
		{"----", "iTunNORM", mp4.DataBox{DataType: mp4.DataTypeUTF8, Data: []byte(" 0000085B")}, ""},
		{"rtng", "", mp4.DataBox{DataType: mp4.DataTypeBESignedInt, Data: []byte{0xff}}, "-1"},
		{"plID", "", mp4.DataBox{DataType: mp4.DataTypeBEUnsignedInt, Data: []byte{0xff, 0xff}}, "65535"},
		{"covr", "", mp4.DataBox{DataType: mp4.DataTypePNG, Data: make([]byte, 531)}, "PNG image, 531 bytes"},
		{"", "", mp4.DataBox{Data: []byte{0, 0, 0, 2, 0, 21, 0, 0}}, ""},
	}
	for _, c := range cases {
		if got := c.box.Meaning(c.item, c.name); got != c.want {
			t.Errorf("%s %s: %q, want %q", c.item, c.name, got, c.want)
		}
	}
}

func TestDataInfoInIlst(t *testing.T) {
	box := func(typ string, payload ...[]byte) []byte {
		p := bytes.Join(payload, nil)
		return append(append(binary.BigEndian.AppendUint32(nil, uint32(8+len(p))), typ...), p...)
	}
	fullBox := func(typ, text string) []byte { return box(typ, []byte{0, 0, 0, 0}, []byte(text)) }
	data := func(dataType byte, value []byte) []byte {
		return box("data", []byte{0, 0, 0, dataType, 0, 0, 0, 0}, value)
	}
	// An ilst with trkn track 2 of 21 and a freeform iTunSMPB item.
	ilst := box("ilst", box("trkn", data(0, []byte{0, 0, 0, 2, 0, 21, 0, 0})),
		box("----", fullBox("mean", "com.apple.iTunes"), fullBox("name", "iTunSMPB"), data(1, []byte(" 0 840 1 0"))))
	for _, decoded := range []func() (mp4.Box, error){
		func() (mp4.Box, error) { return mp4.DecodeBox(0, bytes.NewReader(ilst)) },
		func() (mp4.Box, error) { return mp4.DecodeBoxSR(0, bits.NewFixedSliceReader(ilst)) },
	} {
		b, err := decoded()
		if err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		if err := b.Info(&buf, "", "", "  "); err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"value: 0000000200150000 (track 2 of 21)",
			`value: " 0 840 1 0" (encoder delay 2112, padding 1, 0 samples)`} {
			if !bytes.Contains(buf.Bytes(), []byte(want)) {
				t.Errorf("no %q in\n%s", want, buf.String())
			}
		}
	}
	var buf bytes.Buffer
	covr := &mp4.DataBox{DataType: mp4.DataTypeJPEG, Data: []byte{0xff, 0xd8, 0xff, 0xe0}}
	if err := covr.Info(&buf, "data:1", "", "  "); err != nil {
		t.Fatal(err)
	}
	if want := " - value: ffd8ffe0 (JPEG image, 4 bytes)\n"; !bytes.HasSuffix(buf.Bytes(), []byte(want)) {
		t.Errorf("level 1 image: %q", buf.String())
	}
}
