package iamf

import (
	"fmt"
)

/**
 * @file
 * Immersive Audio Model and Formats API header
 * @see <a href="https://aomediacodec.github.io/iamf/">Immersive Audio Model and Formats</a>
 */

/**
 * Types were extracted from the ffmpeg implementation.
 * Based on AVUTIL_IAMF_H
 */

// Rational represents a rational number
type Rational struct {
	Num int32
	Den int32
}

func MakeRational(num int32, den int32) Rational {
	return Rational{Num: num, Den: den}
}

func (r Rational) Float64() float64 {
	if r.Den == 0 {
		return 0
	}
	return float64(r.Num) / float64(r.Den)
}

func (r Rational) String() string {
	return fmt.Sprintf("%d/%d", r.Num, r.Den)
}

// AnimationType defines the animation type for parameters
type AnimationType uint8

const (
	AnimationTypeStep AnimationType = iota
	AnimationTypeLinear
	AnimationTypeBezier
)

func (a AnimationType) String() string {
	switch a {
	case AnimationTypeStep:
		return "Step"
	case AnimationTypeLinear:
		return "Linear"
	case AnimationTypeBezier:
		return "Bezier"
	default:
		return fmt.Sprintf("Unknown(%d)", a)
	}
}

// MixGain represents Mix Gain Parameter Data as defined in section 3.8.1 of IAMF
type MixGain struct {
	// Duration for the given subblock, in units of 1 / parameter_rate
	SubblockDuration uint32
	// The type of animation applied to the parameter values
	AnimationType AnimationType
	// Parameter value that is applied at the start of the subblock
	// Valid range: -128.0 to 128.0
	StartPointValue Rational
	// Parameter value that is applied at the end of the subblock
	// Applies only to Linear and Bezier animation types
	// Valid range: -128.0 to 128.0
	EndPointValue Rational
	// Parameter value of the middle control point of a quadratic Bezier curve (y-axis)
	// Applies only to Bezier animation type
	// Valid range: -128.0 to 128.0
	ControlPointValue Rational
	// Parameter value of the time of the middle control point (x-axis)
	// Applies only to Bezier animation type
	// Valid range: 0.0 to 1.0
	ControlPointRelativeTime Rational
}

// DemixingInfo represents Demixing Info Parameter Data as defined in section 3.8.2 of IAMF
type DemixingInfo struct {
	// Duration for the given subblock, in units of 1 / parameter_rate
	SubblockDuration uint32
	// Pre-defined combination of demixing parameters
	DmixpMode uint32
}

// ReconGain represents Recon Gain Info Parameter Data as defined in section 3.8.3 of IAMF
type ReconGain struct {
	// Duration for the given subblock, in units of 1 / parameter_rate
	SubblockDuration uint32
	// Array of gain values to be applied to each channel for each layer
	// Channel order: FL, C, FR, SL, SR, TFL, TFR, BL, BR, TBL, TBR, LFE
	// [6 layers][12 channels]
	ReconGain [6][12]uint8
}

// ParamDefinitionType identifies the type of parameter definition
type ParamDefinitionType uint8

const (
	// ParamDefinitionMixGain - subblocks are of type MixGain
	ParamDefinitionMixGain ParamDefinitionType = iota
	// ParamDefinitionDemixing - subblocks are of type DemixingInfo
	ParamDefinitionDemixing
	// ParamDefinitionReconGain - subblocks are of type ReconGain
	ParamDefinitionReconGain
)

func (p ParamDefinitionType) String() string {
	switch p {
	case ParamDefinitionMixGain:
		return "MixGain"
	case ParamDefinitionDemixing:
		return "Demixing"
	case ParamDefinitionReconGain:
		return "ReconGain"
	default:
		return fmt.Sprintf("Unknown(%d)", p)
	}
}

// ParamDefinition represents Parameters as defined in section 3.6.1 of IAMF
type ParamDefinition struct {
	// Parameters type - determines the type of subblock elements
	Type ParamDefinitionType
	// Identifier for the parameter substream
	ParameterID uint32
	// Parameter rate (samples per second)
	ParameterRate uint32
	// Duration of the parameter block
	Duration uint32
	// Constant subblock duration (0 if variable)
	ConstantSubblockDuration uint32
	// Number of subblocks
	NumSubblocks uint32
	// Subblocks - type depends on Type field
	// Can be []MixGain, []DemixingInfo, or []ReconGain
	Subblocks interface{}
}

// AmbisonicsMode defines the ambisonics mode
type AmbisonicsMode uint8

const (
	AmbisonicsModeMono AmbisonicsMode = iota
	AmbisonicsModeProjection
)

