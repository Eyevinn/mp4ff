package aac

import (
	"fmt"
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

const (
	// AACMain - AAC Main
	AACMain = 1
	// AAClc - AAC-LC Low Complexity
	AAClc = 2
	// HEAACv1 - HE-AAC version 1 with SBR
	HEAACv1 = 5
	// HEAACv2 - HE-AAC version 2 with SBR and PS
	HEAACv2 = 29
)

// AudioSpecificConfig according to ISO/IEC 14496-3
// Syntax specified in Table 1.15
type AudioSpecificConfig struct {
	ObjectType             byte
	ChannelConfiguration   byte // Defined in Table 1.19
	SamplingFrequency      int
	ExtensionFrequency     int
	SBRPresentFlag         bool
	PSPresentFlag          bool
	FrameLengthFlag        bool // 960 core samples per frame instead of 1024.
	DependsOnCoreCoder     bool
	CoreCoderDelay         uint16 // 14 bits; present only with DependsOnCoreCoder.
	ExtensionFlag          bool
	ExtensionFlag3         bool // Future GA extensions are unsupported.
	SyncExtensionPresent   bool // Backward-compatible SBR signaling after GA config.
	PSSyncExtensionPresent bool // Explicit PS presence bit in a sync extension.
}

// CoreSamplesPerFrame returns the duration in core sampling-frequency units.
// SBR output samples use the extension frequency instead.
func (a *AudioSpecificConfig) CoreSamplesPerFrame() uint32 {
	if a.FrameLengthFlag {
		return 960
	}
	return 1024
}

// FrequencyTable maps frequency index to sample rate in Hz
var FrequencyTable = map[byte]int{
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

// ReverseFrequencies converts sample frequency to index
var ReverseFrequencies = map[int]byte{
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

// NrChannels returns the number of channels of a channelConfiguration value
// according to ISO/IEC 14496-3:2019 Table 1.23, or 0 for values without a
// fixed layout: 0 (layout given by a program_config_element) and the
// reserved values 8-10 and 15.
//
//	1: 1 channel (mono)
//	2: 2 channels (stereo)
//	3: 3 channels
//	4: 4 channels
//	5: 5 channels
//	6: 6 channels (5.1)
//	7: 8 channels (7.1 front)
//	11: 7 channels (6.1)
//	12: 8 channels (7.1 back)
//	13: 24 channels (22.2)
//	14: 8 channels (7.1 top)
func NrChannels(channelConfiguration byte) int {
	switch channelConfiguration {
	case 1, 2, 3, 4, 5, 6:
		return int(channelConfiguration)
	case 7, 12, 14:
		return 8
	case 11:
		return 7
	case 13:
		return 24
	default:
		return 0
	}
}

// AudioSpecificConfigHeader - the leading fields shared by every audio
// object type in an AudioSpecificConfig (ISO/IEC 14496-3 Table 1.15).
type AudioSpecificConfigHeader struct {
	ObjectType           byte
	ChannelConfiguration byte
	SamplingFrequency    int
}

// DecodeAudioSpecificConfigHeader decodes the leading AudioSpecificConfig
// fields and stops before the object-type-specific payload, so it works for
// object types that DecodeAudioSpecificConfig does not support. The sampling
// frequency is the base frequency, without any HE-AAC extension.
func DecodeAudioSpecificConfigHeader(r io.Reader) (*AudioSpecificConfigHeader, error) {
	br := bits.NewReader(r)
	hdr := &AudioSpecificConfigHeader{}
	objectType := br.Read(5)
	if objectType == 31 {
		objectType = 32 + br.Read(6)
	}
	hdr.ObjectType = byte(objectType)
	frequency, ok := getFrequency(br)
	if !ok {
		if br.AccError() != nil {
			return nil, fmt.Errorf("audio specific config too short: %w", br.AccError())
		}
		return nil, fmt.Errorf("strange frequency index")
	}
	hdr.SamplingFrequency = frequency
	hdr.ChannelConfiguration = byte(br.Read(4))
	if br.AccError() != nil {
		return nil, fmt.Errorf("audio specific config too short: %w", br.AccError())
	}
	return hdr, nil
}

// DecodeAudioSpecificConfig reads one complete AudioSpecificConfig, including
// backward-compatible SBR/PS sync extensions. r must end at the end of the
// configuration (for example, a reader over an esds DecConfig slice).
// Channel configuration zero requires a Program Config Element and is unsupported.
func DecodeAudioSpecificConfig(r io.Reader) (*AudioSpecificConfig, error) {
	const maxConfigSize = 1 << 20
	data, err := io.ReadAll(io.LimitReader(r, maxConfigSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxConfigSize {
		return nil, fmt.Errorf("audio specific config exceeds %d bytes", maxConfigSize)
	}
	br := bits.NewReaderFromSlice(data)
	asc := &AudioSpecificConfig{ObjectType: byte(br.Read(5))}
	if br.AccError() != nil {
		return nil, fmt.Errorf("audio specific config too short: %w", br.AccError())
	}
	switch asc.ObjectType {
	case AACMain, AAClc:
	case HEAACv1:
		asc.SBRPresentFlag = true
	case HEAACv2:
		asc.SBRPresentFlag, asc.PSPresentFlag = true, true
	default:
		return nil, fmt.Errorf("unsupported object type: %d", asc.ObjectType)
	}
	frequency, err := readFrequency(br)
	if err != nil {
		return nil, err
	}
	asc.SamplingFrequency = frequency
	asc.ChannelConfiguration = byte(br.Read(4))
	if br.AccError() != nil {
		return nil, fmt.Errorf("audio specific config too short: %w", br.AccError())
	}
	if asc.ChannelConfiguration == 0 {
		return nil, fmt.Errorf("channel configuration zero requires an unsupported Program Config Element")
	}
	if asc.ObjectType == HEAACv1 || asc.ObjectType == HEAACv2 {
		asc.ExtensionFrequency, err = readFrequency(br)
		if err != nil {
			return nil, err
		}
		baseType := br.Read(5)
		if br.AccError() != nil {
			return nil, fmt.Errorf("audio specific config too short: %w", br.AccError())
		}
		if baseType != AAClc {
			return nil, fmt.Errorf("base audioObjectType is %d instead of AAC-LC (2)", baseType)
		}
	}
	asc.FrameLengthFlag = br.ReadFlag()
	asc.DependsOnCoreCoder = br.ReadFlag()
	if asc.DependsOnCoreCoder {
		asc.CoreCoderDelay = uint16(br.Read(14))
	}
	asc.ExtensionFlag = br.ReadFlag()
	if asc.ExtensionFlag {
		// Main and LC add only extensionFlag3 here; ER and scalable types
		// have different syntax and remain outside this parser's profile set.
		asc.ExtensionFlag3 = br.ReadFlag()
	}
	if br.AccError() != nil {
		return nil, fmt.Errorf("GA specific config too short: %w", br.AccError())
	}
	if asc.ExtensionFlag3 {
		return nil, fmt.Errorf("extensionFlag3 requires unsupported future GA syntax")
	}
	remaining := func() int { return len(data)*8 - br.NrBitsRead() }
	if remaining() > 7 {
		if asc.ObjectType == HEAACv1 || asc.ObjectType == HEAACv2 || br.Read(11) != 0x2b7 {
			return nil, fmt.Errorf("unsupported audio specific config extension")
		}
		asc.SyncExtensionPresent = true
		if br.Read(5) != HEAACv1 {
			return nil, fmt.Errorf("unsupported sync extension object type")
		}
		asc.SBRPresentFlag = br.ReadFlag()
		if asc.SBRPresentFlag {
			asc.ExtensionFrequency, err = readFrequency(br)
			if err != nil {
				return nil, err
			}
		}
		if remaining() > 7 {
			if br.Read(11) != 0x548 {
				return nil, fmt.Errorf("unsupported PS sync extension")
			}
			asc.PSSyncExtensionPresent = true
			asc.PSPresentFlag = br.ReadFlag()
		}
	}
	if br.AccError() != nil {
		return nil, fmt.Errorf("sync extension too short: %w", br.AccError())
	}
	if remaining() > 7 || br.Read(remaining()) != 0 {
		return nil, fmt.Errorf("invalid audio specific config padding")
	}
	if err := asc.validate(); err != nil {
		return nil, err
	}
	return asc, nil
}

// Encode writes Main, LC or HE-AAC configuration, including GA and sync extensions.
func (a *AudioSpecificConfig) Encode(w io.Writer) error {
	if err := a.validate(); err != nil {
		return err
	}
	bw := bits.NewWriter(w)
	bw.Write(uint(a.ObjectType), 5)
	writeFrequency(bw, a.SamplingFrequency)
	bw.Write(uint(a.ChannelConfiguration), 4)
	if a.ObjectType == HEAACv1 || a.ObjectType == HEAACv2 {
		writeFrequency(bw, a.ExtensionFrequency)
		bw.Write(AAClc, 5)
	}
	writeFlag(bw, a.FrameLengthFlag)
	writeFlag(bw, a.DependsOnCoreCoder)
	if a.DependsOnCoreCoder {
		bw.Write(uint(a.CoreCoderDelay), 14)
	}
	writeFlag(bw, a.ExtensionFlag)
	if a.ExtensionFlag {
		writeFlag(bw, a.ExtensionFlag3)
	}
	if a.SyncExtensionPresent {
		bw.Write(0x2b7, 11)
		bw.Write(HEAACv1, 5)
		writeFlag(bw, a.SBRPresentFlag)
		if a.SBRPresentFlag {
			writeFrequency(bw, a.ExtensionFrequency)
		}
		if a.PSSyncExtensionPresent {
			bw.Write(0x548, 11)
			writeFlag(bw, a.PSPresentFlag)
		}
	}
	bw.Flush()
	return bw.AccError()
}

func (a *AudioSpecificConfig) validate() error {
	switch a.ObjectType {
	case AACMain, AAClc, HEAACv1, HEAACv2:
	default:
		return fmt.Errorf("audioObjectType %d not supported", a.ObjectType)
	}
	if a.ChannelConfiguration == 0 {
		return fmt.Errorf("channel configuration zero requires an unsupported Program Config Element")
	}
	if a.ChannelConfiguration > 15 {
		return fmt.Errorf("channel configuration %d does not fit in four bits", a.ChannelConfiguration)
	}
	if a.SamplingFrequency <= 0 || a.SamplingFrequency > 0xffffff {
		return fmt.Errorf("invalid sampling frequency %d", a.SamplingFrequency)
	}
	explicitHE := a.ObjectType == HEAACv1 || a.ObjectType == HEAACv2
	if explicitHE && a.SyncExtensionPresent {
		return fmt.Errorf("explicit HE-AAC cannot also carry an SBR sync extension")
	}
	if explicitHE || a.SBRPresentFlag {
		if a.ExtensionFrequency <= 0 || a.ExtensionFrequency > 0xffffff {
			return fmt.Errorf("invalid extension frequency %d", a.ExtensionFrequency)
		}
	} else if a.ExtensionFrequency != 0 {
		return fmt.Errorf("extension frequency requires SBR signaling")
	}
	if !explicitHE && a.SBRPresentFlag && !a.SyncExtensionPresent {
		return fmt.Errorf("SBR presence requires a sync extension")
	}
	if a.PSSyncExtensionPresent && !a.SyncExtensionPresent {
		return fmt.Errorf("PS sync extension requires an SBR sync extension")
	}
	if a.PSPresentFlag && a.ObjectType != HEAACv2 &&
		(!a.PSSyncExtensionPresent || !a.SBRPresentFlag) {
		return fmt.Errorf("PS presence requires SBR and explicit PS signaling")
	}
	if a.CoreCoderDelay > 0x3fff || (!a.DependsOnCoreCoder && a.CoreCoderDelay != 0) {
		return fmt.Errorf("core coder delay requires dependsOnCoreCoder and a 14-bit value")
	}
	if a.ExtensionFlag3 {
		return fmt.Errorf("extensionFlag3 requires unsupported future GA syntax")
	}
	return nil
}

func readFrequency(br *bits.Reader) (int, error) {
	frequency, ok := getFrequency(br)
	if br.AccError() != nil {
		return 0, fmt.Errorf("audio specific config too short: %w", br.AccError())
	}
	if !ok {
		return 0, fmt.Errorf("strange frequency index")
	}
	if frequency == 0 {
		return 0, fmt.Errorf("invalid sampling frequency zero")
	}
	return frequency, nil
}

func writeFrequency(bw *bits.Writer, frequency int) {
	if index, ok := ReverseFrequencies[frequency]; ok {
		bw.Write(uint(index), 4)
	} else {
		bw.Write(0x0f, 4)
		bw.Write(uint(frequency), 24)
	}
}

func writeFlag(bw *bits.Writer, flag bool) {
	if flag {
		bw.Write(1, 1)
	} else {
		bw.Write(0, 1)
	}
}

// getFrequency - either from 4-bit index or 24-bit value
func getFrequency(br *bits.Reader) (frequency int, ok bool) {
	frequencyIndex := br.Read(4)
	if frequencyIndex == 0x0f {
		f := br.Read(24)
		if br.AccError() != nil {
			return 0, false
		}
		return int(f), true
	}
	if br.AccError() != nil {
		return 0, false
	}
	frequency, ok = FrequencyTable[byte(frequencyIndex)]
	return frequency, ok
}
