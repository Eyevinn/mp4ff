package mp4

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"testing"
)

func TestMediaSegmentFragmentation(t *testing.T) {

	trex := &TrexBox{
		TrackID: 2,
	}

	//fd, err := os.Open("test_data/1.m4s")
	fd, err := os.Open("test_data/1_frag.m4s")
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
	if len(f.Segments) != 1 {
		t.Errorf("Not exactly one mediasegment")
	}
	mediaSegment := f.Segments[0]
	var timeScale uint64 = 90000
	var duration uint32 = 45000

	fragments, err := mediaSegment.Fragmentify(timeScale, trex, duration)
	if err != nil {
		t.Errorf("Fragmentation went wrong")
	}
	if len(fragments) != 4 {
		t.Errorf("%d fragments instead of 4", len(fragments))
	}

	// Write to a buffer so that we can read and check
	var buf bytes.Buffer
	f.Segments[0].Styp.Encode(&buf)
	for _, frag := range fragments {
		frag.Encode(&buf)
	}

	inFileContent, err := ioutil.ReadFile("test_data/1_frag.m4s")
	if err != nil {
		t.Errorf("Could not read test content")
	}
	outFileContent := buf.Bytes()
	if bytes.Compare(outFileContent, inFileContent) != 0 {
		t.Errorf("Wanted outfile len %d but got len %d", len(inFileContent), len(outFileContent))
	}
}
