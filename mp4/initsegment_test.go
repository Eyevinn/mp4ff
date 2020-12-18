package mp4

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"testing"
)

const sps1nalu = "67640020accac05005bb0169e0000003002000000c9c4c000432380008647c12401cb1c31380"

func parseInitFile(fileName string) (*File, error) {
	fd, err := os.Open(fileName)
	if err != nil {
		if err != nil {
			return nil, err
		}
	}
	defer fd.Close()

	f, err := DecodeFile(fd)
	if err != io.EOF && err != nil {
		if err != nil {
			return nil, err
		}
	}
	if f.IsFragmented() && f.Init.Ftyp == nil {
		return nil, fmt.Errorf("No ftyp present")
	}

	if f.isFragmented && len(f.Init.Moov.Traks) != 1 {
		return nil, fmt.Errorf("Not exactly one track")
	}
	return f, nil
}

// InitSegmentParsing - Check  to read a file with moov box.
func TestInitSegmentParsing(t *testing.T) {
	f, err := parseInitFile("testdata/init1.cmfv")
	if err != nil {
		t.Error(err)
	}
	url := f.Init.Moov.Trak.Mdia.Minf.Dinf.Dref.Children[0]
	if url.Type() != "url " {
		t.Errorf("Could not find url box")
	}
	avcC := f.Init.Moov.Trak.Mdia.Minf.Stbl.Stsd.AvcX.AvcC

	wanted := byte(31)
	got := avcC.AVCLevelIndication
	if got != wanted {
		t.Errorf("Got level %d insted of %d", got, wanted)
	}
}

func TestMoovParsingWithBtrtParsing(t *testing.T) {
	f, err := parseInitFile("testdata/init_prog.mp4")
	if err != nil {
		t.Error(err)
	}
	url := f.Moov.Trak.Mdia.Minf.Dinf.Dref.Children[0]
	if url.Type() != "url " {
		t.Errorf("Could not find url box")
	}
	avcx := f.Moov.Trak.Mdia.Minf.Stbl.Stsd.AvcX
	avcC := avcx.AvcC

	wanted := byte(31)
	got := avcC.AVCLevelIndication
	if got != wanted {
		t.Errorf("Got level %d insted of %d", got, wanted)
	}

	btrt := avcx.Btrt

	if btrt.AvgBitrate != 1384000 {
		t.Errorf("Got averate bitrate %d instead of %d", btrt.AvgBitrate, 1384000)
	}

}

// sps1nalu is already defined in avc_test.go
const pps1nalu = "68b5df20"

func TestGenerateInitSegment(t *testing.T) {
	sps, _ := hex.DecodeString(sps1nalu)
	spsData := [][]byte{sps}
	pps, _ := hex.DecodeString(pps1nalu)
	ppsData := [][]byte{pps}

	init := CreateEmptyMP4Init(180000, "video", "und")
	trak := init.Moov.Trak
	trak.SetAVCDescriptor("avc3", spsData, ppsData)
	width := trak.Mdia.Minf.Stbl.Stsd.AvcX.Width
	height := trak.Mdia.Minf.Stbl.Stsd.AvcX.Height
	if width != 1280 || height != 720 {
		t.Errorf("Did not get righ width and height")
	}
	// Write to a buffer so that we can read and check
	var buf bytes.Buffer
	err := init.Encode(&buf)
	if err != nil {
		t.Error(err)
	}

	initRead, err := DecodeFile(&buf)
	if err != io.EOF && err != nil {
		if err != nil {
			t.Error(err)
		}
	}
	if initRead.Moov.Size() != init.Moov.Size() {
		t.Errorf("Mismatch generated vs read moov size: %d != %d", init.Moov.Size(), initRead.Moov.Size())
	}

	// Next write to a file
	ofd, err := os.Create("testdata/out_init.cmfv")
	if err != nil {
		t.Error(err)
	}
	defer ofd.Close()
	err = init.Encode(ofd)
	if err != nil {
		t.Error(err)
	}
}
