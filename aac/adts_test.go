package aac

import (
	"bytes"
	"testing"

	"github.com/go-test/deep"
)

func TestADTS(t *testing.T) {

	adtsHdrStart, err := NewADTSHeader(48000, 2, 2, 412)
	if err != nil {
		t.Error(err)
	}

	adtsBytes := adtsHdrStart.Encode()

	buf := bytes.NewBuffer(adtsBytes)

	adtsHdrEnd, err := DecodedAdtsHeader(buf)
	if err != nil {
		t.Error(err)
	}

	diff := deep.Equal(adtsHdrEnd, adtsHdrStart)
	if diff != nil {
		t.Error(diff)
	}

}
