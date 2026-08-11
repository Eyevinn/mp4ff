package mp4_test

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

func TestLabl(t *testing.T) {
	t.Run("encode and decode", func(t *testing.T) {
		labl := &mp4.LablBox{LabelID: 42, Language: "en-US", Label: "Director's commentary"}
		boxDiffAfterEncodeAndDecode(t, labl)
	})
	t.Run("encode and decode group label", func(t *testing.T) {
		labl := &mp4.LablBox{
			Flags:    mp4.LablIsGroupLabelFlag,
			LabelID:  1,
			Language: "zh-CN",
			Label:    "音轨",
		}
		boxDiffAfterEncodeAndDecode(t, labl)
		if !labl.IsGroupLabel() {
			t.Error("expected group label")
		}
	})
	t.Run("empty strings", func(t *testing.T) {
		labl := &mp4.LablBox{}
		if labl.IsGroupLabel() {
			t.Error("did not expect group label")
		}
		boxDiffAfterEncodeAndDecode(t, labl)
	})
	t.Run("decode raw bytes", func(t *testing.T) {
		rawHex := "000000196c61626c00000001" + "0002" + "656e2d555300" + "4d61696e00"
		rawBytes, err := hex.DecodeString(rawHex)
		if err != nil {
			t.Fatal(err)
		}
		box, err := mp4.DecodeBox(0, bytes.NewReader(rawBytes))
		if err != nil {
			t.Fatalf("error decoding labl box: %v", err)
		}
		labl, ok := box.(*mp4.LablBox)
		if !ok {
			t.Fatalf("expected *mp4.LablBox, got %T", box)
		}
		if labl.Size() != uint64(len(rawBytes)) {
			t.Errorf("expected size %d, got %d", len(rawBytes), labl.Size())
		}
		if !labl.IsGroupLabel() {
			t.Error("expected is_group_label to be set")
		}
		if labl.LabelID != 2 {
			t.Errorf("expected labelID 2, got %d", labl.LabelID)
		}
		if labl.Language != "en-US" {
			t.Errorf("expected language en-US, got %s", labl.Language)
		}
		if labl.Label != "Main" {
			t.Errorf("expected label Main, got %s", labl.Label)
		}
	})
	t.Run("bad data", func(t *testing.T) {
		cases := []struct {
			desc   string
			rawHex string
		}{
			{"too short payload", "0000000f6c61626c" + "00000000" + "0002" + "00"},
			{"unterminated language", "000000106c61626c" + "00000000" + "0002" + "6565"},
			{"unterminated label", "000000136c61626c" + "00000000" + "0002" + "656e00" + "4d61"},
		}
		for _, c := range cases {
			rawBytes, err := hex.DecodeString(c.rawHex)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := mp4.DecodeBox(0, bytes.NewReader(rawBytes)); err == nil {
				t.Errorf("%s: expected error from DecodeBox", c.desc)
			}
			sr := bits.NewFixedSliceReader(rawBytes)
			if _, err := mp4.DecodeBoxSR(0, sr); err == nil {
				t.Errorf("%s: expected error from DecodeBoxSR", c.desc)
			}
		}
	})
}