func (a AmbisonicsMode) String() string {
	switch a {
	case AmbisonicsModeMono:
		return "Mono"
	case AmbisonicsModeProjection:
		return "Projection"
	default:
		return fmt.Sprintf("Unknown(%d)", a)
	}
}

// LayerFlag defines the flags for the layer
type LayerFlag uint8

const (
	LayerFlagReconGain LayerFlag = 1 << iota
)

func (a LayerFlag) String() string {
	var parts []string
	if a&LayerFlagReconGain != 0 {
		parts = append(parts, "ReconGain")
		parts = append(parts, " ")
	}
	if len(parts) == 0 {
		return fmt.Sprintf("Unknown(%d)", a)
	} else if len(parts) != 1 {
		parts = parts[:len(parts)-1]
	}
	return fmt.Sprintf("%s", parts)
}

// Layer represents an audio layer within an audio element
type Layer struct {
	// Channel layout for this layer
	ChannelLayout ChannelLayout
	// Layer is a bitmask of LayerFlags* flags.
	Flags LayerFlag
	// Output gain flags (for channel-based audio)
	OutputGainFlags uint8
	// Output gain (for channel-based audio)
	OutputGain Rational
	// Ambisonics mode (for scene-based audio)
	AmbisonicsMode AmbisonicsMode
	// Demixing matrix (for projection ambisonics)
	DemixingMatrix []Rational
	// Number of demixing matrices
	NumDemixingMatrix uint32
}

// AudioElementType identifies the type of audio element
type AudioElementType uint8

const (
	AudioElementTypeChannel AudioElementType = iota
	AudioElementTypeScene
)

func (a AudioElementType) String() string {
	switch a {
	case AudioElementTypeChannel:
		return "Channel"
	case AudioElementTypeScene:
		return "Scene"
	default:
		return fmt.Sprintf("Unknown(%d)", a)
	}
}

// AudioElement represents an Audio Element as defined in section 3.6 of IAMF
type AudioElement struct {
	// Audio element identifier
	AudioElementID uint32
	// Type of audio element
	AudioElementType AudioElementType
	// Number of layers
	NumLayers uint32
	// Array of layers
	Layers []*Layer
	// Demixing info parameter definition
	DemixingInfo *ParamDefinition
	// Recon gain info parameter definition
	ReconGainInfo *ParamDefinition
	// Default weight for this element
	DefaultW uint32
}

// HeadphonesMode identifies the headphone rendering mode
type HeadphonesMode uint8

const (
	HeadphonesModeStereo HeadphonesMode = iota
	HeadphonesModeBinaural
)

func (a HeadphonesMode) String() string {
	switch a {
	case HeadphonesModeStereo:
		return "Stereo"
	case HeadphonesModeBinaural:
		return "Binaural"
	default:
		return fmt.Sprintf("Unknown(%d)", a)
	}
}

// SubmixElement represents an element within a submix
type SubmixElement struct {
	// Audio element ID this submix element refers to
	AudioElementID uint32
	// Element mix configuration
	ElementMixConfig *ParamDefinition
	// Default mix gain for this element
	DefaultMixGain Rational
	// Headphones rendering mode
	HeadphonesRenderingMode HeadphonesMode
	// Annotations (language -> text mapping)
	Annotations map[string]string
}

// AudioElementType identifies the type of submix layout
type SubMixLayoutType uint8

const (
	SubMixLayoutTypeLoudspeakers SubMixLayoutType = 2
	SubMixLayoutTypeBinaural     SubMixLayoutType = 3
)

func (a SubMixLayoutType) String() string {
	switch a {
	case SubMixLayoutTypeLoudspeakers:
		return "Loudspeakers"
	case SubMixLayoutTypeBinaural:
		return "Binaural"
	default:
		return fmt.Sprintf("Unknown(%d)", a)
	}
}

// SubmixLayout represents a layout within a submix
type SubmixLayout struct {
	// Type of a submix
	LayoutType SubMixLayoutType
	// Sound system (channel layout)
	SoundSystem ChannelLayout
	// Integrated loudness
	IntegratedLoudness Rational
	// Digital peak
	DigitalPeak Rational
	// True peak
	TruePeak Rational
	// Dialogue anchored loudness
	DialogueAnchoredLoudness Rational
	// Album anchored loudness
	AlbumAnchoredLoudness Rational
}

// Submix represents a submix within a mix presentation
type Submix struct {
	// Number of elements in this submix
	NumElements uint32
	// Elements in this submix
	Elements []*SubmixElement
	// Number of layouts
	NumLayouts uint32
	// Layouts for this submix
	Layouts []*SubmixLayout
	// Default mix gain
	DefaultMixGain Rational
	// Output mix configuration
	OutputMixConfig *ParamDefinition
}

