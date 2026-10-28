package mp4_test

import (
	"encoding/hex"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

// The gmhd atom and the stsd of the timecode track of a QuickTime movie with Apple Pixlet video at 25 Hz.
const (
	timecodeGmhd = "00000056676d686400000018676d696e0000000000408000800080000000000000000036746d63640000002e7463" +
		"6d690000000000000000000c0000000000000000ffffffffffff0d4c7563696461204772616e6465"
	timecodeStsd = "0000003273747364000000000000000100000022746d636400000000000000010000000000000000000000190000" +
		"00011900"
)

func TestTimecodeMediaInformation(t *testing.T) {
	box, _ := decodeHex(t, timecodeGmhd)
	gmhd, ok := box.(*mp4.GenericContainerBox)
	if !ok || gmhd.Type() != "gmhd" || len(gmhd.Children) != 2 {
		t.Fatalf("gmhd decoded as %T %+v", box, box)
	}
	gmin, ok := gmhd.Children[0].(*mp4.GminBox)
	if !ok || gmin.GraphicsMode != 0x40 || gmin.Opcolor != [3]uint16{0x8000, 0x8000, 0x8000} {
		t.Errorf("gmin: %T %+v", gmhd.Children[0], gmhd.Children[0])
	}
	tmcd, ok := gmhd.Children[1].(*mp4.GenericContainerBox)
	if !ok || tmcd.Type() != "tmcd" || len(tmcd.Children) != 1 {
		t.Fatalf("tmcd in gmhd: %T %+v", gmhd.Children[1], gmhd.Children[1])
	}
	tcmi, ok := tmcd.Children[0].(*mp4.TcmiBox)
	if !ok || tcmi.TextSize != 12 || tcmi.FontName != "Lucida Grande" || tcmi.BackgroundColor != [3]uint16{0xffff, 0xffff, 0xffff} {
		t.Errorf("tcmi: %T %+v", tmcd.Children[0], tmcd.Children[0])
	}
}

func TestTimecodeSampleEntry(t *testing.T) {
	box, _ := decodeHex(t, timecodeStsd)
	stsd := box.(*mp4.StsdBox)
	entry := stsd.Tmcd
	if entry == nil || entry.DataReferenceIndex != 1 || entry.TimeScale != 25 || entry.FrameDuration != 1 ||
		entry.NumberOfFrames != 25 {
		t.Fatalf("tmcd entry: %+v", stsd.Children[0])
	}
	name := mp4.CreateUnknownBox("name", 20, []byte{0, 8, 0x15, 0xc7, 'T', 'a', 'p', 'e', ' ', '0', '0', '1'})
	boxDiffAfterEncodeAndDecode(t, &mp4.TimecodeSampleEntryBox{DataReferenceIndex: 1,
		Flags: mp4.TimecodeFlagDropFrame | mp4.TimecodeFlag24HourMax, TimeScale: 30000, FrameDuration: 1001,
		NumberOfFrames: 30, Reserved2: []byte{0}, Children: []mp4.Box{name}})
	boxDiffAfterEncodeAndDecode(t, &mp4.TimecodeSampleEntryBox{DataReferenceIndex: 1, TimeScale: 25, FrameDuration: 1,
		NumberOfFrames: 25, Reserved2: []byte{0, 0, 0}})

	// A timecode sample entry with three reserved bytes after the number of frames, as in the specification.
	box, _ = decodeHex(t, "00000024746d636400000000000000010000000000000000000032000000020019000000")
	if entry, ok := box.(*mp4.TimecodeSampleEntryBox); !ok || entry.TimeScale != 12800 || len(entry.Reserved2) != 3 {
		t.Errorf("entry with 3 reserved bytes: %T %+v", box, box)
	}
	boxDiffAfterEncodeAndDecode(t, &mp4.GminBox{GraphicsMode: 0x40, Opcolor: [3]uint16{1, 2, 3}, Balance: -1})
	boxDiffAfterEncodeAndDecode(t, &mp4.TcmiBox{TextSize: 12, FontName: "Helvetica"})

	box, _ = decodeHex(t, "00000010746d63640000000000000001") // too short for a sample entry
	if _, ok := box.(*mp4.UnknownBox); !ok {
		t.Errorf("short tmcd decoded as %T", box)
	}
}

// TestDecodeWithDataAfter decodes atoms whose decoders find the end of their payload, with more data after
// them in the SliceReader, as when they are children of other boxes.
func TestDecodeWithDataAfter(t *testing.T) {
	for _, h := range []string{timecodeGmhd, iPhoneKeys} {
		data, err := hex.DecodeString(h)
		if err != nil {
			t.Fatal(err)
		}
		withMore := append(append([]byte(nil), data...), 0, 0, 0, 8, 'f', 'r', 'e', 'e')
		box, err := mp4.DecodeBoxSR(0, bits.NewFixedSliceReader(withMore))
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := box.(*mp4.UnknownBox); ok || box.Size() != uint64(len(data)) {
			t.Errorf("%s decoded as %T of size %d", box.Type(), box, box.Size())
		}
		if g, ok := box.(*mp4.GenericContainerBox); ok {
			if _, ok := g.Children[1].(*mp4.GenericContainerBox).Children[0].(*mp4.TcmiBox); !ok {
				t.Errorf("tcmi in gmhd decoded as %T", g.Children[1].(*mp4.GenericContainerBox).Children[0])
			}
		}
	}
}
