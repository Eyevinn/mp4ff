package mp4

import (
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
	avcC := f.Init.Moov.Trak[0].Mdia.Minf.Stbl.Stsd.AvcX.AvcC

	wanted := byte(31)
	got := avcC.AVCLevelIndication
	if got != wanted {
		t.Errorf("Got level %d insted of %d", got, wanted)
	}
}
