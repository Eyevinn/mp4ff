package mp4

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"testing"

	"github.com/go-test/deep"
)

const sps1nalu = "674d401fe4605017fcb80b4f00000300010000030032e4800753003a9e08200e58e189c0"
const pps1nalu = "685bdf20"

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

func TestGenerateInitSegment(t *testing.T) {
	goldenAssetPath := "testdata/init_video.golden"
	goldenDumpPath := "testdata/init_video_dump.golden"
	sps, _ := hex.DecodeString(sps1nalu)
	spsData := [][]byte{sps}
	pps, _ := hex.DecodeString(pps1nalu)
	ppsData := [][]byte{pps}

	init := CreateEmptyMP4Init(180000, "video", "und")
	trak := init.Moov.Trak
	trak.SetAVCDescriptor("avc3", spsData, ppsData)
	width := trak.Mdia.Minf.Stbl.Stsd.AvcX.Width
	height := trak.Mdia.Minf.Stbl.Stsd.AvcX.Height
	if width != 640 || height != 360 {
		t.Errorf("Got %dx%d instead of 640x360", width, height)
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

	// Regenerated buf
	err = init.Encode(&buf)
	if err != nil {
		t.Error(err)
	}

	var dumpBuf bytes.Buffer
	err = init.Dump(&dumpBuf, "all:1", "  ")
	if err != nil {
		t.Error(err)
	}

	// Generate or compare with golden files
	if *update {
		err = writeGolden(t, goldenAssetPath, buf.Bytes())
		if err != nil {
			t.Error(err)
		}
		err = writeGolden(t, goldenDumpPath, dumpBuf.Bytes())
		if err != nil {
			t.Error(err)
		}

		return
	}

	golden, err := ioutil.ReadFile(goldenAssetPath)
	if err != nil {
		t.Error(err)
	}
	diff := deep.Equal(golden, buf.Bytes())
	if diff != nil {
		t.Errorf("Generated init segment different from %s", goldenAssetPath)
	}

	goldenDump, err := ioutil.ReadFile(goldenDumpPath)
	if err != nil {
		t.Error(err)
	}
	diff = deep.Equal(goldenDump, dumpBuf.Bytes())
	if diff != nil {
		t.Errorf("Generated init segment different from %s", goldenDumpPath)
	}

}
