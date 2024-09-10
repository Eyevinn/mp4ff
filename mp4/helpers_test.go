package mp4

import (
	"bytes"
	"flag"
	"os"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/go-test/deep"
)

// Helpers to tests. By including t.Helper(), the right failing line in the test
// itself is reported.

var (
	update = flag.Bool("update", false, "update the golden files of this test")
)

// cmpAfterDecodeEncodeBox compares bytes after a box has been decoded and encoded.
func cmpAfterDecodeEncodeBox(t *testing.T, data []byte) {
	t.Helper()

	// First SliceReader + SliceWriter
	sr := bits.NewFixedSliceReader(data)
	box, err := DecodeBoxSR(0, sr)
	if err != nil {
		t.Error(err)
	}
	sw := bits.NewFixedSliceWriter(int(box.Size()))
	err = box.EncodeSW(sw)
	if err != nil {
		t.Error(err)
	}
	if !bytes.Equal(data, sw.Bytes()) {
		t.Error("non-matching DecodeBoxSR/EncodeSW")
	}

	bufIn := bytes.NewBuffer(data)
	box, err = DecodeBox(0, bufIn)
	if err != nil {
		t.Error(err)
	}
	bufOut := bytes.Buffer{}
	err = box.Encode(&bufOut)
	if err != nil {
		t.Error(err)
	}
	if !bytes.Equal(data, bufOut.Bytes()) {
		t.Error("non-matchin DecodeBox/Encode")
	}
}

// boxDiffAfterEncodeAndDecode compares a box after encoding and decoding using deep.Equal().
// Further check that Info can be run without an error.
func boxDiffAfterEncodeAndDecode(t *testing.T, box Box) {
	t.Helper()

	// First do encode in a slice via SliceWriter
	size := box.Size()
	sw := bits.NewFixedSliceWriter(int(size))
	err := box.EncodeSW(sw)
	if err != nil {
		t.Error(err)
	}
	buf := bytes.NewBuffer(sw.Bytes())

	boxDec, err := DecodeBox(0, buf)
	if err != nil {
		t.Error(err)
	}

	if diff := deep.Equal(boxDec, box); diff != nil {
		t.Error(diff)
	}

	// Then do encode using io.Writer
	midBuf := bytes.Buffer{}
	err = box.Encode(&midBuf)
	if err != nil {
		t.Error(err)
	}
	// and decode using SliceReader
	sr := bits.NewFixedSliceReader(midBuf.Bytes())
	boxDec, err = DecodeBoxSR(0, sr)
	if err != nil {
		t.Error(err)
	}

	if diff := deep.Equal(boxDec, box); diff != nil {
		t.Error(diff)
	}
	// Finally check that the Info method does not render an error

	infoBuf := bytes.Buffer{}
	err = box.Info(&infoBuf, "all:1", "", "  ")
	if err != nil {
		t.Error(err)
	}
}

func boxAfterEncodeAndDecode(t *testing.T, box Box) Box {
	t.Helper()
	buf := bytes.Buffer{}
	err := box.Encode(&buf)
	if err != nil {
		t.Error(err)
	}

	boxDec, err := DecodeBox(0, &buf)
	if err != nil {
		t.Error(err)
	}
	return boxDec
}

func assertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Errorf("Got error %s but expected none", err)
	}
}

func assertError(t *testing.T, err error, msg string) {
	t.Helper()
	if err == nil {
		t.Error(msg)
	}
}

// writeGolden - write golden file that to be used for later tests
func writeGolden(t *testing.T, goldenAssetPath string, data []byte) error {
	t.Helper()
	fd, err := os.Create(goldenAssetPath)
	if err != nil {
		return err
	}
	_, err = fd.Write(data)
	if err != nil {
		return err
	}
	return nil
}

// TestMain is to set flags for tests. In particular, the update flag to update golden files.
func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}
