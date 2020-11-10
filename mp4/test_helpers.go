package mp4

import (
	"bytes"
	"testing"

	"github.com/go-test/deep"
)

func boxDiffAfterEncodeAndDecode(t *testing.T, box Box) {
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

	if diff := deep.Equal(boxDec, box); diff != nil {
		t.Error(diff)
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