// MixPresentation represents a Mix Presentation as defined in section 3.7 of IAMF
type MixPresentation struct {
	// Mix presentation identifier
	MixPresentationID uint32
	// Number of submixes
	NumSubmixes uint32
	// Array of submixes
	Submixes []*Submix
	// Annotations (language -> text mapping)
	Annotations map[string]string
}

// MaxOBUHeaderSize is the maximum size of an IAMF OBU header
const MaxIAMFOBUHeaderSize = 1 + 8*3

// ObuType represents OBU types (section 3.2)
type ObuType int

const (
	ObuTypeCodecConfig       ObuType = 0
	ObuTypeAudioElement      ObuType = 1
	ObuTypeMixPresentation   ObuType = 2
	ObuTypeParameterBlock    ObuType = 3
	ObuTypeTemporalDelimiter ObuType = 4
	ObuTypeAudioFrame        ObuType = 5
	ObuTypeAudioFrameID0     ObuType = 6
	ObuTypeAudioFrameID1     ObuType = 7
	ObuTypeAudioFrameID2     ObuType = 8
	ObuTypeAudioFrameID3     ObuType = 9
	ObuTypeAudioFrameID4     ObuType = 10
	ObuTypeAudioFrameID5     ObuType = 11
	ObuTypeAudioFrameID6     ObuType = 12
	ObuTypeAudioFrameID7     ObuType = 13
	ObuTypeAudioFrameID8     ObuType = 14
	ObuTypeAudioFrameID9     ObuType = 15
	ObuTypeAudioFrameID10    ObuType = 16
	ObuTypeAudioFrameID11    ObuType = 17
	ObuTypeAudioFrameID12    ObuType = 18
	ObuTypeAudioFrameID13    ObuType = 19
	ObuTypeAudioFrameID14    ObuType = 20
	ObuTypeAudioFrameID15    ObuType = 21
	ObuTypeAudioFrameID16    ObuType = 22
	ObuTypeAudioFrameID17    ObuType = 23
	ObuTypeSequenceHeader    ObuType = 31
)

// OBU type name mapping
var obuTypeNames = map[ObuType]string{
	ObuTypeCodecConfig:       "Codec Config",
	ObuTypeAudioElement:      "Audio Element",
	ObuTypeMixPresentation:   "Mix Presentation",
	ObuTypeParameterBlock:    "Parameter Block",
	ObuTypeTemporalDelimiter: "Temporal Delimiter",
	ObuTypeAudioFrame:        "Audio Frame",
	ObuTypeSequenceHeader:    "IA Sequence Header",
}

func (o ObuType) String() string {
	if name, ok := obuTypeNames[o]; ok {
		return name
	}
	if o >= ObuTypeAudioFrameID0 && o <= ObuTypeAudioFrameID17 {
		return fmt.Sprintf("Audio Frame ID%d", o-ObuTypeAudioFrameID0)
	}
	if o >= 24 && o <= 30 {
		return fmt.Sprintf("Reserved OBU Type (%d)", o)
	}
	return fmt.Sprintf("Unknown OBU Type (%d)", o)
}

/**
 * Types were extracted from the ffmpeg implementation.
 * Based on AVFORMAT_IAMF_H
 */

// CodecConfig represents IAMF codec configuration
type IamfCodecConfig struct {
	CodecConfigID     uint32
	CodecID           string
	CodecTag          uint32
	NumSamples        uint32
	AudioRollDistance int32
	SampleRate        int32
	ExtradataSize     int32
	Extradata         []byte
}

// Layer represents IAMF layer configuration
type IamfLayer struct {
	SubstreamCount        uint32
	CoupledSubstreamCount uint32
}

// SubStream represents IAMF audio substream
type IamfSubStream struct {
	AudioSubstreamID uint32
	CodecParameters  CodecParameters
}

// AudioElement represents IAMF audio element
type IamfAudioElement struct {
	Element        AudioElement
	AudioElementID uint32
	Substreams     []*IamfSubStream
	NumSubstreams  uint32
	CodecConfigID  uint32
	Layers         []*IamfLayer
	NumLayers      uint32
}

// MixPresentation represents IAMF mix presentation
type IamfMixPresentation struct {
	Mix               MixPresentation
	MixPresentationID uint32
	CountLabel        uint32
	LanguageLabel     []string
}

// ParamDefinition represents IAMF parameter definition
type IamfParamDefinition struct {
	AudioElement *IamfAudioElement
	Param        ParamDefinition
	Mode         int32
	ParamSize    uint64
}

