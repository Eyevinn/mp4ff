package mp4_test

import (
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
	"github.com/go-test/deep"
)

func TestTextSampleEntries(t *testing.T) {
	serif := &mp4.FtabBox{Fonts: []mp4.FontRecord{{ID: 0, Name: "Serif"}}}
	tx3g := mp4.NewTextSampleEntryBox("tx3g")
	tx3g.DataReferenceIndex = 1
	tx3g.BackgroundColorRGBA = [4]byte{0xff, 0xff, 0xff, 0xff}
	tx3g.DefaultStyle = mp4.TextStyleRecord{FontSize: 12, TextColorRGBA: [4]byte{0, 0, 0, 0xff}}
	tx3g.AddChild(serif)

	// The text sample entry that ffmpeg writes for chapter tracks, with the layout of tx3g.
	ffmpegText := mp4.NewTextSampleEntryBox("text")
	ffmpegText.DataReferenceIndex = 1
	ffmpegText.DisplayFlags = 1
	ffmpegText.DefaultStyle = mp4.TextStyleRecord{FontID: 1}
	ffmpegText.AddChild(&mp4.FtabBox{Fonts: []mp4.FontRecord{{ID: 1}}})

	testCases := []struct {
		desc string
		hex  string
		want mp4.Box
	}{
		{"tx3g of a chapter track", "0000004074783367000000000000000100000000" +
			"0000ffffffff0000000000000000000000000000000c000000ff000000126674616200010000055365726966", tx3g},
		{"text with the tx3g layout", "0000003b74657874000000000000000100000001" +
			"00000000000000000000000000000000000000010000000000000000000d667461620001000100", ffmpegText},
		{"QuickTime text", "0000003b746578740000000000000001000000000000000100000000000000000000006200a0" +
			"000000000010000e00010000000cffffffffffff00",
			&mp4.QuickTimeTextSampleEntryBox{DataReferenceIndex: 1, TextJustification: 1,
				DefaultTextBox: mp4.TextBoxRecord{Bottom: 98, Right: 160}, LineHeight: 16, Ascent: 14,
				FontNumber: 1, FontSize: 12, ForegroundColor: [3]uint16{0xffff, 0xffff, 0xffff}}},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			box, _ := decodeHex(t, tc.hex)
			if diff := deep.Equal(box, tc.want); diff != nil {
				t.Errorf("decoded as %T %+v: %v", box, box, diff)
			}
		})
	}

	// The undocumented text atom in the gmhd of a text track, as ffmpeg writes it.
	box, _ := decodeHex(t, "0000002c74657874000100000000000000000000000000000001000000000000000000000000000040000000")
	if _, ok := box.(*mp4.UnknownBox); !ok || box.Type() != "text" {
		t.Errorf("gmhd text atom decoded as %T", box)
	}

	stsd := mp4.NewStsdBox()
	stsd.AddChild(tx3g)
	if stsd.Tx3g != tx3g {
		t.Error("stsd.Tx3g is not set")
	}
	boxDiffAfterEncodeAndDecode(t, tx3g)
	boxDiffAfterEncodeAndDecode(t, &mp4.QuickTimeTextSampleEntryBox{DataReferenceIndex: 1, TextJustification: -1,
		FontFace: 1, FontSize: 18, FontName: "Helvetica", Children: []mp4.Box{serif}})
	boxDiffAfterEncodeAndDecode(t, &mp4.FtabBox{Fonts: []mp4.FontRecord{{ID: 1, Name: "Sans-serif"}, {ID: 2}}})
	if box, _ := decodeHex(t, "00000010667461620002000105414243"); box.Type() != "ftab" {
		t.Error("ftab with a missing font has another type")
	} else if _, ok := box.(*mp4.UnknownBox); !ok {
		t.Errorf("ftab with a missing font decoded as %T", box)
	}
}
