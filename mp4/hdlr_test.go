package mp4_test

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

func TestHdlr(t *testing.T) {

	cases := []struct {
		mediaType     string
		handlerType   string
		handlerName   string
		expectedError string
	}{
		{"video", "vide", "mp4ff video handler", ""},
		{"vide", "vide", "mp4ff video handler", ""},
		{"audio", "soun", "mp4ff audio handler", ""},
		{"soun", "soun", "mp4ff audio handler", ""},
		{"subtitle", "subt", "mp4ff subtitle handler", ""},
		{"text", "text", "mp4ff text handler", ""},
		{"wvtt", "text", "mp4ff text handler", ""},
		{"meta", "meta", "mp4ff timed metadata handler", ""},
		{"clcp", "subt", "mp4ff closed captions handler", ""},
		{"roses", "", "", "handler type is not four characters: roses"},
		{"auxv", "auxv", "mp4ff auxv handler", ""},
	}

	for _, c := range cases {
		t.Run(c.mediaType, func(t *testing.T) {
			hdlr, err := mp4.CreateHdlr(c.mediaType)
			if c.expectedError != "" {
				if err == nil {
					t.Errorf("Expected error %s, but got nil", c.expectedError)
				} else if err.Error() != c.expectedError {
					t.Errorf("Expected error %s, but got %s", c.expectedError, err.Error())
				}
				return
			}
			if hdlr.HandlerType != c.handlerType {
				t.Errorf("Expected handler type %s, but got %s", c.handlerType, hdlr.HandlerType)
			}
			if hdlr.Name != c.handlerName {
				t.Errorf("Expected handler name %s, but got %s", c.handlerName, hdlr.Name)
			}
			boxDiffAfterEncodeAndDecode(t, hdlr)
			hdlr.LacksNullTermination = true
			boxDiffAfterEncodeAndDecode(t, hdlr)
		})
	}
}

func TestHdlrDecodeMissingNullTermination(t *testing.T) {
	hdlrExample := "0000002068646C72000000000000000049443332000000000000000000000000"
	byteData, _ := hex.DecodeString(hdlrExample)
	buf := bytes.NewBuffer(byteData)
	box, err := mp4.DecodeBox(0, buf)
	if err != nil {
		t.Error(err)
	}
	hdlr := box.(*mp4.HdlrBox)
	if hdlr.Size() != uint64(len(byteData)) {
		t.Errorf("Got size %d instead of %d", hdlr.Size(), len(byteData))
	}
	if hdlr.Name != "" {
		t.Errorf("Expected empty name, but got %s", hdlr.Name)
	}
}

// TestDecodeMdirHandler tests decoding of a mdir handler box
// found in output from ffmpeg when encoding Opus in ISOBMFF.
func TestDecodeMdirHandler(t *testing.T) {
	mdirHdlrHex := "0000002168646c7200000000000000006d6469726170706c000000000000000000"
	byteData, _ := hex.DecodeString(mdirHdlrHex)
	buf := bytes.NewBuffer(byteData)
	box, err := mp4.DecodeBox(0, buf)
	if err != nil {
		t.Error(err)
	}
	mdirHdlr := box.(*mp4.HdlrBox)
	if mdirHdlr.HandlerType != "mdir" {
		t.Errorf("Expected handler type 'mdir', but got %s", mdirHdlr.HandlerType)
	}
	if mdirHdlr.Name != "" {
		t.Errorf("Expected empty name, but got %s", mdirHdlr.Name)
	}
	if mdirHdlr.Reserved != [3]uint32{0x6170706c, 0, 0} { // "appl"
		t.Errorf("Expected reserved [appl 0 0], but got %08x", mdirHdlr.Reserved)
	}
	out := bytes.Buffer{}
	if err := mdirHdlr.Encode(&out); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out.Bytes(), byteData) {
		t.Errorf("Encoded %x instead of %x", out.Bytes(), byteData)
	}
	boxDiffAfterEncodeAndDecode(t, mdirHdlr)
	info := bytes.Buffer{}
	if err := mdirHdlr.Info(&info, "", "", "  "); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(info.String(), " - reserved: 6170706c 00000000 00000000") {
		t.Errorf("Info lacks the reserved words: %s", info.String())
	}
}

