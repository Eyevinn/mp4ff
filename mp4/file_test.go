package mp4

import (
	"os"
	"testing"
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
			if frag.Mdat.decLazyDataSize == 0 {
				t.Error("decLazyDataSize is expected to be greater than 0")
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
			if frag.Mdat.decLazyDataSize != 0 {
				t.Error("decLazyDataSize is expected to be 0")
			}
			if frag.Mdat.Data == nil || len(frag.Mdat.Data) == 0 {
				t.Error("Mdat Data is expected to be non-nil")
			}
		}
	}

}