// Context represents the IAMF context
type IamfContext struct {
	CodecConfigs        []*IamfCodecConfig
	NumCodecConfigs     int32
	AudioElements       []*IamfAudioElement
	NumAudioElements    int32
	MixPresentations    []*IamfMixPresentation
	NumMixPresentations int32
	ParamDefinitions    []*IamfParamDefinition
	NumParamDefinitions int32
}

// AnchorElement represents IAMF anchor element
type AnchorElement int

const (
	AnchorElementUnknown AnchorElement = iota
	AnchorElementDialogue
	AnchorElementAlbum
)

func (a AnchorElement) String() string {
	switch a {
	case AnchorElementUnknown:
		return "Unknown"
	case AnchorElementDialogue:
		return "Dialogue"
	case AnchorElementAlbum:
		return "Album"
	default:
		return fmt.Sprintf("Unknown(%d)", a)
	}
}

// SoundSystem represents IAMF sound system configuration
type SoundSystem int

const (
	SoundSystemA_0_2_0  SoundSystem = iota // Loudspeaker configuration for Sound System A
	SoundSystemB_0_5_0                     // Loudspeaker configuration for Sound System B
	SoundSystemC_2_5_0                     // Loudspeaker configuration for Sound System C
	SoundSystemD_4_5_0                     // Loudspeaker configuration for Sound System D
	SoundSystemE_4_5_1                     // Loudspeaker configuration for Sound System E
	SoundSystemF_3_7_0                     // Loudspeaker configuration for Sound System F
	SoundSystemG_4_9_0                     // Loudspeaker configuration for Sound System G
	SoundSystemH_9_10_3                    // Loudspeaker configuration for Sound System H
	SoundSystemI_0_7_0                     // Loudspeaker configuration for Sound System I
	SoundSystemJ_4_7_0                     // Loudspeaker configuration for Sound System J
	SoundSystem10_2_7_0                    // Loudspeaker configuration for Sound System I + Ltf + Rtf
	SoundSystem11_2_3_0                    // Front subset of Loudspeaker configuration for Sound System J
	SoundSystem12_0_1_0                    // Mono
	SoundSystem13_9_1_6                    // Subset of Loudspeaker configuration for Sound System H
)

func (a SoundSystem) String() string {
	switch a {
	case SoundSystemA_0_2_0:
		return "Sound System A (0.2.0)"
	case SoundSystemB_0_5_0:
		return "Sound System B (0.5.0)"
	case SoundSystemC_2_5_0:
		return "Sound System C (2.5.0)"
	case SoundSystemD_4_5_0:
		return "Sound System D (4.5.0)"
	case SoundSystemE_4_5_1:
		return "Sound System E (4.5.1)"
	case SoundSystemF_3_7_0:
		return "Sound System F (3.7.0)"
	case SoundSystemG_4_9_0:
		return "Sound System G (4.9.0)"
	case SoundSystemH_9_10_3:
		return "Sound System H (9.10.3)"
	case SoundSystemI_0_7_0:
		return "Sound System I (0.7.0)"
	case SoundSystemJ_4_7_0:
		return "Sound System J (4.7.0)"
	case SoundSystem10_2_7_0:
		return "Sound System I + Ltf + Rtf (10.2.7.0)"
	case SoundSystem11_2_3_0:
		return "Sound System J Front Subset (11.2.3.0)"
	case SoundSystem12_0_1_0:
		return "Mono (12.0.1.0)"
	case SoundSystem13_9_1_6:
		return "Sound System H Subset (13.9.1.6)"
	default:
		return fmt.Sprintf("Unknown(%d)", a)
	}
}

type SoundSystemMap struct {
	Id     SoundSystem
	Layout ChannelLayout
}

/**
 * Custom types
 */

type ObuInfo struct {
	Size  uint
	Start int
	Type  ObuType
}

func (o ObuInfo) PayloadSize() int {
	return int(o.Size) - o.Start
}

// ChannelLayout represents a channel layout/configuration
type ChannelLayout struct {
	// Number of channels
	NumChannels uint32
	// Layout description (e.g., "stereo", "5.1", "7.1.4")
	Description string
	// Bitmask of channels
	ChannelMask uint64
	// Map of channels
	ChannelMap map[string]string
}

func (c ChannelLayout) String() string {
	if c.Description != "" {
		return c.Description
	}
	return fmt.Sprintf("%d channels (mask: 0x%X)", c.NumChannels, c.ChannelMask)
}

type CodecParameters struct {
	Ptr *IamfCodecConfig
}
