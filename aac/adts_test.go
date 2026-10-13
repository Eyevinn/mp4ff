package aac

import (
	"bytes"
	"encoding/hex"
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
		adtsBytes       []byte
		wantedHdr       *ADTSHeader
		wantedOffset    int
		wantedFrequency uint16
		wantedError     error
	}{
		{
			adtsBytes:       adtsBytes,
			wantedHdr:       adtsHdrStart,
			wantedOffset:    0,
			wantedFrequency: 48000,
		},
		{
			adtsBytes:       append([]byte{0xfe}, adtsBytes...),
			wantedHdr:       adtsHdrStart,
			wantedOffset:    1,
			wantedFrequency: 48000,
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
		if tc.wantedFrequency != gotHdr.Frequency() {
			t.Errorf("Got frequency %d instead of %d", gotHdr.Frequency(), tc.wantedFrequency)
		}
		if diff := deep.Equal(gotHdr, tc.wantedHdr); diff != nil {
			t.Error(diff)
		}
	}
}

func TestAdtsHdrLengthWithCRC(t *testing.T) {
	hexData := "fff84c802cdffc1183"
	data, err := hex.DecodeString(hexData)
	if err != nil {
		t.Error(err)
	}
	gotHdr, gotOffset, err := DecodeADTSHeader(bytes.NewBuffer(data))
	if err != nil {
		t.Error(err)
	}
	if gotOffset != 0 {
		t.Errorf("Got offset %d instead of 0", gotOffset)
	}
	if gotHdr.HeaderLength != 9 {
		t.Errorf("Got header length %d instead of 9", gotHdr.HeaderLength)
	}
}

func TestAdtsHeaderParsingWithDoubleFF(t *testing.T) {
	hexData := "48fffff94cb02b5ffc21aa14"
	data, err := hex.DecodeString(hexData)
	if err != nil {
		t.Error(err)
	}
	_, gotOffset, err := DecodeADTSHeader(bytes.NewBuffer(data))
	if err != nil {
		t.Error(err)
	}
	wantedOffset := 2
	if gotOffset != wantedOffset {
		t.Errorf("Got offset %d instead of %d", gotOffset, wantedOffset)
	}
}

// TestADTSFrameLengthShorterThanHeader checks that a frame_length below the header length,
// 7 bytes or 9 bytes with CRC, is an error, while a frame of only a header has payload length 0.
func TestADTSFrameLengthShorterThanHeader(t *testing.T) {
	setFrameLength := func(hdr []byte, frameLength uint) []byte {
		b := append([]byte{}, hdr...)
		b[3] = (b[3] &^ 0x03) | byte(frameLength>>11)
		b[4] = byte(frameLength >> 3)
		b[5] = (b[5] & 0x1f) | byte(frameLength&7)<<5
		return b
	}
	noCRC, err := NewADTSHeader(48000, 2, AAClc, 412)
	if err != nil {
		t.Fatal(err)
	}
	withCRC, err := hex.DecodeString("fff84c802cdffc1183")
	if err != nil {
		t.Fatal(err)
	}
	testCases := []struct {
		desc        string
		hdr         []byte
		frameLength uint
		wantErr     bool
	}{
		{"7-byte header, frame length 3", noCRC.Encode(), 3, true},
		{"7-byte header, frame length 6", noCRC.Encode(), 6, true},
		{"7-byte header, frame length 7", noCRC.Encode(), 7, false},
		{"9-byte header, frame length 8", withCRC, 8, true},
		{"9-byte header, frame length 9", withCRC, 9, false},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			hdr, _, err := DecodeADTSHeader(bytes.NewReader(setFrameLength(tc.hdr, tc.frameLength)))
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected an error, got payload length %d", hdr.PayloadLength)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if hdr.PayloadLength != 0 {
				t.Errorf("got payload length %d instead of 0", hdr.PayloadLength)
			}
		})
	}
}
