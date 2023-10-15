package av1

import (
	"encoding/hex"
	"testing"

	"github.com/go-test/deep"
)

const av1DecoderConfigRecord = "81094c000a0b0000004aabbfc377ffe701"
const configOBUs = "0a0b0000004aabbfc377ffe701"

func TestDecodeAV1DecConfRec(t *testing.T) {
	byteData, _ := hex.DecodeString(av1DecoderConfigRecord)
	configOBUsBytes, _ := hex.DecodeString(configOBUs)

	wanted := DecConfRec{
		Marker:                           1,
		Version:                          1,
		SeqProfile:                       0,
		SeqLevelIdx0:                     9,
		SeqTier0:                         0,
		HighBitdepth:                     1,
		TwelveBit:                        0,
		MonoChrome:                       0,
		ChromaSubsamplingX:               1,
		ChromaSubsamplingY:               1,
		ChromaSamplePosition:             0,
		InitialPresentationDelayPresent:  0,
		InitialPresentationDelayMinusOne: 0,
		ConfigOBUs:                       configOBUsBytes,
	}

	got, err := DecodeAV1DecConfRec(byteData)
	if err != nil {
		t.Error("Error parsing Av1DecoderConfigRecord")
	}
	if diff := deep.Equal(got, wanted); diff != nil {
		t.Error(diff)
	}
}
