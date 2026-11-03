package aac

import (
	"fmt"

	"github.com/Eyevinn/mp4ff/bits"
)

// USACConfig is the UsacConfig() of a USAC (xHE-AAC) AudioSpecificConfig,
// defined in ISO/IEC 23003-3:2022 Table 7.
//
// Data holds all bits from the start of the UsacConfig to the end of the
// AudioSpecificConfig, and Encode writes them back unchanged. The other
// fields are the leading fields decoded from Data. Changing them does not
// change what Encode writes.
type USACConfig struct {
	SamplingFrequency         int  // Output sampling frequency, from Table 72 or the 24-bit escape
	CoreSbrFrameLengthIndex   byte // Core coder frame length and SBR ratio, Table 75
	ChannelConfigurationIndex byte // Channel configuration, Table 73. 0 means that NumOutChannels gives the count
	NumOutChannels            int  // Present if ChannelConfigurationIndex is 0
	Data                      []byte
	NrBits                    int // Number of bits in Data, starting with the most significant bit of Data[0]
}

// usacFrequencies maps usacSamplingFrequencyIndex to the sampling frequency
// according to ISO/IEC 23003-3:2022 Table 72. Reserved indices map to 0.
var usacFrequencies = [31]int{
	96000, 88200, 64000, 48000, 44100, 32000, 24000, 22050, 16000, 12000, 11025, 8000, 7350, 0, 0,
	57600, 51200, 40000, 38400, 34150, 28800, 25600, 20000, 19200, 17075, 14400, 12800, 9600, 0, 0, 0,
}

// usacFrameLengths maps coreSbrFrameLengthIndex to coreCoderFrameLength and
// outputFrameLength according to ISO/IEC 23003-3:2022 Table 75.
var usacFrameLengths = [5][2]int{{768, 768}, {1024, 1024}, {768, 2048}, {1024, 2048}, {1024, 4096}}

// CoreCoderFrameLength returns the core coder frame length given by
// CoreSbrFrameLengthIndex, or 0 for a reserved index.
func (u *USACConfig) CoreCoderFrameLength() int {
	if int(u.CoreSbrFrameLengthIndex) >= len(usacFrameLengths) {
		return 0
	}
	return usacFrameLengths[u.CoreSbrFrameLengthIndex][0]
}

// OutputFrameLength returns the number of output samples per frame at
// SamplingFrequency given by CoreSbrFrameLengthIndex, or 0 for a reserved index.
func (u *USACConfig) OutputFrameLength() int {
	if int(u.CoreSbrFrameLengthIndex) >= len(usacFrameLengths) {
		return 0
	}
	return usacFrameLengths[u.CoreSbrFrameLengthIndex][1]
}

// NrChannels returns the number of channels given by ChannelConfigurationIndex
// according to ISO/IEC 23003-3:2022 Table 73, NumOutChannels for index 0, or 0
// for a reserved index. Indices 1 to 7 have the same meaning as the
// channelConfiguration of an AudioSpecificConfig, but 8 to 13 do not.
func (u *USACConfig) NrChannels() int {
	switch u.ChannelConfigurationIndex {
	case 0:
		return u.NumOutChannels
	case 1, 2, 3, 4, 5, 6, 7:
		return NrChannels(u.ChannelConfigurationIndex)
	case 8:
		return 2
	case 9:
		return 3
	case 10:
		return 4
	case 11:
		return 7
	case 12:
		return 8
	case 13:
		return 24
	default:
		return 0
	}
}

// decodeUSACConfig keeps the remaining bits of br, which holds nrBits bits,
// and decodes their leading UsacConfig fields.
func decodeUSACConfig(br *bits.Reader, nrBits int) (*USACConfig, error) {
	u := &USACConfig{NrBits: nrBits}
	u.Data = make([]byte, (nrBits+7)/8)
	for i := range u.Data {
		n := min(8, nrBits-8*i)
		u.Data[i] = byte(br.Read(n) << (8 - n))
	}
	if br.AccError() != nil {
		return nil, fmt.Errorf("USAC config: %w", br.AccError())
	}

	ur := bits.NewReaderFromSlice(u.Data)
	frequencyIndex := ur.Read(5)
	if frequencyIndex == 0x1f {
		u.SamplingFrequency = int(ur.Read(24))
	} else {
		u.SamplingFrequency = usacFrequencies[frequencyIndex]
	}
	u.CoreSbrFrameLengthIndex = byte(ur.Read(3))
	u.ChannelConfigurationIndex = byte(ur.Read(5))
	if u.ChannelConfigurationIndex == 0 {
		u.NumOutChannels = int(readEscapedValue(ur, 5, 8, 16))
	}
	if ur.AccError() != nil || ur.NrBitsRead() > nrBits {
		return u, fmt.Errorf("USAC config too short")
	}
	if u.SamplingFrequency == 0 {
		return u, fmt.Errorf("USAC config: reserved or zero sampling frequency, index %d", frequencyIndex)
	}
	return u, nil
}

// readEscapedValue reads an escapedValue() (ISO/IEC 23003-3:2022 Table 19).
func readEscapedValue(br *bits.Reader, nBits1, nBits2, nBits3 int) uint {
	value := br.Read(nBits1)
	if value == bits.Mask(nBits1) {
		valueAdd := br.Read(nBits2)
		value += valueAdd
		if valueAdd == bits.Mask(nBits2) {
			value += br.Read(nBits3)
		}
	}
	return value
}

func (u *USACConfig) encode(aw *ascWriter) {
	for i, b := range u.Data {
		n := min(8, u.NrBits-8*i)
		aw.write(uint(b)>>(8-n), n)
	}
}

func (u *USACConfig) validate() error {
	if u.NrBits <= 0 || u.NrBits > 8*len(u.Data) || u.NrBits <= 8*(len(u.Data)-1) {
		return fmt.Errorf("USAC config has %d bits in %d bytes of data", u.NrBits, len(u.Data))
	}
	return nil
}
