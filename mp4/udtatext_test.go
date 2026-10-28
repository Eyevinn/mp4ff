package mp4_test

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

// quickTimeUdta is the udta box of a QuickTime file written by ffmpeg, with the user data text atoms ©nam and ©swr.
const quickTimeUdta = "00000038756474610000001" + "8a96e616d000c55c46d70346c656e732074657374" +
	"00000018a9737772000c55c44c61766636322e312e313032"

// decodeBoth decodes data with DecodeBox and DecodeBoxSR, checks that both give the same box, and that it
// encodes to data.
func decodeBoth(t *testing.T, data []byte) mp4.Box {
	t.Helper()
	box, err := mp4.DecodeBox(0, bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	boxSR, err := mp4.DecodeBoxSR(0, bits.NewFixedSliceReader(data))
	if err != nil {
		t.Fatal(err)
	}
	var out, outSR bytes.Buffer
	if err := box.Encode(&out); err != nil {
		t.Fatal(err)
	}
	if err := boxSR.Encode(&outSR); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out.Bytes(), data) || !bytes.Equal(outSR.Bytes(), data) {
		t.Errorf("encoded\n%x\n%x\nwant\n%x", out.Bytes(), outSR.Bytes(), data)
	}
	return box
}

func TestUserDataTextRoundTrip(t *testing.T) {
	boxDiffAfterEncodeAndDecode(t, mp4.NewUserDataTextBox("\xa9nam",
		mp4.UserDataText{Language: 0x55c4, Text: "A title"}, mp4.UserDataText{Language: 0, Text: "Rubrik"}))
	boxDiffAfterEncodeAndDecode(t, mp4.NewUserDataTextBox("\xa9cmt", mp4.UserDataText{Language: 0x7fff, Text: "x"}))
}

func TestUserDataTextInUdta(t *testing.T) {
	data, err := hex.DecodeString(quickTimeUdta)
	if err != nil {
		t.Fatal(err)
	}
	udta, ok := decodeBoth(t, data).(*mp4.UdtaBox)
	if !ok || len(udta.Children) != 2 {
		t.Fatalf("got %T with children %v", udta, udta)
	}
	nam, ok := udta.Children[0].(*mp4.UserDataTextBox)
	if !ok || nam.Type() != "\xa9nam" || len(nam.Texts) != 1 {
		t.Fatalf("first child %T %+v", udta.Children[0], udta.Children[0])
	}
	if txt := nam.Texts[0]; txt.Text != "mp4lens test" || txt.LanguageString() != "und" {
		t.Errorf("text %q language %s", txt.Text, txt.LanguageString())
	}
	if swr, ok := udta.Children[1].(*mp4.UserDataTextBox); !ok || swr.Texts[0].Text != "Lavf62.1.102" {
		t.Errorf("second child %T %+v", udta.Children[1], udta.Children[1])
	}
	var info bytes.Buffer
	if err := udta.Info(&info, "", "", "  "); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(info.String(), `- text: language=und "mp4lens test"`) {
		t.Errorf("info:\n%s", info.String())
	}
}

// itunesItem returns an ilst item of type typ with a data box of type 1 (UTF-8) holding value.
func itunesItem(typ string, value []byte) []byte {
	dataBox := binary.BigEndian.AppendUint32(nil, uint32(16+len(value)))
	dataBox = append(dataBox, "data\x00\x00\x00\x01\x00\x00\x00\x00"...)
	dataBox = append(dataBox, value...)
	item := binary.BigEndian.AppendUint32(nil, uint32(8+len(dataBox)))
	return append(append(item, typ...), dataBox...)
}

func TestItunesItemsAreNotUserDataText(t *testing.T) {
	// The four types that are decoded as containers stay containers.
	if box := decodeBoth(t, itunesItem("\xa9nam", []byte("Title"))); box == nil {
		t.Fatal("no box")
	} else if _, ok := box.(*mp4.GenericContainerBox); !ok {
		t.Errorf("ilst ©nam decoded as %T", box)
	}
	// An item with a large value, so that the bytes after the data box size make a text size that fits, is not
	// taken as text, since the bytes "ta" of the data box type are not a valid language code.
	value := bytes.Repeat([]byte("la "), 9000)
	if box := decodeBoth(t, itunesItem("\xa9lyr", value)); box == nil {
		t.Fatal("no box")
	} else if _, ok := box.(*mp4.UnknownBox); !ok {
		t.Errorf("ilst ©lyr decoded as %T", box)
	}
}

func TestNotUserDataText(t *testing.T) {
	for name, payload := range map[string]string{
		"empty texts":         "00000000",
		"bad iso language":    "0002" + "0401" + "6869", // letters 0x01, 0x00, 0x01: the second is not a letter
		"top bit set":         "0002" + "d5c4" + "6869",
		"size beyond the end": "0009" + "55c4" + "6869",
		"short item":          "0002" + "55c4" + "686900",
	} {
		p, err := hex.DecodeString(payload)
		if err != nil {
			t.Fatal(err)
		}
		data := append(binary.BigEndian.AppendUint32(nil, uint32(8+len(p))), "\xa9cmt"...)
		box := decodeBoth(t, append(data, p...))
		if _, ok := box.(*mp4.UnknownBox); !ok {
			t.Errorf("%s: decoded as %T", name, box)
		}
	}
}

func TestUserDataTextLanguageAndUTF16(t *testing.T) {
	cases := []struct {
		text mp4.UserDataText
		lang string
		utf8 string
	}{
		{mp4.UserDataText{Language: 0x15c7, Text: "plain"}, "eng", "plain"},
		{mp4.UserDataText{Language: 0, Text: "plain"}, "mac:0", "plain"},
		{mp4.UserDataText{Language: 0x7fff, Text: "plain"}, "mac:unspecified", "plain"},
		{mp4.UserDataText{Language: 0x55c4, Text: "\xfe\xff\x00h\x00\xe9"}, "und", "hé"},
	}
	for _, c := range cases {
		if got := c.text.LanguageString(); got != c.lang {
			t.Errorf("LanguageString of %#x = %s, want %s", c.text.Language, got, c.lang)
		}
		if got := c.text.UTF8(); got != c.utf8 {
			t.Errorf("UTF8 of %q = %q, want %q", c.text.Text, got, c.utf8)
		}
	}
}

func TestUserDataTextTooLong(t *testing.T) {
	box := mp4.NewUserDataTextBox("\xa9cmt", mp4.UserDataText{Language: 0x55c4, Text: strings.Repeat("x", 0x10000)})
	if err := box.EncodeSW(bits.NewFixedSliceWriter(int(box.Size()))); err == nil {
		t.Error("no error for a text of 65536 bytes")
	}
}
