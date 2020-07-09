package mp4

import (
	"errors"
	"io"

	"github.com/edgeware/mp4ff/bits"
)

const (
	// AAClc - AAC-LC Low Complexity
	AAClc = 2
	// HEAACv1 - HE-AAC version 1 with SBR
	HEAACv1 = 5
	// HEAACv2 - HE-AAC version 2 with SBR and PS
	HEAACv2 = 29
)

// AudioSpecificConfig according to ISO/IES 14496-3
// Syntax specified in Table 1.15
type AudioSpecificConfig struct {
	ObjectType           byte
	ChannelConfiguration byte // Defined in Table 1.19
	SamplingFrequency    int
	ExtensionFrequency   int
	SBRPresentFlag       bool
	PSPresentFlag        bool
}

var frequencyTable = map[byte]int{
	0:  96000,
	1:  88200,
	2:  64000,
	3:  48000,
	4:  44100,
	5:  32000,
	6:  24000,
	7:  22050,
	8:  16000,
	9:  12000,
	10: 11025,
	11: 8000,
	12: 7350,
}

var reverseFrequencies = map[int]byte{
	96000: 0,
	88200: 1,
	64000: 2,
	48000: 3,
	44100: 4,
	32000: 5,
	24000: 6,
	22050: 7,
	16000: 8,
	12000: 9,
	11025: 10,
	8000:  11,
	7350:  12,
}

// DecodeAudioSpecificConfig -
func DecodeAudioSpecificConfig(r io.Reader) (AudioSpecificConfig, error) {
	rb := bits.NewReader(r)

	asf := AudioSpecificConfig{}
	audioObjectType := byte(rb.MustRead(5))
	switch audioObjectType {
	case AAClc, HEAACv1, HEAACv2:
		// All fine
	default:
		return asf, errors.New("Only LC, HE-AACv1, and HE-AACv2 supported")
	}
	asf.ObjectType = audioObjectType
	frequency, ok := getFrequency(rb)
	if !ok {
		return asf, errors.New("Strange frequency index")
	}
	asf.SamplingFrequency = frequency
	asf.ChannelConfiguration = byte(rb.MustRead(4))
	switch audioObjectType {
	case HEAACv1, HEAACv2:
		frequency, ok := getFrequency(rb)
		if !ok {
			return asf, errors.New("Strange frequency index")
		}
		asf.ExtensionFrequency = frequency
		audioObjectType = byte(rb.MustRead(4))
		if audioObjectType == 22 {
			return asf, errors.New("ExtensionChannelConfiguration not supported")
		}
	}
	switch audioObjectType {
	case AAClc:
		// Read GASpecificConfig - Table 4.1 in ISO/IEC 14496-3 part 4
		frameLengthFlag := rb.MustReadFlag()
		if !frameLengthFlag {
			return asf, errors.New("Does not support frameLengthFlag = 1")
		}
		dependsOnCoreCoder := rb.MustReadFlag()
		if dependsOnCoreCoder {
			return asf, errors.New("Does not support dependsOnCoreCoder")
		}
		extensionFlag := rb.MustReadFlag()
		if extensionFlag {
			return asf, errors.New("Does not support dependsOnCoreCoder")
		}
	default:
		panic("Cannot handle audioObjectType")
	}
	// Done (there may be trailing bits)
	return asf, nil
}

// Encode - write AudioSpecificConfig to w for AAC-LC and HE-AAC
func (a *AudioSpecificConfig) Encode(w io.Writer) error {
	switch a.ObjectType {
	case AAClc, HEAACv1, HEAACv2:
		// fine
	default:
		return errors.New("Unsupported audio object type")
	}
	bw := bits.NewWriter(w)
	bw.Write(uint(a.ObjectType), 5)
	samplingIndex, ok := reverseFrequencies[a.SamplingFrequency]
	if ok {
		bw.Write(uint(samplingIndex), 4)
	} else {
		bw.Write(0x0f, 4)
		bw.Write(uint(a.SamplingFrequency), 24)
	}
	bw.Write(uint(a.ChannelConfiguration), 4)
	switch a.ObjectType {
	case HEAACv1, HEAACv2:
		samplingIndex, ok := reverseFrequencies[a.ExtensionFrequency]
		if ok {
			bw.Write(uint(samplingIndex), 4)
		} else {
			bw.Write(0x0f, 4)
			bw.Write(uint(a.ExtensionFrequency), 24)
		}
		bw.Write(AAClc, 5) // audioObjectType
	}
	bw.Write(0x00, 3) // GASpecificConfig
	bw.Flush()
	return bw.Error()
}

// getFrequency - either from 4-bit index or 24-bit value
func getFrequency(rb *bits.Reader) (frequency int, ok bool) {
	frequencyIndex, err := rb.Read(4)
	if err != nil {
		return 0, false
	}
	if frequencyIndex == 0x0f {
		f, err := rb.Read(24)
		if err != nil {
			return 0, false
		}
		return int(f), true
	}
	frequency, ok = frequencyTable[byte(frequencyIndex)]
	return frequency, ok
}
