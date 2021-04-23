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

	testCases := []struct {
		adtsBytes    []byte
		wantedHdr    *ADTSHeader
		wantedOffset int
		wantedError  error
	}{
		{
			adtsBytes:    adtsBytes,
			wantedHdr:    adtsHdrStart,
			wantedOffset: 0,
			wantedError:  nil,
		},
		{
			adtsBytes:    append([]byte{0xfe}, adtsBytes...),
			wantedHdr:    adtsHdrStart,
			wantedOffset: 1,
			wantedError:  nil,
		},
	}

	for _, tc := range testCases {
		gotHdr, gotOffset, gotErr := DecodeADTSHeader(bytes.NewBuffer(tc.adtsBytes))
		if gotErr != tc.wantedError {
			t.Errorf("Got error %s instead of %s", gotErr, tc.wantedError)
		}
		if gotOffset != tc.wantedOffset {
			t.Errorf("Got offset %d instead of %d", gotOffset, tc.wantedOffset)
		}
		if diff := deep.Equal(gotHdr, tc.wantedHdr); diff != nil {
			t.Error(diff)
		}
	}
}
