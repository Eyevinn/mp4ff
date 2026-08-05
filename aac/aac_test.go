package aac

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/go-test/deep"
)

func encodeASC(t *testing.T, asc AudioSpecificConfig) []byte {
	t.Helper()
	buf := &bytes.Buffer{}
	if err := asc.Encode(buf); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestDecodeAudioSpecificConfigHeader(t *testing.T) {
	testCases := []struct {
		desc      string
		asc       []byte
		wanted    AudioSpecificConfigHeader
		wantedErr string
	}{
		{
			desc:   "AAC-LC 48000 stereo",
			asc:    []byte{0x11, 0x90},
			wanted: AudioSpecificConfigHeader{ObjectType: 2, SamplingFrequency: 48000, ChannelConfiguration: 2},
		},
		{
			desc: "HE-AACv1 24000 stereo (base frequency, not the extension)",
			asc: encodeASC(t, AudioSpecificConfig{ObjectType: HEAACv1, ChannelConfiguration: 2,
				SamplingFrequency: 24000, ExtensionFrequency: 48000, SBRPresentFlag: true}),
			wanted: AudioSpecificConfigHeader{ObjectType: 5, SamplingFrequency: 24000, ChannelConfiguration: 2},
		},
		{
			desc: "object type 39 (ER AAC ELD), unsupported by the full decoder",
			asc:  []byte{0xf8, 0xe8, 0x40},
			wanted: AudioSpecificConfigHeader{
				ObjectType: 39, SamplingFrequency: 44100, ChannelConfiguration: 2,
			},
		},
		{
			desc: "explicit 24-bit frequency",
			asc:  []byte{0x17, 0x80, 0x2e, 0x6c, 0x08},
			wanted: AudioSpecificConfigHeader{
				ObjectType: 2, SamplingFrequency: 23768, ChannelConfiguration: 1,
			},
		},
		{
			desc:      "invalid frequency index 13",
			asc:       []byte{0x16, 0x90},
			wantedErr: "strange frequency index",
		},
		{
			desc:      "truncated config",
			asc:       []byte{0x11},
			wantedErr: "audio specific config too short",
		},
		{
			desc:      "truncated inside the object type escape",
			asc:       []byte{0xf8},
			wantedErr: "audio specific config too short",
		},
		{
			desc:      "truncated inside the 24-bit frequency",
			asc:       []byte{0x17, 0x80},
			wantedErr: "audio specific config too short",
		},
		{
			desc:      "truncated at the channel configuration",
			asc:       []byte{0xf8, 0xe8},
			wantedErr: "audio specific config too short",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			hdr, err := DecodeAudioSpecificConfigHeader(bytes.NewReader(tc.asc))
			if tc.wantedErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantedErr) {
					t.Errorf("wanted error %q, got %v", tc.wantedErr, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if diff := deep.Equal(*hdr, tc.wanted); diff != nil {
				t.Error(diff)
			}
		})
	}
}

func TestAudioSpecificConfigEncodeDecode(t *testing.T) {

	testCases := []AudioSpecificConfig{
		{
			ObjectType:           AAClc,
			ChannelConfiguration: 2,
			SamplingFrequency:    48000,
			ExtensionFrequency:   0,
			SBRPresentFlag:       false,
			PSPresentFlag:        false,
		},
		{
			ObjectType:           HEAACv1,
			ChannelConfiguration: 2,
			SamplingFrequency:    24000,
			ExtensionFrequency:   48000,
			SBRPresentFlag:       true,
			PSPresentFlag:        false,
		},
		{
			ObjectType:           HEAACv2,
			ChannelConfiguration: 1,
			SamplingFrequency:    24000,
			ExtensionFrequency:   48000,
			SBRPresentFlag:       true,
			PSPresentFlag:        true,
		},
	}

	for _, asc := range testCases {

		buf := &bytes.Buffer{}
		err := asc.Encode(buf)
		if err != nil {
			t.Error(err)
		}
		ascBytes := buf.Bytes()
		t.Logf("ASC: %s\n", hex.EncodeToString(ascBytes))

		readBuf := bytes.NewBuffer(ascBytes)
		gotAsc, err := DecodeAudioSpecificConfig(readBuf)
		if err != nil {
			t.Error(err)
		}
		diff := deep.Equal(*gotAsc, asc)
		if diff != nil {
			t.Errorf("Diff %v for %+v", diff, asc)
		}
	}
}

func TestVariousInputs(t *testing.T) {

	testCases := []struct {
		desc          string
		data          []byte
		expectedError string
	}{
		{
			desc:          "unsupported object type",
			data:          []byte{0x0f, 0x00},
			expectedError: "unsupported object type: 1",
		},
		{
			desc:          "bad frequency index",
			data:          []byte{0x17, 0x30},
			expectedError: "strange frequency index",
		},
		{
			desc:          "too short extended frequency",
			data:          []byte{0x17, 0x80, 0x40},
			expectedError: "strange frequency index",
		},
	}

	for _, c := range testCases {
		t.Run(c.desc, func(t *testing.T) {
			readBuf := bytes.NewBuffer(c.data)
			_, err := DecodeAudioSpecificConfig(readBuf)
			if err == nil || err.Error() != c.expectedError {
				t.Errorf("Expected error: %s", c.expectedError)
			}
		})
	}

	t.Run("32768Hz", func(t *testing.T) {
		data := []byte{0x17, 0x80, 0x40, 0x00, 0x00}
		readBuf := bytes.NewBuffer(data)
		gotAsc, err := DecodeAudioSpecificConfig(readBuf)
		if err != nil {
			t.Error(err)
		}
		if gotAsc.SamplingFrequency != 32768 {
			t.Errorf("Expected 32768Hz, got %d", gotAsc.SamplingFrequency)
		}
	})
}