func TestQuickTimeHdlr(t *testing.T) {
	mhlr, dhlr := uint32(0x6d686c72), uint32(0x64686c72)
	cases := []struct {
		desc          string
		hdlr          mp4.HdlrBox
		componentType string
		name          string
		counted       bool
	}{
		{"counted", mp4.HdlrBox{PreDefined: mhlr, Name: "\x05Video", LacksNullTermination: true}, "mhlr", "Video", true},
		{"counted with null", mp4.HdlrBox{PreDefined: dhlr, Name: "\x05Alias"}, "dhlr", "Alias", true},
		{"counted with zeros", mp4.HdlrBox{PreDefined: mhlr, Name: "\x05Sound\x00"}, "mhlr", "Sound", true},
		{"empty counted", mp4.HdlrBox{PreDefined: dhlr, Name: ""}, "dhlr", "", true},
		{"no name", mp4.HdlrBox{PreDefined: mhlr, LacksNullTermination: true}, "mhlr", "", true},
		{"length beyond name", mp4.HdlrBox{PreDefined: mhlr, Name: "\x06Video", LacksNullTermination: true}, "mhlr", "", false},
		{"bytes after counted", mp4.HdlrBox{PreDefined: mhlr, Name: "\x04Video", LacksNullTermination: true}, "mhlr", "", false},
		{"C string", mp4.HdlrBox{PreDefined: mhlr, Name: "VideoHandler"}, "mhlr", "", false},
		{"ISO", mp4.HdlrBox{Name: "VideoHandler"}, "", "", false},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			c.hdlr.HandlerType = "vide"
			if got := c.hdlr.QuickTimeComponentType(); got != c.componentType {
				t.Errorf("component type %q, want %q", got, c.componentType)
			}
			name, ok := c.hdlr.QuickTimeComponentName()
			if name != c.name || ok != c.counted {
				t.Errorf("component name %q %t, want %q %t", name, ok, c.name, c.counted)
			}
			boxDiffAfterEncodeAndDecode(t, &c.hdlr)
		})
	}
}

func TestQuickTimeHdlrInfo(t *testing.T) {
	// The media handler of an audio track written by Core Media.
	box, _ := decodeHex(t, "0000003168646c72000000006d686c72736f756e6170706c000000000000000010436f7265204d6564696120417564696f")
	hdlr := box.(*mp4.HdlrBox)
	var info bytes.Buffer
	if err := hdlr.Info(&info, "", "", "  "); err != nil {
		t.Fatal(err)
	}
	want := "[hdlr] size=49 version=0 flags=000000\n" +
		" - componentType: mhlr (media handler)\n" +
		" - handlerType: soun\n" +
		" - componentManufacturer: appl (0x6170706c), componentFlags: 00000000, componentFlagsMask: 00000000\n" +
		" - componentName: \"Core Media Audio\"\n"
	if info.String() != want {
		t.Errorf("got\n%s\nwant\n%s", info.String(), want)
	}

	cases := []struct {
		desc string
		hdlr mp4.HdlrBox
		want string
	}{
		{"data handler without counted name",
			mp4.HdlrBox{PreDefined: 0x64686c72, HandlerType: "alis", Reserved: [3]uint32{0, 1, 2}, Name: "Alias"},
			" - componentType: dhlr (data handler)\n - handlerType: alis\n" +
				" - componentManufacturer: 0x00000000, componentFlags: 00000001, componentFlagsMask: 00000002\n" +
				" - handlerName: \"Alias\"\n"},
		{"unknown pre_defined",
			mp4.HdlrBox{PreDefined: 1, HandlerType: "vide", Name: "Video"},
			" - preDefined: 00000001\n - handlerType: vide\n - handlerName: \"Video\"\n"},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			var info bytes.Buffer
			if err := c.hdlr.Info(&info, "", "", "  "); err != nil {
				t.Fatal(err)
			}
			if _, lines, _ := strings.Cut(info.String(), "\n"); lines != c.want {
				t.Errorf("got\n%s\nwant\n%s", lines, c.want)
			}
		})
	}
}
