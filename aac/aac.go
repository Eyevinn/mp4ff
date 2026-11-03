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

// Sync words of the backward-compatible extensions after the GASpecificConfig.
const (
	syncExtensionSBR = 0x2b7
	syncExtensionPS  = 0x548
)

// AudioSpecificConfig according to ISO/IEC 14496-3:2019 Table 1.19,
// for AAC Main, AAC-LC and HE-AAC v1 and v2.
//
// HE-AAC is signalled either explicitly by ObjectType HEAACv1 or HEAACv2,
// or backward compatibly by a sync extension after the GASpecificConfig
// of an AAC-LC configuration (SyncExtensionPresent).
type AudioSpecificConfig struct {
	ObjectType             byte
	ChannelConfiguration   byte // Defined in Table 1.23. 0 means that PCE gives the layout
	SamplingFrequency      int
	ExtensionFrequency     int // SBR output sampling frequency
	SBRPresentFlag         bool
	PSPresentFlag          bool
	FrameLengthFlag        bool   // 960 instead of 1024 samples per frame
	DependsOnCoreCoder     bool   // Scalable configuration with a core coder
	CoreCoderDelay         uint16 // 14 bits, present if DependsOnCoreCoder
	ExtensionFlag          bool
	ExtensionFlag3         bool                  // Reserved for future GA syntax, which is not supported
	PCE                    *ProgramConfigElement // Present if ChannelConfiguration is 0
	SyncExtensionPresent   bool                  // SBR signalled by a 0x2b7 sync extension
	PSSyncExtensionPresent bool                  // PS signalled by a 0x548 sync extension
}

// CoreSamplesPerFrame returns the number of samples per frame at the core
// sampling frequency SamplingFrequency: 960 if FrameLengthFlag is set,
// otherwise 1024. SBR doubles the number of output samples per frame.
func (a *AudioSpecificConfig) CoreSamplesPerFrame() uint32 {
	if a.FrameLengthFlag {
		return 960
	}
	return 1024
}

