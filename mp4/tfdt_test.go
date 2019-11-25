package mp4

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func TestTfdtReadingV0(t *testing.T) {
	byteData, _ := hex.DecodeString("00000010746664740000000000ffffff")

	r := bytes.NewReader(byteData[8:]) // Don't include header
	bHdr := &boxHeader{
		name:   "tfdt",
		size:   uint64(len(byteData)/2 + 8),
		hdrlen: 8,
	}
	box, _ := DecodeTfdt(bHdr, 0, r)
	tfdt := box.(*TfdtBox)

	if tfdt.Version != 0 {
		t.Errorf("Tfdt version is not 0")
	}
	if tfdt.BaseMediaDecodeTime != 0x00ffffff {
		t.Errorf("Tfdt basemediaDecodeTime is %v not %v", tfdt.BaseMediaDecodeTime, 0x00ffffff)
	}
}

func TestTfdtReadingV1(t *testing.T) {
	byteData, _ := hex.DecodeString("0000001474666474010000000000000000ffffff")

	r := bytes.NewReader(byteData[8:]) // Don't include header
	bHdr := &boxHeader{
		name:   "tfdt",
		size:   uint64(len(byteData)/2 + 8),
		hdrlen: 8,
	}
	box, _ := DecodeTfdt(bHdr, 0, r)
	tfdt := box.(*TfdtBox)

	if tfdt.Version != 1 {
		t.Errorf("Tfdt version is not 1")
	}
	if tfdt.BaseMediaDecodeTime != 0x00ffffff {
		t.Errorf("Tfdt basemediaDecodeTime is %v not %v", tfdt.BaseMediaDecodeTime, 0x00ffffff)
	}
}

func TestTfdtWriteV1(t *testing.T) {
	byteData, _ := hex.DecodeString("0000001474666474010000000000000000ffffff")

	r := bytes.NewReader(byteData[8:]) // Don't include header
	bHdr := &boxHeader{
		name:   "tfdt",
		size:   uint64(len(byteData)/2 + 8),
		hdrlen: 8,
	}
	box, _ := DecodeTfdt(bHdr, 0, r)
	tfdt := box.(*TfdtBox)

	outBuf := make([]byte, 0, tfdt.Size())

	w := bytes.NewBuffer(outBuf)
	tfdt.Encode(w)

	writtenBytes := w.Bytes()

	if bytes.Compare(byteData, writtenBytes) != 0 {
		t.Errorf("Encoded tfdt body not same as decoded")
	}

}
