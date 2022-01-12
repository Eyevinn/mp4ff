package mp4

import (
	"bytes"
	"io/ioutil"
	"os"
	"testing"

	"github.com/edgeware/mp4ff/bits"
)

func TestDecodeFileWithLazyMdatOption(t *testing.T) {

	// load a segment
	file, err := os.Open("./testdata/1.m4s")
	if err != nil {
		t.Error(err)
	}

	parsedFile, err := DecodeFile(file, WithDecodeMode(DecModeLazyMdat))
	if err != nil {
		t.Error(err)
	}

	for _, seg := range parsedFile.Segments {
		for _, frag := range seg.Fragments {
			if frag.Mdat.GetLazyDataSize() == 0 {
				t.Error("lazyDataSize is expected to be greater than 0")
			}
			if frag.Mdat.Data != nil {
				t.Error("Mdat Data is expected to be nil")
			}
		}
	}

}

func TestDecodeFileWithNoLazyMdatOption(t *testing.T) {

	// load a segment
	file, err := os.Open("./testdata/1.m4s")
	if err != nil {
		t.Error(err)
	}

	parsedFile, err := DecodeFile(file)
	if err != nil {
		t.Error(err)
	}

	for _, seg := range parsedFile.Segments {
		for _, frag := range seg.Fragments {
			if frag.Mdat.lazyDataSize != 0 {
				t.Error("decLazyDataSize is expected to be 0")
			}
			if frag.Mdat.Data == nil || len(frag.Mdat.Data) == 0 {
				t.Error("Mdat Data is expected to be non-nil")
			}
		}
	}
}

// TestExtractTrackSampleData both lazily and by reading in full file
func TestCopyTrackSampleData(t *testing.T) {
	// load a progressive file

	for j := 0; j < 2; j++ {
		fd, err := os.Open("./testdata/prog_8s.mp4")
		if err != nil {
			t.Error(err)
		}
		defer fd.Close()
		var mp4f *File
		if j == 0 {
			mp4f, err = DecodeFile(fd, WithDecodeMode(DecModeLazyMdat))
		} else {
			mp4f, err = DecodeFile(fd)
		}
		if err != nil {
			t.Error(err)
		}
		var startSampleNr uint32 = 31
		var endSampleNr uint32 = 60

		for _, trak := range mp4f.Moov.Traks {
			totSize := 0
			stsz := trak.Mdia.Minf.Stbl.Stsz
			for i := startSampleNr; i <= endSampleNr; i++ {
				totSize += int(stsz.GetSampleSize(int(i)))
			}
			sampleData := bytes.Buffer{}

			err := mp4f.CopySampleData(&sampleData, fd, trak, startSampleNr, endSampleNr)
			if err != nil {
				t.Error(err)
			}
			if sampleData.Len() != int(totSize) {
				t.Errorf("Got %d bytes instead of %d", sampleData.Len(), totSize)
			}
		}
	}
}

func TestDecodeEncodeProgressiveSliceWriter(t *testing.T) {
	// load a segment
	rawInput, err := ioutil.ReadFile("./testdata/prog_8s.mp4")
	if err != nil {
		t.Error(err)
	}
	rawOutput := make([]byte, len(rawInput))
	inBuf := bytes.NewBuffer(rawInput)
	parsedFile, err := DecodeFile(inBuf)
	if err != nil {
		t.Error(err)
	}
	sw := bits.NewFixedSliceWriterFromSlice(rawOutput)
	err = parsedFile.EncodeSW(sw)
	if err != nil {
		t.Error(err)
	}
	if !bytes.Equal(rawOutput, rawInput) {
		t.Errorf("output differs from input")
	}
}
