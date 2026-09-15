package mp4_test

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

func TestStpp(t *testing.T) {
	t.Run("encode and decode", func(t *testing.T) {
		testCases := []struct {
			namespace      string
			schemaLocation string
			mimeTypes      string
			hasBtrt        bool
		}{
			{
				namespace:      "NS",
				schemaLocation: "location",
				mimeTypes:      "image/png image/jpg",
				hasBtrt:        false,
			},
			{
				namespace:      "NS",
				schemaLocation: "location",
				mimeTypes:      "image/png image/jpg",
				hasBtrt:        true,
			},
			{
				namespace:      "NS",
				schemaLocation: "",
				mimeTypes:      "",
				hasBtrt:        false,
			},
			{
				namespace:      "NS",
				schemaLocation: "",
				mimeTypes:      "",
				hasBtrt:        true,
			},
		}
		for _, tc := range testCases {
			t.Logf("Test case: %+v", tc)
			stpp := mp4.NewStppBox(tc.namespace, tc.schemaLocation, tc.mimeTypes)
			if tc.hasBtrt {
				btrt := &mp4.BtrtBox{}
				stpp.AddChild(btrt)
				if stpp.Btrt != btrt {
					t.Error("Btrt link is broken")
				}
			}
			boxDiffAfterEncodeAndDecode(t, stpp)
		}
	})
	t.Run("empty lists", func(t *testing.T) {
		const hexData = ("00000040737470700000000000000001" +
			"687474703a2f2f7777772e77332e6f72" +
			"672f6e732f74746d6c00000000000014" +
			"62747274000003ce00003b5800000430")
		data, err := hex.DecodeString(hexData)
		if err != nil {
			t.Error(err)
		}
		sr := bits.NewFixedSliceReader(data)
		box, err := mp4.DecodeBoxSR(0, sr)
		if err != nil {
			t.Error(err)
		}
		stpp, ok := box.(*mp4.StppBox)
		if !ok {
			t.Error("not an stpp box")
		}
		if int(stpp.Size()) != len(data) {
			t.Errorf("stpp size %d not same as %d", stpp.Size(), len(data))
		}
		buf := bytes.Buffer{}
		err = stpp.Encode(&buf)
		if err != nil {
			t.Error(err)
		}
		outData := buf.Bytes()
		if !bytes.Equal(data, outData) {
			t.Error("written stpp box differs from read")
		}
	})
	t.Run("decode completely missing auxiliary mime types", func(t *testing.T) {
		hexData := ("0000002b737470700000000000000001" +
			"687474703a2f2f7777772e77332e6f72" +
			"672f6e732f74746d6c0000")
		hexData = strings.ReplaceAll(hexData, " ", "")
		data, err := hex.DecodeString(hexData)
		if err != nil {
			t.Error(err)
		}
		sr := bits.NewFixedSliceReader(data)
		box, err := mp4.DecodeBoxSR(0, sr)
		if err != nil {
			t.Error(err)
		}
		stpp, ok := box.(*mp4.StppBox)
		if !ok {
			t.Error("not an stpp box")
		}
		if int(stpp.Size()) != len(data) {
			t.Errorf("stpp size %d not same as %d", stpp.Size(), len(data))
		}
	})
}

func TestStpc(t *testing.T) {
	stpc := mp4.NewStpcBox("http://www.w3.org/ns/ttml", "", "")
	if stpc.Type() != "stpc" {
		t.Errorf("got type %s instead of stpc", stpc.Type())
	}

	boxDiffAfterEncodeAndDecode(t, stpc)

	// The name must survive a round trip, or the entry would turn into an stpp.
	// boxDiffAfterEncodeAndDecode does not compare unexported fields.
	buf := bytes.Buffer{}
	if err := stpc.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	box, err := mp4.DecodeBoxSR(0, bits.NewFixedSliceReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if box.Type() != "stpc" {
		t.Errorf("decoded type %s instead of stpc", box.Type())
	}
	if _, ok := box.(*mp4.StppBox); !ok {
		t.Errorf("decoded box is not a StppBox")
	}
}

func TestTtmn(t *testing.T) {
	ttmn := &mp4.TtmnBox{}
	if ttmn.Size() != 8 {
		t.Errorf("ttmn size %d instead of 8", ttmn.Size())
	}
	boxDiffAfterEncodeAndDecode(t, ttmn)

	// A no-change sample is the eight bytes 00 00 00 08 "ttmn".
	buf := bytes.Buffer{}
	if err := ttmn.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	if got := hex.EncodeToString(buf.Bytes()); got != "0000000874746d6e" {
		t.Errorf("ttmn sample is %s", got)
	}
}

func TestTtmb(t *testing.T) {
	body := `<body><div><p begin="00:00:10" region="r1">Hello</p></div></body>`
	ttmb := &mp4.TtmbBox{Body: body}
	if ttmb.Size() != uint64(8+len(body)) {
		t.Errorf("ttmb size %d instead of %d", ttmb.Size(), 8+len(body))
	}
	boxDiffAfterEncodeAndDecode(t, ttmb)

	// The body is a boxstring: no length prefix and no trailing zero byte.
	buf := bytes.Buffer{}
	if err := ttmb.Encode(&buf); err != nil {
		t.Fatal(err)
	}
	if got := string(buf.Bytes()[8:]); got != body {
		t.Errorf("ttmb body is %q", got)
	}
}
