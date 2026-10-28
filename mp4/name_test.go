package mp4_test

import (
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
	"github.com/go-test/deep"
)

func TestFreeformItem(t *testing.T) {
	// An ilst with a freeform item ---- whose mean, name and data are com.apple.iTunes, iTunSMPB and "x".
	box, _ := decodeHex(t, "00000051696c7374000000492d2d2d2d"+
		"0000001c6d65616e00000000636f6d2e6170706c652e6954756e6573"+
		"000000146e616d65000000006954756e534d5042"+
		"0000001164617461000000010000000078")
	item, ok := box.(*mp4.IlstBox).Children[0].(*mp4.GenericContainerBox)
	if !ok || len(item.Children) != 3 {
		t.Fatalf("item decoded as %T %+v", box.(*mp4.IlstBox).Children[0], box.(*mp4.IlstBox).Children[0])
	}
	if mean, ok := item.Children[0].(*mp4.MeanBox); !ok || mean.Domain != "com.apple.iTunes" {
		t.Errorf("mean decoded as %T %+v", item.Children[0], item.Children[0])
	}
	if name, ok := item.Children[1].(*mp4.NameBox); !ok || !name.FullBox || name.Name != "iTunSMPB" {
		t.Errorf("name decoded as %T %+v", item.Children[1], item.Children[1])
	}
	boxDiffAfterEncodeAndDecode(t, &mp4.MeanBox{Domain: "com.apple.iTunes"})
	boxDiffAfterEncodeAndDecode(t, &mp4.NameBox{FullBox: true, Name: "iTunNORM"})
	if box, _ := decodeHex(t, "0000000a6d65616e0000"); box.Type() != "mean" {
		t.Error("short mean has another type")
	} else if _, ok := box.(*mp4.UnknownBox); !ok {
		t.Errorf("short mean decoded as %T", box)
	}
}

func TestName(t *testing.T) {
	testCases := []struct {
		desc string
		hex  string
		want mp4.Box
	}{
		{"udta track name", "000000116e616d6548524546547261636b", &mp4.NameBox{Name: "HREFTrack"}},
		{"empty item name", "0000000c6e616d6500000000", &mp4.NameBox{FullBox: true}},
		{"empty udta name", "000000086e616d65", &mp4.NameBox{}},
		{"timecode source tape name", "000000116e616d650005000033303b3030",
			mp4.NewUserDataTextBox("name", mp4.UserDataText{Text: "30;00"})},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			box, _ := decodeHex(t, tc.hex)
			if diff := deep.Equal(box, tc.want); diff != nil {
				t.Errorf("decoded as %T %+v: %v", box, box, diff)
			}
		})
	}
	boxDiffAfterEncodeAndDecode(t, &mp4.NameBox{Name: "Sound Track"})
}
