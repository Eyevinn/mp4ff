package mp4

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"testing"

	"github.com/go-test/deep"
)

func TestMediaSegmentFragmentation(t *testing.T) {

	trex := &TrexBox{
		TrackID: 2,
	}

	inFile := "testdata/1.m4s"
	inFileGoldenDumpPath := "testdata/golden_1_m4s_dump.txt"
	goldenFragPath := "testdata/golden_1_frag.m4s"
	goldenFragDumpPath := "testdata/golden_1_frag_m4s_dump.txt"
	fd, err := os.Open(inFile)
	//fd, err := os.Open("testdata/1_frag.m4s")
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

	var bufInSeg bytes.Buffer
	err = f.Encode(&bufInSeg)
	if err != nil {
		t.Error(err)
	}

	inSeg, err := ioutil.ReadFile(inFile)
	if err != nil {
		t.Error(err)
	}

	diff := deep.Equal(inSeg, bufInSeg.Bytes())
	if diff != nil {
		t.Errorf("Written segment differs from %s", inFile)
	}

	err = compareOrUpdateInfo(t, f, inFileGoldenDumpPath)

	if err != nil {
		t.Error(err)
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

	var bufFrag bytes.Buffer
	fragmentedSegment := NewMediaSegment()
	fragmentedSegment.Styp = f.Segments[0].Styp
	fragmentedSegment.Fragments = fragments

	err = fragmentedSegment.Encode(&bufFrag)
	if err != nil {
		t.Error(err)
	}

	err = compareOrUpdateInfo(t, fragmentedSegment, goldenFragDumpPath)
	if err != nil {
		t.Error(err)
	}

	if *update {
		err = writeGolden(t, goldenFragPath, bufFrag.Bytes())
		if err != nil {
			t.Error(err)
		}
	} else {
		goldenFrag, err := ioutil.ReadFile(goldenFragPath)
		if err != nil {
			t.Error(err)
		}
		diff := deep.Equal(goldenFrag, bufFrag.Bytes())
		if diff != nil {
			t.Errorf("Generated dump different from %s", goldenFragPath)
		}
	}
}
