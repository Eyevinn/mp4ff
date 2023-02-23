package mp4

import (
	"bytes"
	"io/ioutil"
	"os"
	"testing"

	"github.com/Eyevinn/mp4ff/aac"
	"github.com/Eyevinn/mp4ff/bits"
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

// TestCopyTrackSampleData checks that full early read and lazy with and without workSpace gives good and same result.
func TestCopyTrackSampleData(t *testing.T) {
	// load a progressive file
	testCases := []struct {
		lazy          bool
		workSpaceSize int
	}{
		{lazy: false, workSpaceSize: 0},
		{lazy: true, workSpaceSize: 0},
		{lazy: true, workSpaceSize: 256},
	}
	sampleDataRead := make([][]byte, 0, len(testCases))
	for j, tc := range testCases {
		fd, err := os.Open("./testdata/prog_8s.mp4")
		if err != nil {
			t.Error(err)
		}
		defer fd.Close()
		var mp4f *File
		var workSpace []byte
		if tc.lazy {
			mp4f, err = DecodeFile(fd, WithDecodeMode(DecModeLazyMdat))
			workSpace = make([]byte, tc.workSpaceSize)
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

			err := mp4f.CopySampleData(&sampleData, fd, trak, startSampleNr, endSampleNr, workSpace)
			if err != nil {
				t.Error(err)
			}
			if sampleData.Len() != int(totSize) {
				t.Errorf("Got %d bytes instead of %d", sampleData.Len(), totSize)
			}
			if trak.Tkhd.TrackID == 1 {
				sampleDataRead = append(sampleDataRead, sampleData.Bytes())
				if len(sampleDataRead) > 1 {
					if res := bytes.Compare(sampleDataRead[j], sampleDataRead[0]); res != 0 {
						t.Errorf("sample data read differs %d", res)
					}
				}
			}
		}
	}
}

func TestDecodeEncode(t *testing.T) {
	testFiles := []string{"./testdata/prog_8s.mp4", "./testdata/multi_sidx_segment.m4s"}

	for _, testFile := range testFiles {
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

		// SliceWriter case:
		sw := bits.NewFixedSliceWriterFromSlice(rawOutput)
		err = parsedFile.EncodeSW(sw)
		if err != nil {
			t.Error(err)
		}
		if !bytes.Equal(rawOutput, rawInput) {
			t.Errorf("encode differs from input for EncodeSW() and %s", testFile)
		}

		// io.Writer case
		rawOutput = rawOutput[:0]
		outBuf := bytes.NewBuffer(rawOutput)
		err = parsedFile.Encode(outBuf)
		if err != nil {
			t.Error(err)
		}
		if !bytes.Equal(outBuf.Bytes(), rawInput) {
			t.Errorf("encode differs from input for Encode() and %s", testFile)
		}
	}
}

func TestFilesWithEmsg(t *testing.T) {
	// File with ftyp, moov, styp, emsg, emsg, moof, mdat, moof, mdat
	init := CreateEmptyInit()
	init.AddEmptyTrack(uint32(48000), "audio", "en")
	trak := init.Moov.Trak
	err := trak.SetAACDescriptor(aac.AAClc, 48000)
	if err != nil {
		t.Error(err)
	}
	data := make([]byte, 0, init.Size())
	buf := bytes.NewBuffer(data)
	err = init.Encode(buf)
	if err != nil {
		t.Error(err)
	}
	seg := NewMediaSegment()
	frag, err := CreateFragment(1, 1)
	if err != nil {
		t.Error(err)
	}
	frag.AddFullSample(FullSample{
		Sample: Sample{
			Flags:                 0x0,
			Dur:                   1024,
			Size:                  6,
			CompositionTimeOffset: 0,
		},
		DecodeTime: 0,
		Data:       []byte{0, 1, 2, 3, 4, 5},
	})
	emsg1 := EmsgBox{ID: 1}
	frag.AddEmsg(&emsg1)
	emsg2 := EmsgBox{ID: 2}
	frag.AddEmsg(&emsg2)
	seg.AddFragment(frag)
	err = seg.Encode(buf)
	if err != nil {
		t.Error(err)
	}
	decFile, err := DecodeFile(buf)
	if err != nil {
		t.Error(err)
	}
	if len(decFile.Segments) != 1 {
		t.Error("not 1 segment in file")
	}
	if len(decFile.Segments[0].Fragments) != 1 {
		t.Error("not 1 fragment in segment")
	}
	dFrag := decFile.Segments[0].Fragments[0]
	if len(dFrag.Emsgs) != 2 {
		t.Error("not 2 emsg boxes in fragment")
	}
	if dFrag.Emsgs[0].ID != 1 {
		t.Error("first emsg box does not have index 1")
	}
	if dFrag.Emsgs[1].ID != 2 {
		t.Error("second emsg box does not have index 2")
	}
}
