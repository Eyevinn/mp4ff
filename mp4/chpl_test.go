package mp4_test

import (
	"testing"
	"time"

	"github.com/Eyevinn/mp4ff/mp4"
	"github.com/go-test/deep"
)

func TestChpl(t *testing.T) {
	testCases := []struct {
		desc string
		hex  string
		want []mp4.NeroChapter
	}{
		{"Nero", "0000001a6368706c0100000000000000010000000000102a5200",
			[]mp4.NeroChapter{{Start: 1059410}}},
		{"ffmpeg", "000000366368706c010000000000000002000000000000000005496e74726f00000000004c4b400e5365636f6e6420" +
			"63686170746572", []mp4.NeroChapter{{Title: "Intro"}, {Start: 5000000, Title: "Second chapter"}}},
		{"GPAC", "000000346368706c010000000000000002000000000000000005537461727400000000007270e00c4b61706974656c" +
			"207476c3a5", []mp4.NeroChapter{{Title: "Start"}, {Start: 7500000, Title: "Kapitel två"}}},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			box, _ := decodeHex(t, tc.hex)
			chpl, ok := box.(*mp4.ChplBox)
			if !ok || chpl.Version != 1 {
				t.Fatalf("decoded as %T %+v", box, box)
			}
			if diff := deep.Equal(chpl.Chapters, tc.want); diff != nil {
				t.Error(diff)
			}
		})
	}
	if got := (mp4.NeroChapter{Start: 7500000}).StartTime(); got != 750*time.Millisecond {
		t.Errorf("start time %v", got)
	}
	boxDiffAfterEncodeAndDecode(t, &mp4.ChplBox{Version: 1, Chapters: []mp4.NeroChapter{{Title: "One"}, {Start: 1}}})
	// A chpl without the reserved bytes before the number of chapters.
	if box, _ := decodeHex(t, "000000166368706c0000000001000000000000000000"); box.Type() != "chpl" {
		t.Error("chpl without reserved bytes has another type")
	} else if _, ok := box.(*mp4.UnknownBox); !ok {
		t.Errorf("chpl without reserved bytes decoded as %T", box)
	}
}