// NrChannels returns the number of channels given by the channel
// configuration, or by the program config element for channel
// configuration 0. It returns 0 for the reserved channel configurations.
// For HE-AAC v2, it is the mono channel that parametric stereo builds on.
func (a *AudioSpecificConfig) NrChannels() int {
	if a.ChannelConfiguration == 0 {
		if a.PCE == nil {
			return 0
		}
		return a.PCE.NrChannels()
	}
	return NrChannels(a.ChannelConfiguration)
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
// object type in an AudioSpecificConfig (ISO/IEC 14496-3:2019 Table 1.19).
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
	hdr.ObjectType = readAudioObjectType(br)
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

// maxConfigSize bounds how much DecodeAudioSpecificConfig reads.
const maxConfigSize = 1 << 20

// DecodeAudioSpecificConfig decodes the AudioSpecificConfig of AAC Main,
// AAC-LC or HE-AAC v1 and v2, including the GASpecificConfig, the program
// config element of channel configuration 0, and the SBR and PS sync
// extensions.
//
// A sync extension is only looked for if enough bits are left, so the whole
// of r is read, and r must end where the configuration ends, as a reader
// over an esds DecConfig does. Bits after the last field of the syntax are
// ignored.
//
// On a decode error, the returned config holds the fields decoded so far.
func DecodeAudioSpecificConfig(r io.Reader) (*AudioSpecificConfig, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxConfigSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxConfigSize {
		return nil, fmt.Errorf("audio specific config exceeds %d bytes", maxConfigSize)
	}
	br := bits.NewReaderFromSlice(data)
	tooShort := func(part string) error {
		return fmt.Errorf("%s too short: %w", part, br.AccError())
	}
	asc := &AudioSpecificConfig{}
	asc.ObjectType = readAudioObjectType(br)
	if br.AccError() != nil {
		return asc, tooShort("audio specific config")
	}
	switch asc.ObjectType {
	case AACMain, AAClc:
	case HEAACv1:
		asc.SBRPresentFlag = true
	case HEAACv2:
		asc.SBRPresentFlag, asc.PSPresentFlag = true, true
	default:
		return asc, fmt.Errorf("unsupported object type: %d", asc.ObjectType)
	}
	if asc.SamplingFrequency, err = readFrequency(br); err != nil {
		return asc, err
	}
	asc.ChannelConfiguration = byte(br.Read(4))
	explicitHE := asc.ObjectType == HEAACv1 || asc.ObjectType == HEAACv2
	if explicitHE {
		if asc.ExtensionFrequency, err = readFrequency(br); err != nil {
			return asc, err
		}
		baseType := readAudioObjectType(br)
		if br.AccError() != nil {
			return asc, tooShort("audio specific config")
		}
		if baseType != AAClc {
			return asc, fmt.Errorf("base audioObjectType is %d instead of AAC-LC (2)", baseType)
		}
	}

	// GASpecificConfig, ISO/IEC 14496-3:2019 Table 4.1
	asc.FrameLengthFlag = br.ReadFlag()
	asc.DependsOnCoreCoder = br.ReadFlag()
	if asc.DependsOnCoreCoder {
		asc.CoreCoderDelay = uint16(br.Read(14))
	}
	asc.ExtensionFlag = br.ReadFlag()
	if br.AccError() != nil {
		return asc, tooShort("GA specific config")
	}
	if asc.ChannelConfiguration == 0 {
		if asc.PCE, err = decodeProgramConfigElement(br); err != nil {
			return asc, err
		}
	}
	if asc.ExtensionFlag {
		// Only error resilient object types have more fields before extensionFlag3.
		asc.ExtensionFlag3 = br.ReadFlag()
		if br.AccError() != nil {
			return asc, tooShort("GA specific config")
		}
		if asc.ExtensionFlag3 {
			return asc, fmt.Errorf("extensionFlag3 requires unsupported future GA syntax")
		}
	}

	// Sync extensions. A decoder only looks for one if enough bits remain,
	// and an unknown sync word or extension object type ends the parsing.
	bitsLeft := func() int { return len(data)*8 - br.NrBitsRead() }
	if !explicitHE && bitsLeft() >= 16 && br.Read(11) == syncExtensionSBR {
		if readAudioObjectType(br) == HEAACv1 {
			asc.SyncExtensionPresent = true
			asc.SBRPresentFlag = br.ReadFlag()
			if asc.SBRPresentFlag {
				if asc.ExtensionFrequency, err = readFrequency(br); err != nil {
					return asc, err
				}
			}
			if bitsLeft() >= 12 && br.Read(11) == syncExtensionPS {
				asc.PSSyncExtensionPresent = true
				asc.PSPresentFlag = br.ReadFlag()
			}
		}
		if br.AccError() != nil {
			return asc, tooShort("sync extension")
		}
	}
	return asc, nil
}

// Encode writes the AudioSpecificConfig of AAC Main, AAC-LC or HE-AAC,
// including the GASpecificConfig, the program config element and the sync
// extensions.
func (a *AudioSpecificConfig) Encode(w io.Writer) error {
	if err := a.validate(); err != nil {
		return err
	}
	aw := &ascWriter{bw: bits.NewWriter(w)}
	aw.write(uint(a.ObjectType), 5)
	aw.frequency(a.SamplingFrequency)
	aw.write(uint(a.ChannelConfiguration), 4)
	if a.ObjectType == HEAACv1 || a.ObjectType == HEAACv2 {
		aw.frequency(a.ExtensionFrequency)
		aw.write(AAClc, 5)
	}
	aw.flag(a.FrameLengthFlag)
	aw.flag(a.DependsOnCoreCoder)
	if a.DependsOnCoreCoder {
		aw.write(uint(a.CoreCoderDelay), 14)
	}
	aw.flag(a.ExtensionFlag)
	if a.ChannelConfiguration == 0 {
		a.PCE.encode(aw)
	}
	if a.ExtensionFlag {
		aw.flag(a.ExtensionFlag3)
	}
	if a.SyncExtensionPresent {
		aw.write(syncExtensionSBR, 11)
		aw.write(HEAACv1, 5)
		aw.flag(a.SBRPresentFlag)
		if a.SBRPresentFlag {
			aw.frequency(a.ExtensionFrequency)
		}
		if a.PSSyncExtensionPresent {
			aw.write(syncExtensionPS, 11)
			aw.flag(a.PSPresentFlag)
		}
	}
	aw.bw.Flush()
	return aw.bw.AccError()
}

// validate checks that the fields can be encoded and are read back as they are.
func (a *AudioSpecificConfig) validate() error {
	switch a.ObjectType {
	case AACMain, AAClc, HEAACv1, HEAACv2:
	default:
		return fmt.Errorf("audioObjectType %d not supported", a.ObjectType)
	}
	if a.ChannelConfiguration > 15 {
		return fmt.Errorf("channel configuration %d does not fit in four bits", a.ChannelConfiguration)
	}
	if a.ChannelConfiguration == 0 {
		if a.PCE == nil {
			return fmt.Errorf("channel configuration 0 needs a program config element")
		}
		if err := a.PCE.validate(); err != nil {
			return err
		}
	} else if a.PCE != nil {
		return fmt.Errorf("program config element needs channel configuration 0, not %d", a.ChannelConfiguration)
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
	if a.PSPresentFlag && a.ObjectType != HEAACv2 && !a.PSSyncExtensionPresent {
		return fmt.Errorf("PS presence requires HE-AACv2 or a PS sync extension")
	}
	if a.CoreCoderDelay > 0x3fff || (!a.DependsOnCoreCoder && a.CoreCoderDelay != 0) {
		return fmt.Errorf("core coder delay requires dependsOnCoreCoder and a 14-bit value")
	}
	if a.ExtensionFlag3 {
		return fmt.Errorf("extensionFlag3 requires unsupported future GA syntax")
	}
	return nil
}

// readAudioObjectType reads an audio object type with its escape
// (ISO/IEC 14496-3:2019 Table 1.20).
func readAudioObjectType(br *bits.Reader) byte {
	objectType := br.Read(5)
	if objectType == 31 {
		objectType = 32 + br.Read(6)
	}
	return byte(objectType)
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

// ascWriter counts the bits it writes, since the byte alignment in a
// program config element is relative to the start of the AudioSpecificConfig.
type ascWriter struct {
	bw     *bits.Writer
	nrBits int
}

func (aw *ascWriter) write(value uint, n int) {
	aw.bw.Write(value, n)
	aw.nrBits += n
}

func (aw *ascWriter) flag(f bool) {
	if f {
		aw.write(1, 1)
	} else {
		aw.write(0, 1)
	}
}

// frequency writes a 4-bit frequency index, or the escape index 15 and a
// 24-bit frequency.
func (aw *ascWriter) frequency(frequency int) {
	if index, ok := ReverseFrequencies[frequency]; ok {
		aw.write(uint(index), 4)
	} else {
		aw.write(0x0f, 4)
		aw.write(uint(frequency), 24)
	}
}

func (aw *ascWriter) byteAlign() {
	if rest := aw.nrBits % 8; rest != 0 {
		aw.write(0, 8-rest)
	}
}
