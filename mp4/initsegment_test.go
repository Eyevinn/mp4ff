package mp4

import (
	"encoding/hex"
	"io"
	"os"
	"testing"
)

// InitSegmentParsing - Check
func TestInitSegmentParsing(t *testing.T) {
	fd, err := os.Open("test_data/init1.cmfv")
	if err != nil {
		if err != nil {
			t.Error(err)
		}
	}
	defer fd.Close()

	f, err := DecodeFile(fd)
	if err != io.EOF && err != nil {
		if err != nil {
			t.Error(err)
		}
	}
	if f.Init.Ftyp == nil {
		t.Errorf("No ftyp present")
	}
	if len(f.Init.Moov.Trak) != 1 {
		t.Errorf("Not exactly one track")
	}
	url := f.Init.Moov.Trak[0].Mdia.Minf.Dinf.Dref.Boxes[0]
	if url.Type() != "url " {
		t.Errorf("Could not find url box")
	}
	avcC := f.Init.Moov.Trak[0].Mdia.Minf.Stbl.Stsd.AvcX.AvcC

	wanted := byte(31)
	got := avcC.AVCLevelIndication
	if got != wanted {
		t.Errorf("Got level %d insted of %d", got, wanted)
	}
}

// sps1nalu is already defined in avc_test.go
const pps1nalu = "68b5df20"

func TestGenerateInitSegment(t *testing.T) {

	spsData, _ := hex.DecodeString(sps1nalu)
	pps, _ := hex.DecodeString(pps1nalu)
	ppsData := [][]byte{pps}

	init := CreateEmptyMP4Init(180000, "video", "und")
	trak := init.Moov.Trak[0]
	trak.SetAVCDescriptor("avc3", spsData, ppsData)
	width := trak.Mdia.Minf.Stbl.Stsd.AvcX.Width
	height := trak.Mdia.Minf.Stbl.Stsd.AvcX.Height
	if width != 1280 || height != 720 {
		t.Errorf("Did not get righ width and height")
	}
	// Next write to a file
	ofd, err := os.Create("test_data/out_init.cmfv")
	defer ofd.Close()
	if err != nil {
		t.Error(err)
	}
	init.Encode(ofd)
}
