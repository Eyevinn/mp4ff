package aac

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/go-test/deep"
)

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
		fmt.Printf("ASC: %s\n", hex.EncodeToString(ascBytes))

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
