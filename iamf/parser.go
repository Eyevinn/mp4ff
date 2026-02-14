package iamf

import (
	"bytes"
	"errors"
	"fmt"
	"math"

	"github.com/Eyevinn/mp4ff/aac"
	"github.com/Eyevinn/mp4ff/bits"
)

/**
 * Types were extracted from the ffmpeg implementation.
 * Based on IAMF_C
 */

type channelMask uint64

// define masks for channel layouts
const (
	chmFrontLeft channelMask = 1 << iota
	chmFrontRight
	chmFrontCenter
	chmLowFrequency
	chmBackLeft
	chmBackRight
	chmFrontLeftOfCenter
	chmFrontRightOfCenter
	chmBackCenter
	chmSideLeft
	chmSideRight
	chmTopCenter
	chmTopFrontLeft
	chmTopFrontCenter
	chmTopFrontRight
	chmTopBackLeft
	chmTopBackCenter
	chmTopBackRight
	chmLowFrequency2
	chmTopSideLeft
	chmTopSideRight
	chmBottomFrontCenter
	chmBottomFrontLeft
	chmBottomFrontRight
	chmSideSurroundLeft
	chmSideSurroundRight
	chmBinauralLeft
	chmBinauralRight
	chmBottomBackLeft
	chmBottomBackRight
)

type channelOrder uint8

// define constants for audio channel order
const (
	coScalable channelOrder = iota
	coExpanded
	coAmbisonics
	coCustom
)

// channelLayout defines a struct for scalable audio channel layouts
type channelLayout struct {
	TableIndex int
	Channels   int
	Order      channelOrder
	Mask       channelMask
	Map        *map[int]int
}

func scalable(index int, channels int, mask channelMask) channelLayout {
	return channelLayout{
		TableIndex: index,
		Channels:   channels,
		Order:      coScalable,
		Mask:       mask,
		Map:        nil,
	}
}

// scalableChannelLayouts defines an array of channel layouts
// Based on IAMF loudspeaker_layout specification
// https://aomediacodec.github.io/iamf/#loudspeaker_layout
var scalableChannelLayouts = []channelLayout{
	// 0: Mono (C)
	// the mono channel
	scalable(0, 1,
		chmFrontCenter),

	// 1: Stereo (L/R)
	// the config of (0+2+0) of [ITU-2051-3] (Sound System A)
	scalable(1, 2,
		chmFrontLeft|chmFrontRight),

	// 2: 5.1ch (L/C/R/Ls/Rs/LFE)
	// the config of (0+5+0) of [ITU-2051-3] (Sound System B)
	scalable(2, 6, 0|
		chmFrontLeft|chmFrontCenter|chmFrontRight|
		chmSideLeft|chmSideRight|chmLowFrequency),

	// 3: 5.1.2ch (L/C/R/Ls/Rs/Ltf/Rtf/LFE)
	// the config of (2+5+0) of [ITU-2051-3] (Sound System C)
	scalable(3, 8, 0|
		chmFrontLeft|chmFrontCenter|chmFrontRight|
		chmSideLeft|chmSideRight|
		chmTopFrontLeft|chmTopFrontRight|
		chmLowFrequency),

	// 4: 5.1.4ch (L/C/R/Ls/Rs/Ltf/Rtf/Ltr/Rtr/LFE)
	// the config of (4+5+0) of [ITU-2051-3] (Sound System D)
	scalable(4, 10, 0|
		chmFrontLeft|chmFrontCenter|chmFrontRight|
		chmSideLeft|chmSideRight|
		chmTopFrontLeft|chmTopFrontRight|
		chmTopBackLeft|chmTopBackRight|
		chmLowFrequency),

	// 5: 7.1ch (L/C/R/Lss/Rss/Lrs/Rrs/LFE)
	// the config of (0+7+0) of [ITU-2051-3] (Sound System I)
	scalable(5, 8, 0|
		chmFrontLeft|chmFrontCenter|chmFrontRight|
		chmSideSurroundLeft|chmSideSurroundRight|
		chmBackLeft|chmBackRight|
		chmLowFrequency),

	// 6: 7.1.2ch (L/C/R/Lss/Rss/Lrs/Rrs/Ltf/Rtf/LFE)
	// The combination of 7.1ch and the Left and Right top front pair of 7.1.4ch
	scalable(6, 10, 0|
		chmFrontLeft|chmFrontCenter|chmFrontRight|
		chmSideSurroundLeft|chmSideSurroundRight|
		chmBackLeft|chmBackRight|
		chmTopFrontLeft|chmTopFrontRight|
		chmLowFrequency),

	// 7: 7.1.4ch (L/C/R/Lss/Rss/Lrs/Rrs/Ltf/Rtf/Ltb/Rtb/LFE)
	// the config of (4+7+0) of [ITU-2051-3] (Sound System J)
	scalable(7, 12, 0|
		chmFrontLeft|chmFrontCenter|chmFrontRight|
		chmSideSurroundLeft|chmSideSurroundRight|
		chmBackLeft|chmBackRight|
		chmTopFrontLeft|chmTopFrontRight|
		chmTopBackLeft|chmTopBackRight|
		chmLowFrequency),

	// 8: 3.1.2ch (L/C/R/Ltf/Rtf/LFE)
	// The front subset of 7.1.4ch (Sound System J)
	scalable(8, 6, 0|
		chmFrontLeft|chmFrontCenter|chmFrontRight|
		chmTopFrontLeft|chmTopFrontRight|
		chmLowFrequency),

	// 9: Binaural (L/R)
	// the binaural channels
	scalable(9, 2,
		chmBinauralLeft|chmBinauralRight),

	// 10-14: Reserved for future use

	// 15: Expanded channel layouts - defined in expanded_loudspeaker_layout field bellow
}

func expanded(index int, channels int, mask channelMask) channelLayout {
	return channelLayout{
		Channels: channels,
		Order:    coExpanded,
		Mask:     mask,
		Map:      nil,
	}
}

// expandedScalableChannelLayouts defines an array of expanded scalable channel layouts
// Based on IAMF expanded_loudspeaker_layout specification
// https://aomediacodec.github.io/iamf/#expanded_loudspeaker_layout
// https://www.itu.int/dms_pubrec/itu-r/rec/bs/R-REC-BS.2051-3-202205-I!!PDF-E.pdf
var expandedScalableChannelLayouts = []channelLayout{

	// 0: LFE - The low-frequency effects subset (LFE) of 7.1.4ch (Sound System J)
	expanded(0, 1,
		chmLowFrequency),

	// 1: Stereo-S (Ls/Rs) - The surround subset of 5.1.4ch (Sound System I)
	expanded(1, 2,
		chmSideLeft|chmSideRight),

	// 2: Stereo-SS (Lss/Rss) - The side surround subset of 7.1.4ch (Sound System J)
	// it is wrong in ffmpeg
	expanded(2, 2,
		chmSideSurroundLeft|chmSideSurroundRight),

	// 3: Stereo-RS (Lrs/Rrs) - The rear surround subset of 7.1.4ch (Sound System J)
	expanded(3, 2,
		chmBackLeft|chmBackRight),

	// 4: Stereo-TF (Ltf/Rtf) - The top front subset of 7.1.4ch (Sound System J)
	expanded(4, 2,
		chmTopFrontLeft|chmTopFrontRight),

	// 5: Stereo-TB (Ltb/Rtb) - The top back subset of 7.1.4ch (Sound System J)
	expanded(5, 2,
		chmTopBackLeft|chmTopBackRight),

	// 6: Top-4ch (Ltf/Rtf/Ltb/Rtb) - The top 4 channels of 7.1.4ch (Sound System J)
	expanded(6, 4,
		chmTopFrontLeft|chmTopFrontRight|chmTopBackLeft|chmTopBackRight),

	// 7: 3.0ch (L/C/R) - The front 3 channels of 7.1.4ch (Sound System J)
	expanded(7, 3,
		chmFrontLeft|chmFrontCenter|chmFrontRight),

	// 8: 9.1.6ch - FLc/FC/FRc/FL/FR/SiL/SiR/BL/BR/
	//              TpFL/TpFR/TpSiL/TpSiR/TpBL/TpBR/LFE1
	// The subset of (9+10+3) of [ITU-2051-3] (Sound System H)
	expanded(8, 16, 0|
		chmFrontLeftOfCenter|chmFrontCenter|chmFrontRightOfCenter|
		chmFrontLeft|chmFrontRight|
		chmSideSurroundLeft|chmSideSurroundRight|
		chmBackLeft|chmBackRight|
		chmTopFrontLeft|chmTopFrontRight|
		chmTopSideLeft|chmTopSideRight|
		chmTopBackLeft|chmTopBackRight|
		chmLowFrequency),

	// 9: Stereo-F (FL/FR) - The front subset of 9.1.6ch (Sound System H)
	expanded(9, 2,
		chmFrontLeft|chmFrontRight),

	// 10: Stereo-Si (SiL/SiR) - The side subset of 9.1.6ch (Sound System H)
	expanded(10, 2,
		chmSideSurroundLeft|chmSideSurroundRight),

	// 11: Stereo-TpSi (TpSiL/TpSiR) - The top side subset of 9.1.6ch (Sound System H)
	expanded(11, 2,
		chmTopSideLeft|chmTopSideRight),

	// 12: Top-6ch (TpFL/TpFR/TpSiL/TpSiR/TpBL/TpBR)
	// The top 6 channels of 9.1.6ch (Sound System H)
	expanded(12, 6, 0|
		chmTopFrontLeft|chmTopFrontRight|
		chmTopSideLeft|chmTopSideRight|
		chmTopBackLeft|chmTopBackRight),

	// ffmpeg table is missing the following ones

	// 13: 10.2.9.3ch - FLc/FC/FRc/FL/FR/SiL/SiR/BL/BC/BR/
	//                  TpFL/TpFC/TpFR/TpSiL/TpC/TpSiR/
	//                  TpBL/TpBC/TpBR/BtFL/BtFC/BtFR/LFE1/LFE2
	// The subset of (9+10+3) of [ITU-2051-3] (Sound System H)
	expanded(13, 24, 0|
		chmFrontLeftOfCenter|chmFrontCenter|chmFrontRightOfCenter|
		chmFrontLeft|chmFrontRight|
		chmSideSurroundLeft|chmSideSurroundRight|
		chmBackLeft|chmBackCenter|chmBackRight|
		chmTopFrontLeft|chmTopFrontCenter|chmTopFrontRight|
		chmTopSideLeft|chmTopCenter|chmTopSideRight|
		chmTopBackLeft|chmTopBackCenter|chmTopBackRight|
		chmBottomFrontLeft|chmBottomFrontCenter|chmBottomFrontRight|
		chmLowFrequency|chmLowFrequency2),

	// 14: LFE-Pair (LFE1/LFE2)
	// The low-frequency effects subset of 10.2.9.3ch (Sound System H)
	expanded(14, 2,
		chmLowFrequency|chmLowFrequency2),

	// 15: Bottom-3ch (BtFL/BtFC/BtFR)
	// The bottom 3 channels of 10.2.9.3ch (Sound System H)
	expanded(15, 3,
		chmBottomFrontLeft|chmBottomFrontCenter|chmBottomFrontRight),

	// 16: 7.1.5.4ch - L/C/R/Lss/Rss/Lrs/Rrs/Ltf/Rtf/TpC/
	//                 Ltb/Rtb/BtFL/BtFR/BtBL/BtBR/LFE
	// Top and bottom speakers added to (4+7+0) of [ITU-2051-3] (Sound System J)
	expanded(16, 16, 0|
		chmFrontLeft|chmFrontCenter|chmFrontRight|
		chmSideSurroundLeft|chmSideSurroundRight|
		chmBackLeft|chmBackRight|
		chmTopFrontLeft|chmTopFrontRight|
		chmTopCenter|
		chmTopBackLeft|chmTopBackRight|
		chmBottomFrontLeft|chmBottomFrontRight|
		chmBottomBackLeft|chmBottomBackRight|
		chmLowFrequency),

	// 17: Bottom-4ch (BtFL/BtFR/BtBL/BtBR)
	// The bottom 4 channels of 7.1.5.4ch (Sound System J)
	expanded(17, 4, 0|
		chmBottomFrontLeft|chmBottomFrontRight|
		chmBottomBackLeft|chmBottomBackRight),

	// 18: Top-1ch (TpC) - The top subset of 7.1.5.4ch (Sound System J)
	expanded(18, 1,
		chmTopCenter),

	// 19: Top-5ch (Ltf/Rtf/TpC/Ltb/Rtb)
	// The top 5 channels of 7.1.5.4ch (Sound System J)
	expanded(19, 5, 0|
		chmTopFrontLeft|chmTopFrontRight|
		chmTopCenter|
		chmTopBackLeft|chmTopBackRight),

	// 20-255: Reserved for future use
}

// specific channel layouts
var channelLayoutBinaural = scalableChannelLayouts[9]

// soundSystemMap defines a struct for system maps
type soundSystemMap struct {
	SoundSystem SoundSystem
	Layout      channelLayout
}

// 5.1.4ch + Bottom Front Center (not in standard layouts)
var systemE = expanded(-1, 11, 0|
	chmFrontLeft|chmFrontCenter|chmFrontRight|
	chmSideLeft|chmSideRight|
	chmTopFrontLeft|chmTopFrontRight|
	chmTopBackLeft|chmTopBackRight|
	chmBottomFrontCenter|
	chmLowFrequency)

// 7.1.2ch + TpBC + LFE2 (not in standard layouts)
var systemF = expanded(-2, 12, 0|
	chmFrontLeft|chmFrontCenter|chmFrontRight|
	chmSideLeft|chmSideRight|
	chmBackLeft|chmBackRight|
	chmTopFrontLeft|chmTopFrontRight|
	chmTopBackLeft|chmTopBackCenter|chmLowFrequency2)

// 9.1.4ch (not in standard layouts)
var systemG = expanded(-3, 14, 0|
	chmFrontLeft|chmFrontCenter|chmFrontRight|
	chmFrontLeftOfCenter|chmFrontRightOfCenter|
	chmSideLeft|chmSideRight|
	chmBackLeft|chmBackRight|
	chmTopFrontLeft|chmTopFrontRight|
	chmTopBackLeft|chmTopBackRight|
	chmLowFrequency)

// mapping between IAMF types and the structs above
var iamfSoundSystemMap = []soundSystemMap{
	{SoundSystemA_0_2_0, scalableChannelLayouts[1]},           // Stereo
	{SoundSystemB_0_5_0, scalableChannelLayouts[2]},           // 5.1ch
	{SoundSystemC_2_5_0, scalableChannelLayouts[3]},           // 5.1.2ch
	{SoundSystemD_4_5_0, scalableChannelLayouts[4]},           // 5.1.4ch
	{SoundSystemE_4_5_1, systemE},                             // 5.1.4ch + BFC
	{SoundSystemF_3_7_0, systemF},                             // 7.1.2ch + TpBC + LFE2
	{SoundSystemG_4_9_0, systemG},                             // 9.1.4ch
	{SoundSystemH_9_10_3, expandedScalableChannelLayouts[13]}, // 10.2.9.3ch
	{SoundSystemI_0_7_0, scalableChannelLayouts[5]},           // 7.1ch
	{SoundSystemJ_4_7_0, scalableChannelLayouts[7]},           // 7.1.4ch
	{SoundSystem10_2_7_0, scalableChannelLayouts[6]},          // 7.1.2ch
	{SoundSystem11_2_3_0, scalableChannelLayouts[8]},          // 3.1.2ch
	{SoundSystem12_0_1_0, scalableChannelLayouts[0]},          // Mono
	{SoundSystem13_9_1_6, expandedScalableChannelLayouts[8]},  // 9.1.6ch
}

/**
 * implementation.
 * Based on IAMF_PARSE_C
 */

// Constants for IAMF parsing
const (
	MaxIamfObuHeaderSizeBytes = 1 + 8*3
	MaxIamfLabelSize          = 128
)

// OpusDecoderConfig parses Opus decoder configuration
func OpusDecoderConfig(sr bits.SliceReader, codecConfig *IamfCodecConfig) error {
	left := sr.NrRemainingBytes()
	if left < 11 || codecConfig.AudioRollDistance >= 0 {
		return errors.New("Invalid opus decoder config")
	}

	extradata := make([]byte, left+8)
	extradata[0] = 'O'
	extradata[1] = 'p'
	extradata[2] = 'u'
	extradata[3] = 's'
	extradata[4] = 'H'
	extradata[5] = 'e'
	extradata[6] = 'a'
	extradata[7] = 'd'

	opusData := sr.ReadBytes(left)
	if sr.AccError() != nil {
		return sr.AccError()
	}
	copy(extradata[8:], opusData)

	codecConfig.Extradata = extradata
	codecConfig.ExtradataSize = int32(left + 8)
	codecConfig.SampleRate = 48000

	return nil
}

func mp4ReadDescr(sr bits.SliceReader) (uint8, error) {
	tag := sr.ReadUint8()
	if sr.AccError() != nil {
		return 0, sr.AccError()
	}
	sr.ReadUint8() // descLen
	if sr.AccError() != nil {
		return 0, sr.AccError()
	}
	return tag, nil
}

// AACDecoderConfig parses AAC decoder configuration
func AacDecoderConfig(sr bits.SliceReader, codecConfig *IamfCodecConfig) error {
	if codecConfig.AudioRollDistance >= 0 {
		return errors.New("Invalid aac decoder config")
	}

	tag, err := mp4ReadDescr(sr)
	if err != nil {
		return err
	}
	if tag != 0x03 { // MP4DecConfigDescrTag
		return errors.New("Invalid mp4 descriptor tag")
	}

	objectTypeID := sr.ReadUint8()
	if sr.AccError() != nil {
		return sr.AccError()
	}
	if objectTypeID != 0x40 {
		return errors.New("invalid object type id for aac")
	}

	streamType := sr.ReadUint8()
	if sr.AccError() != nil {
		return sr.AccError()
	}
	if ((streamType >> 2) != 5) || (((streamType >> 1) & 1) != 0) {
		return errors.New("invalid stream type for aac")
	}

	sr.SkipBytes(3) // buffer size db
	sr.SkipBytes(4) // rc_max_rate
	sr.SkipBytes(4) // avg bitrate

	specTag, err := mp4ReadDescr(sr)
	if err != nil {
		return err
	}
	if specTag != 0x05 { // MP4DecSpecificDescrTag
		return errors.New("invalid mp4 specific descriptor tag")
	}

	specLen := sr.ReadUint8()
	if sr.AccError() != nil {
		return sr.AccError()
	}
	if specLen == 0 {
		return errors.New("aac decoder specific descriptor has zero length")
	}

	extradata := sr.ReadBytes(int(specLen))
	if sr.AccError() != nil {
		return sr.AccError()
	}

	codecConfig.Extradata = extradata
	codecConfig.ExtradataSize = int32(len(extradata))

	buf := bytes.NewBuffer(extradata)
	aac, err := aac.DecodeAudioSpecificConfig(buf)
	if err != nil {
		codecConfig.SampleRate = int32(aac.SamplingFrequency)
	}

	return nil
}

// FLACDecoderConfig parses FLAC decoder configuration
func FlacDecoderConfig(sr bits.SliceReader, codecConfig *IamfCodecConfig) error {
	if codecConfig.AudioRollDistance != 0 {
		return errors.New("invalid flac decoder config")
	}

	// Skip METADATA_BLOCK_HEADER (4 bytes)
	sr.SkipBytes(4)
	if sr.AccError() != nil {
		return sr.AccError()
	}

	left := sr.NrRemainingBytes()
	if left < 18 { // FLAC_STREAMINFO_SIZE
		return errors.New("flac streaminfo too small")
	}

	extradata := sr.ReadBytes(left)
	if sr.AccError() != nil {
		return sr.AccError()
	}

	codecConfig.Extradata = extradata
	codecConfig.ExtradataSize = int32(len(extradata))

	// Extract sample rate from STREAMINFO
	if len(extradata) >= 13 {
		xsr := bits.NewFixedSliceReader(extradata)
		buf := bytes.NewBuffer(xsr.ReadBytes(10))
		br := bits.NewReader(buf)
		sampleRate := br.Read(24) >> 4
		codecConfig.SampleRate = int32(sampleRate)
	}

	return nil
}

// PCMDecoderConfig parses PCM decoder configuration
func PcmDecoderConfig(sr bits.SliceReader, codecConfig *IamfCodecConfig) error {
	sampleFormat := sr.ReadUint8() // 0 = BE, 1 = LE
	if sr.AccError() != nil {
		return sr.AccError()
	}

	sampleSizeRaw := sr.ReadUint8()
	if sr.AccError() != nil {
		return sr.AccError()
	}

	sampleSize := (sampleSizeRaw / 8) - 2 // 16, 24, 32 bits

	if sampleFormat > 1 || sampleSize > 2 || codecConfig.AudioRollDistance != 0 {
		return errors.New("invalid pcm decoder config")
	}

	sampleRate := sr.ReadUint32()
	if sr.AccError() != nil {
		return sr.AccError()
	}

	if sr.NrRemainingBytes() > 0 {
		return errors.New("extra data in pcm decoder config")
	}

	codecConfig.SampleRate = int32(sampleRate)

	// Map codec IDs based on format and size
	codecs := [2][3]string{
		{"pcm_s16be", "pcm_s24be", "pcm_s32be"},
		{"pcm_s16le", "pcm_s24le", "pcm_s32le"},
	}
	codecConfig.CodecID = codecs[sampleFormat][sampleSize]

	return nil
}

// codecConfigObu parses a Codec Config OBU
func codecConfigObu(sr bits.SliceReader, ctx *IamfContext) error {
	codecConfigID, err := ReadLeb128(sr)
	if err != nil {
		return err
	}

	codecId := sr.ReadBytes(4)

	numSamples, err := ReadLeb128(sr)
	if err != nil {
		return err
	}

	audioRollDistance := sr.ReadInt16()

	// Map codec ID to internal representation
	strCodecID := string(codecId)
	switch strCodecID {
	case "Opus":
		strCodecID = "opus"
	case "mp4a":
		strCodecID = "aac"
	case "fLaC":
		strCodecID = "flac"
	case "ipcm":
		strCodecID = "pcm"
	default:
		strCodecID = "none"
	}

	for _, cc := range ctx.CodecConfigs[:ctx.NumCodecConfigs] {
		if cc.CodecConfigID == uint32(codecConfigID) {
			return fmt.Errorf("duplicate codec config id %d", codecConfigID)
		}
	}

	codecConfig := &IamfCodecConfig{
		CodecConfigID:     uint32(codecConfigID),
		CodecID:           strCodecID,
		NumSamples:        uint32(numSamples),
		AudioRollDistance: int32(audioRollDistance),
	}

	switch strCodecID {
	case "opus":
		if err := OpusDecoderConfig(sr, codecConfig); err != nil {
			return err
		}
	case "aac":
		if err := AacDecoderConfig(sr, codecConfig); err != nil {
			return err
		}
	case "flac":
		if err := FlacDecoderConfig(sr, codecConfig); err != nil {
			return err
		}
	case "pcm":
		if err := PcmDecoderConfig(sr, codecConfig); err != nil {
			return err
		}
	default:
	}

	if codecConfig.NumSamples > math.MaxInt32 || codecConfig.NumSamples == 0 {
		return errors.New("invalid sample count")
	}

	negRollDistance := -codecConfig.AudioRollDistance
	if negRollDistance > 0 && negRollDistance > math.MaxInt32/int32(codecConfig.NumSamples) {
		return errors.New("invalid audio roll distance")
	}

	ctx.CodecConfigs = append(ctx.CodecConfigs, codecConfig)
	ctx.NumCodecConfigs++

	return nil
}

// scalableChannelLayoutConfig parses scalable channel layout configuration
func scalableChannelLayoutConfig(sr bits.SliceReader, ctx *IamfContext, audioElement *IamfAudioElement) error {
	uNumLayers := sr.ReadUint8() >> 5

	numLayers := int(uNumLayers)
	if numLayers > 6 || numLayers == 0 {
		return errors.New("invalid number of layers")
	}

	audioElement.Layers = make([]*IamfLayer, numLayers)
	audioElement.NumLayers = uint32(numLayers)

	k := 0

	for i := 0; i < numLayers; i++ {
		byte1 := sr.ReadUint8()
		loudspeakerLayout := (byte1 >> 4)
		outputGainIsPresent := (byte1 >> 3) & 1
		reconGainIsPresent := (byte1 >> 2) & 1
		// Bits 0-1 are reserved

		substreamCount := int(sr.ReadUint8())
		coupledSubstreamCount := int(sr.ReadUint8())

		if sr.AccError() != nil {
			return sr.AccError()
		}

		if substreamCount == 0 || coupledSubstreamCount > substreamCount ||
			substreamCount+k > int(audioElement.NumSubstreams) {
			return errors.New("invalid substream configuration")
		}

		outputGainFlags := byte(0)
		outputGain := Rational{}
		if outputGainIsPresent > 0 {
			outputGainFlags = sr.ReadUint8() >> 2
			outputGain = MakeRational(signExtend(sr.ReadUint16()), 1<<8)
		}

		reconGainFlags := LayerFlag(0)
		if reconGainIsPresent > 0 {
			reconGainFlags |= LayerFlagReconGain
		}

		if sr.AccError() != nil {
			return sr.AccError()
		}

		expandedLoudspeakerLayout := -1
		if loudspeakerLayout == 15 {
			if i > 0 {
				return errors.New("expanded loudspeaker layout with multiple layers")
			}
			expandedLoudspeakerLayout = int(sr.ReadUint8())
		}

		if sr.AccError() != nil {
			return sr.AccError()
		}

		var chLayout channelLayout
		if expandedLoudspeakerLayout >= 0 && expandedLoudspeakerLayout < len(expandedScalableChannelLayouts) {
			chLayout = expandedScalableChannelLayouts[expandedLoudspeakerLayout]
		} else if int(loudspeakerLayout) < len(scalableChannelLayouts) {
			chLayout = scalableChannelLayouts[loudspeakerLayout]
		} else {
			if expandedLoudspeakerLayout >= 0 {
				return fmt.Errorf("unknown expanded_loudspeaker_layout %d", expandedLoudspeakerLayout)
			} else {
				return fmt.Errorf("unknown loudspeaker_layout %d", loudspeakerLayout)
			}
		}

		channels := chLayout.Channels
		if i > 0 {
			prevLayer := audioElement.Element.Layers[i-1]
			prevChannels := int(prevLayer.ChannelLayout.NumChannels)
			if chLayout.Channels <= prevChannels {
				return errors.New("invalid channel layout: layer must have more channels than previous")
			}
			channels -= prevChannels
		}
		if channels != substreamCount+coupledSubstreamCount {
			return fmt.Errorf("channel count mismatch: expected %d, got %d",
				channels, substreamCount+coupledSubstreamCount)
		}

		iaLayer := &IamfLayer{
			SubstreamCount:        uint32(substreamCount),
			CoupledSubstreamCount: uint32(coupledSubstreamCount),
		}
		audioElement.Layers = append(audioElement.Layers, iaLayer)
		audioElement.NumLayers++

		layer := &Layer{
			ChannelLayout:   chLayout.toChannelLayout(),
			Flags:           reconGainFlags,
			OutputGainFlags: uint8(outputGainFlags),
			OutputGain:      outputGain,
		}
		audioElement.Element.Layers = append(audioElement.Element.Layers, layer)
		audioElement.Element.NumLayers++

		k += substreamCount
	}

	if k != int(audioElement.NumSubstreams) {
		return errors.New("substream count mismatch")
	}

	return nil
}

// ambisonicsConfig parses ambisonics configuration
func ambisonicsConfig(sr bits.SliceReader, audioElement *IamfAudioElement) error {
	ambisonicsMode, err := ReadLeb128(sr)
	if err != nil {
		return err
	}

	if ambisonicsMode > 1 {
		return errors.New("invalid ambisonics mode")
	}

	outputChannelCount := int(sr.ReadUint8())
	substreamCount := int(sr.ReadUint8())

	if int(audioElement.NumSubstreams) != substreamCount || outputChannelCount == 0 {
		return errors.New("invalid ambisonics configuration")
	}

	order := int(math.Floor(math.Sqrt(float64(outputChannelCount - 1))))
	/* incomplete order - some harmonics are missing */
	if (order+1)*(order+1) != outputChannelCount {
		return errors.New("incomplete ambisonics order")
	}

	coupledSubstreamCount := 0

	var layer *Layer
	if ambisonicsMode == 0 {
		channelMap := make(map[int]int)
		for i := 0; i < int(outputChannelCount); i++ {
			channelMap[i] = int(sr.ReadUint8())
		}

		layout := &channelLayout{
			Order:    coCustom,
			Channels: outputChannelCount,
			Map:      &channelMap,
		}

		layer = &Layer{
			ChannelLayout:  layout.toChannelLayout(),
			AmbisonicsMode: AmbisonicsModeMono,
		}
	} else {
		coupledSubstreamCount = int(sr.ReadUint8())
		count := substreamCount + coupledSubstreamCount
		numDemixingMatrix := count * outputChannelCount

		layout := &channelLayout{
			Order:    coAmbisonics,
			Channels: outputChannelCount,
		}

		demixingMatrix := make([]Rational, numDemixingMatrix)
		for i := 0; i < numDemixingMatrix; i++ {
			demixingMatrix[i] = MakeRational(signExtend(sr.ReadUint16()), 1<<15)
		}

		layer = &Layer{
			ChannelLayout:     layout.toChannelLayout(),
			AmbisonicsMode:    AmbisonicsModeProjection,
			DemixingMatrix:    demixingMatrix,
			NumDemixingMatrix: uint32(numDemixingMatrix),
		}
	}

	iaLayer := &IamfLayer{
		SubstreamCount:        uint32(substreamCount),
		CoupledSubstreamCount: uint32(coupledSubstreamCount),
	}
	audioElement.Layers = append(audioElement.Layers, iaLayer)
	audioElement.NumLayers++

	audioElement.Element.Layers = append(audioElement.Element.Layers, layer)
	audioElement.Element.NumLayers++

	return nil
}

// paramParse parses parameter definitions
func paramParse(sr bits.SliceReader, ctx *IamfContext, paramType ParamDefinitionType, audioElement *IamfAudioElement) (*ParamDefinition, error) {
	parameterID, err := ReadLeb128(sr)
	if err != nil {
		return nil, err
	}

	var paramDefinition *IamfParamDefinition
	for _, pd := range ctx.ParamDefinitions[:ctx.NumParamDefinitions] {
		if pd.Param.ParameterID == uint32(parameterID) {
			paramDefinition = pd
			break
		}
	}

	parameterRate, err := ReadLeb128(sr)
	if err != nil {
		return nil, err
	}

	mode := sr.ReadUint8() >> 7

	duration := uint32(0)
	constantSubblockDuration := uint32(0)
	numSubblocks := uint32(0)
	totalDuration := uint32(0)

	if mode == 0 {
		dur, err := ReadLeb128(sr)
		if err != nil {
			return nil, err
		}
		duration = uint32(dur)
		if dur == 0 {
			return nil, fmt.Errorf("zero duration in parameter id %d",
				parameterID)
		}

		subblockDur, err := ReadLeb128(sr)
		if err != nil {
			return nil, err
		}
		constantSubblockDuration = uint32(dur)
		if subblockDur == 0 {
			subBlocks, err := ReadLeb128(sr)
			if err != nil {
				return nil, err
			}
			numSubblocks = uint32(subBlocks)
		} else {
			if constantSubblockDuration > duration {
				return nil, fmt.Errorf("invalid block duration in parameter id %d",
					parameterID)
			}
			numSubblocks = duration / constantSubblockDuration
			totalDuration = duration
		}
	}

	if numSubblocks > duration {
		return nil, fmt.Errorf("invalid block duration in parameter id %d",
			parameterID)
	}

	subblocks := make([]interface{}, numSubblocks)
	for i := uint32(0); i < numSubblocks; i++ {
		subblockDuration := constantSubblockDuration
		if constantSubblockDuration == 0 {
			subBlocks, err := ReadLeb128(sr)
			if err != nil {
				return nil, err
			}
			subblockDuration = uint32(subBlocks)
			totalDuration += subblockDuration
		} else if i == numSubblocks-1 {
			subblockDuration = duration - i*constantSubblockDuration
		}

		var subblock interface{}
		switch paramType {
		case ParamDefinitionMixGain:
			subblock = MixGain{
				SubblockDuration: subblockDuration,
			}
		case ParamDefinitionDemixing:
			dmixpMode := sr.ReadUint8() >> 5
			defaultW := sr.ReadUint8() >> 4

			subblock = DemixingInfo{
				SubblockDuration: subblockDuration,
				DmixpMode:        uint32(dmixpMode),
			}
			if audioElement != nil {
				audioElement.Element.DefaultW = uint32(defaultW)
			}
		case ParamDefinitionReconGain:
			subblock = ReconGain{
				SubblockDuration: subblockDuration,
			}
		}

		subblocks[i] = subblock
	}

	if mode == 0 && constantSubblockDuration == 0 && totalDuration != duration {
		return nil, errors.New("subblock durations don't match total duration")
	}

	if paramDefinition == nil {
		paramDefinition = &IamfParamDefinition{
			Mode:         int32(mode),
			ParamSize:    0,
			AudioElement: audioElement,
			Param: ParamDefinition{
				Type:                     paramType,
				ParameterID:              uint32(parameterID),
				ParameterRate:            uint32(parameterRate),
				Duration:                 duration,
				ConstantSubblockDuration: constantSubblockDuration,
				NumSubblocks:             numSubblocks,
				Subblocks:                subblocks,
			},
		}

		ctx.ParamDefinitions = append(ctx.ParamDefinitions, paramDefinition)
		ctx.NumParamDefinitions++
	}

	return &paramDefinition.Param, nil
}

// audioElementObu parses an Audio Element OBU
func audioElementObu(sr bits.SliceReader, ctx *IamfContext) error {
	audioElementID, err := ReadLeb128(sr)
	if err != nil {
		return err
	}

	for _, ae := range ctx.AudioElements[:ctx.NumAudioElements] {
		if ae.AudioElementID == uint32(audioElementID) {
			return fmt.Errorf("duplicate audio element id %d", audioElementID)
		}
	}

	audioElementType := AudioElementType(sr.ReadUint8() >> 5)
	if audioElementType > AudioElementTypeScene {
		return errors.New("unknown audio element type")
	}

	codecConfigID, err := ReadLeb128(sr)
	if err != nil {
		return err
	}

	var codecConfig *IamfCodecConfig
	for _, cc := range ctx.CodecConfigs[:ctx.NumCodecConfigs] {
		if cc.CodecConfigID == uint32(codecConfigID) {
			codecConfig = cc
			break
		}
	}
	if codecConfig == nil {
		return fmt.Errorf("non-existent codec config id %d", codecConfigID)
	}

	numSubstreams, err := ReadLeb128(sr)
	if err != nil {
		return err
	}

	substreams := make([]*IamfSubStream, numSubstreams)
	for i := uint32(0); i < uint32(numSubstreams); i++ {
		substreamID, err := ReadLeb128(sr)
		if err != nil {
			return err
		}

		substreams[i] = &IamfSubStream{
			AudioSubstreamID: uint32(substreamID),
			CodecParameters: CodecParameters{
				Ptr: codecConfig,
			},
		}
	}

	audioElement := &IamfAudioElement{
		AudioElementID: uint32(audioElementID),
		CodecConfigID:  uint32(codecConfigID),
		NumSubstreams:  uint32(numSubstreams),
		Substreams:     substreams,
		Element: AudioElement{
			AudioElementType: audioElementType,
		},
	}

	numParameters, err := ReadLeb128(sr)
	if err != nil {
		return err
	}

	if numParameters > 2 && audioElementType == AudioElementTypeChannel {
		return errors.New("too many parameters for channel element")
	}
	if numParameters > 0 && audioElementType == AudioElementTypeScene {
		return errors.New("scene elements cannot have parameters")
	}

	for i := uint32(0); i < uint32(numParameters); i++ {
		val, err := ReadLeb128(sr)
		if err != nil {
			return err
		}
		paramType := ParamDefinitionType(val)

		switch paramType {
		case ParamDefinitionMixGain:
			return fmt.Errorf("invalid param type %d", paramType)
		case ParamDefinitionDemixing:
			if audioElement.Element.DemixingInfo != nil {
				return errors.New("invalid data")
			}
			paramDef, err := paramParse(sr, ctx, paramType, audioElement)
			if err != nil {
				return err
			}
			audioElement.Element.DemixingInfo = paramDef
		case ParamDefinitionReconGain:
			if audioElement.Element.ReconGainInfo != nil {
				return errors.New("invalid data")
			}
			paramDef, err := paramParse(sr, ctx, paramType, audioElement)
			if err != nil {
				return err
			}
			audioElement.Element.ReconGainInfo = paramDef
		default:
			paramDefinitionSize, err := ReadLeb128(sr)
			if err != nil {
				return err
			}
			sr.SkipBytes(int(paramDefinitionSize))
		}
	}

	switch audioElementType {
	case AudioElementTypeChannel:
		if err := scalableChannelLayoutConfig(sr, ctx, audioElement); err != nil {
			return err
		}
	case AudioElementTypeScene:
		if err := ambisonicsConfig(sr, audioElement); err != nil {
			return err
		}
	default:
		return errors.New("type should have been checked above")
	}

	ctx.AudioElements = append(ctx.AudioElements, audioElement)
	ctx.NumAudioElements++

	return nil
}

// mixPresentationObu parses a Mix Presentation OBU
func mixPresentationObu(sr bits.SliceReader, ctx *IamfContext) error {
	mixPresentationID, err := ReadLeb128(sr)
	if err != nil {
		return err
	}

	for _, mp := range ctx.MixPresentations[:ctx.NumMixPresentations] {
		if mp.MixPresentationID == uint32(mixPresentationID) {
			return fmt.Errorf("duplicate mix presentation id %d", mixPresentationID)
		}
	}

	countLabel, err := ReadLeb128(sr)
	if err != nil {
		return err
	}

	languageLabels := make([]string, countLabel)
	for i := uint32(0); i < uint32(countLabel); i++ {
		label := sr.ReadZeroTerminatedString(MaxIamfLabelSize)
		if sr.AccError() != nil {
			return sr.AccError()
		}
		languageLabels[i] = label
	}

	annotations := make(map[string]string)
	for i := uint32(0); i < uint32(countLabel); i++ {
		annotation := sr.ReadZeroTerminatedString(MaxIamfLabelSize)
		if sr.AccError() != nil {
			return sr.AccError()
		}
		annotations[languageLabels[i]] = annotation
	}

	numSubmixes, err := ReadLeb128(sr)
	if err != nil {
		return err
	}

	submixes := make([]*Submix, numSubmixes)
	for i := uint32(0); i < uint32(numSubmixes); i++ {
		numElements, err := ReadLeb128(sr)
		if err != nil {
			return err
		}

		submixElements := make([]*SubmixElement, numElements)
		for j := uint32(0); j < uint32(numElements); j++ {
			audioElementID, err := ReadLeb128(sr)
			if err != nil {
				return err
			}

			var audioElement *IamfAudioElement
			for _, ae := range ctx.AudioElements[:ctx.NumAudioElements] {
				if ae.AudioElementID == uint32(audioElementID) {
					audioElement = ae
					break
				}
			}
			if audioElement == nil {
				return fmt.Errorf("invalid audio element id %d referenced by mix parameter %d",
					audioElementID, mixPresentationID)
			}

			elemAnnotations := make(map[string]string)
			for k := uint32(0); k < uint32(countLabel); k++ {
				elemAnnotation := sr.ReadZeroTerminatedString(MaxIamfLabelSize)
				if sr.AccError() != nil {
					return sr.AccError()
				}
				elemAnnotations[languageLabels[k]] = elemAnnotation
			}

			headphonesRenderingMode := HeadphonesMode(sr.ReadUint8() >> 6)

			renderingConfigExtSize, err := ReadLeb128(sr)
			if err != nil {
				return err
			}
			sr.SkipBytes(int(renderingConfigExtSize))

			elementMixGain, err := paramParse(sr, ctx, ParamDefinitionMixGain, audioElement)
			if err != nil {
				return err
			}

			defaultMixGain := MakeRational(signExtend(sr.ReadUint16()), 1<<8)

			submixElements[j] = &SubmixElement{
				AudioElementID:          uint32(audioElementID),
				ElementMixConfig:        elementMixGain,
				DefaultMixGain:          defaultMixGain,
				HeadphonesRenderingMode: headphonesRenderingMode,
				Annotations:             elemAnnotations,
			}
		}

		outputMixConfig, err := paramParse(sr, ctx, ParamDefinitionMixGain, nil)
		if err != nil {
			return err
		}

		defaultMixGain := MakeRational(signExtend(sr.ReadUint16()), 1<<8)

		nbLayouts, err := ReadLeb128(sr)
		if err != nil {
			return err
		}

		submixLayouts := make([]*SubmixLayout, nbLayouts)
		for j := uint32(0); j < uint32(nbLayouts); j++ {
			typeByte := sr.ReadUint8()

			layoutType := SubMixLayoutType(typeByte >> 6)
			if layoutType < SubMixLayoutTypeLoudspeakers || layoutType > SubMixLayoutTypeBinaural {
				return fmt.Errorf("invalid layout type %d referenced by mix parameter %d",
					layoutType, mixPresentationID)
			}

			var soundSystem ChannelLayout
			if layoutType == SubMixLayoutTypeLoudspeakers {
				system := (typeByte >> 2) & 0xF
				if int(system) >= len(iamfSoundSystemMap) {
					return fmt.Errorf("invalid loudspeak layout %d referenced by mix parameter %d",
						system, mixPresentationID)
				}
				soundSystem = iamfSoundSystemMap[system].Layout.toChannelLayout()
			} else {
				soundSystem = channelLayoutBinaural.toChannelLayout()
			}

			infoType := sr.ReadUint8()

			integratedLoudness := MakeRational(signExtend(sr.ReadUint16()), 1<<8)
			digitalPeak := MakeRational(signExtend(sr.ReadUint16()), 1<<8)

			truePeak := Rational{}
			if infoType&1 > 0 {
				truePeak = MakeRational(signExtend(sr.ReadUint16()), 1<<8)
			}

			dialogueAnchoredLoudness := Rational{}
			albumAnchoredLoudness := Rational{}
			if infoType&2 > 0 {
				numAnchoredLoudness := sr.ReadUint8()
				for k := uint32(0); k < uint32(numAnchoredLoudness); k++ {
					anchorElement := AnchorElement(sr.ReadUint8())
					anchoredLoudness := MakeRational(signExtend(sr.ReadUint16()), 1<<8)

					switch anchorElement {
					case AnchorElementDialogue:
						dialogueAnchoredLoudness = anchoredLoudness
					case AnchorElementAlbum:
						albumAnchoredLoudness = anchoredLoudness
					}
				}
			}

			if infoType&0xFC > 0 {
				infoTypeSize, err := ReadLeb128(sr)
				if err != nil {
					return err
				}
				sr.SkipBytes(int(infoTypeSize))
			}

			submixLayouts[j] = &SubmixLayout{
				LayoutType:               layoutType,
				SoundSystem:              soundSystem,
				IntegratedLoudness:       integratedLoudness,
				DigitalPeak:              digitalPeak,
				TruePeak:                 truePeak,
				DialogueAnchoredLoudness: dialogueAnchoredLoudness,
				AlbumAnchoredLoudness:    albumAnchoredLoudness,
			}
		}

		submixes[i] = &Submix{
			NumElements:     uint32(len(submixElements)),
			Elements:        submixElements,
			NumLayouts:      uint32(len(submixLayouts)),
			Layouts:         submixLayouts,
			DefaultMixGain:  defaultMixGain,
			OutputMixConfig: outputMixConfig,
		}
	}

	mixPresentation := &IamfMixPresentation{
		MixPresentationID: uint32(mixPresentationID),
		CountLabel:        uint32(countLabel),
		LanguageLabel:     languageLabels,
		Mix: MixPresentation{
			MixPresentationID: uint32(mixPresentationID),
			NumSubmixes:       uint32(len(submixes)),
			Submixes:          submixes,
			Annotations:       annotations,
		},
	}

	ctx.MixPresentations = append(ctx.MixPresentations, mixPresentation)
	ctx.NumMixPresentations++

	return nil
}

// parseObuSR parses an IAMF OBU
func parseObuSR(sr bits.SliceReader) (ObuInfo, error) {
	if sr.NrRemainingBytes() < 1 {
		return ObuInfo{}, errors.New("insufficient data for obu header")
	}
	offset := sr.GetPos()

	buf := bytes.NewBuffer(sr.ReadBytes(1))
	br := bits.NewReader(buf)

	// Read OBU type (5 bits)
	obuTypeVal := br.Read(5)
	obuType := ObuType(obuTypeVal)
	/* redundant := */ br.ReadFlag()
	trimming := br.ReadFlag()
	extensionFlag := br.ReadFlag()

	obuSize, err := ReadLeb128(sr)
	if err != nil {
		return ObuInfo{}, err
	}
	if obuSize > math.MaxInt32 {
		return ObuInfo{}, errors.New("obu size exceeds maximum")
	}

	if trimming {
		_, _ = ReadLeb128(sr) // num_samples_to_trim_at_end
		_, _ = ReadLeb128(sr) // num_samples_to_trim_at_start
	}

	if extensionFlag {
		extensionBytes, err := ReadLeb128(sr)
		if err != nil {
			return ObuInfo{}, err
		}
		if extensionBytes > math.MaxInt32/8 {
			return ObuInfo{}, errors.New("extension bytes too large")
		}
		sr.SkipBytes(int(extensionBytes))
	}

	if sr.NrRemainingBytes() == 0 {
		return ObuInfo{}, errors.New("insufficient data")
	}

	// headerSize including trimming and extension
	headerSize := sr.GetPos() - offset

	size := headerSize + int(obuSize)
	if size < 0 {
		return ObuInfo{}, errors.New("invalid obu size")
	}

	return ObuInfo{
		Size:  uint(size),
		Start: headerSize,
		Type:  obuType,
	}, nil
}

type ObuReader struct {
	ctx     *IamfContext
	sr      bits.SliceReader
	maxSize int
}

func NewObuReader(data []byte, maxSize int) ObuReader {
	if maxSize < MaxIamfObuHeaderSizeBytes {
		maxSize = MaxIamfObuHeaderSizeBytes
	}
	return ObuReader{
		ctx:     &IamfContext{},
		sr:      bits.NewFixedSliceReader(data),
		maxSize: maxSize,
	}
}

func (r ObuReader) ReadObu() (*ObuInfo, error) {
	if r.sr.GetPos() > r.maxSize {
		return nil, nil
	}

	if r.sr.NrRemainingBytes() <= 0 {
		return nil, nil
	}

	obu, err := parseObuSR(r.sr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse obu header: %w", err)
	}

	if obu.PayloadSize() > r.sr.NrRemainingBytes() {
		return nil, errors.New("obu size exceeds remaining data")
	}

	if obu.Type >= ObuTypeParameterBlock && obu.Type < ObuTypeSequenceHeader {
		return nil, nil
	}

	return &obu, nil
}

func (r *ObuReader) SkipPayload(obu *ObuInfo) {
	payloadSize := obu.PayloadSize()
	if payloadSize > 0 {
		r.sr.SkipBytes(payloadSize)
	}
}

func (r ObuReader) Context() *IamfContext {
	return r.Context()
}

// ReadDescriptors reads and parses IAMF descriptors
func (obu ObuInfo) ReadDescriptors(r *ObuReader) (*IamfContext, error) {
	data := r.sr.ReadBytes(obu.PayloadSize())
	ssr := bits.NewFixedSliceReader(data)

	switch obu.Type {
	case ObuTypeCodecConfig:
		if err := codecConfigObu(ssr, r.ctx); err != nil {
			return nil, fmt.Errorf("failed to parse codec config obu: %w", err)
		}
		return r.ctx, nil
	case ObuTypeAudioElement:
		if err := audioElementObu(ssr, r.ctx); err != nil {
			return nil, fmt.Errorf("failed to parse audio element obu: %w", err)
		}
		// remove already shown
		conf := *r.ctx
		conf.CodecConfigs = nil
		conf.NumCodecConfigs = 0
		ae := conf.AudioElements
		if len(ae) > 0 {
			conf.AudioElements = []*IamfAudioElement{ae[len(ae)-1]}
			conf.NumAudioElements = 1
		}
		return &conf, nil
	case ObuTypeMixPresentation:
		if err := mixPresentationObu(ssr, r.ctx); err != nil {
			return nil, fmt.Errorf("failed to parse mix presentation obu: %w", err)
		}
		// remove already shown
		conf := *r.ctx
		conf.CodecConfigs = nil
		conf.NumCodecConfigs = 0
		conf.AudioElements = nil
		conf.NumAudioElements = 0
		return &conf, nil
	default:
		// Skip unknown OBU types
	}

	return nil, nil
}

func (o *ObuInfo) Info(writer func(format string, p ...interface{})) error {
	writer("obu: Type=%s Size=%d Start=%d", o.Type, o.Size, o.Start)
	return nil
}

func (i *IamfContext) Info(f func(level int, format string, p ...interface{})) error {
	f(0, "IAMF Context:")
	f(1, "Codec Configs (%d):", i.NumCodecConfigs)
	for _, cc := range i.CodecConfigs[:i.NumCodecConfigs] {
		f(2, "CodecConfigID=%d Codec=%s SampleRate=%d NumSamples=%d RollDistance=%d",
			cc.CodecConfigID, cc.CodecID, cc.SampleRate, cc.NumSamples, cc.AudioRollDistance)
	}

	f(1, "Audio Elements (%d):", i.NumAudioElements)
	for _, ae := range i.AudioElements[:i.NumAudioElements] {
		f(2, "AudioElementID=%d Type=%s CodecConfigID=%d NumSubstreams=%d NumLayers=%d",
			ae.AudioElementID, ae.Element.AudioElementType, ae.CodecConfigID, ae.NumSubstreams, ae.NumLayers)
		for j, layer := range ae.Element.Layers {
			f(3, "Layer[%d]: %s", j, layer.ChannelLayout)
			if layer.AmbisonicsMode != 0 {
				f(4, "AmbisonicsMode: %s", layer.AmbisonicsMode)
			}
			if layer.OutputGain.Num != 0 || layer.OutputGain.Den != 1 {
				f(4, "OutputGain: %d/%d", layer.OutputGain.Num, layer.OutputGain.Den)
			}
		}
		for j, ss := range ae.Substreams {
			f(3, "Substream[%d]: ID=%d", j, ss.AudioSubstreamID)
		}
		if ae.Element.DemixingInfo != nil {
			f(3, "Demixing: ID=%d Rate=%d Duration=%d",
				ae.Element.DemixingInfo.ParameterID,
				ae.Element.DemixingInfo.ParameterRate,
				ae.Element.DemixingInfo.Duration)
		}
		if ae.Element.ReconGainInfo != nil {
			f(3, "ReconGain: ID=%d Rate=%d Duration=%d",
				ae.Element.ReconGainInfo.ParameterID,
				ae.Element.ReconGainInfo.ParameterRate,
				ae.Element.ReconGainInfo.Duration)
		}
	}

	f(1, "Mix Presentations (%d):", i.NumMixPresentations)
	for _, mp := range i.MixPresentations[:i.NumMixPresentations] {
		f(2, "MixPresentationID=%d Labels=%v", mp.MixPresentationID, mp.LanguageLabel)
		for lang, ann := range mp.Mix.Annotations {
			f(3, "%s: %s", lang, ann)
		}
		for j, submix := range mp.Mix.Submixes {
			f(3, "Submix[%d]: NumElements=%d NumLayouts=%d", j, submix.NumElements, submix.NumLayouts)
			for k, elem := range submix.Elements {
				f(4, "Element[%d]: AudioElementID=%d HeadphonesMode=%s",
					k, elem.AudioElementID, elem.HeadphonesRenderingMode)
				f(5, "DefaultMixGain: %d/%d", elem.DefaultMixGain.Num, elem.DefaultMixGain.Den)
				for lang, ann := range elem.Annotations {
					f(6, "%s: %s", lang, ann)
				}
			}
			for k, layout := range submix.Layouts {
				f(4, "Layout[%d]: Type=%s", k, layout.LayoutType)
				if layout.SoundSystem.NumChannels > 0 {
					f(5, "SoundSystem: %s", layout.SoundSystem)
					f(5, "Channels: %d", layout.SoundSystem.NumChannels)
				}
				f(5, "IntegratedLoudness: %d/%d", layout.IntegratedLoudness.Num, layout.IntegratedLoudness.Den)
				f(5, "DigitalPeak: %d/%d", layout.DigitalPeak.Num, layout.DigitalPeak.Den)
				if layout.TruePeak.Num != 0 {
					f(5, "TruePeak: %d/%d", layout.TruePeak.Num, layout.TruePeak.Den)
				}
			}
		}
	}

	f(1, "Param Definitions (%d):", i.NumParamDefinitions)
	for _, pd := range i.ParamDefinitions[:i.NumParamDefinitions] {
		f(2, "Type=%s ParameterID=%d ParameterRate=%d Duration=%d",
			pd.Param.Type, pd.Param.ParameterID, pd.Param.ParameterRate, pd.Param.Duration)
		f(3, "Mode=%d NumSubblocks=%d", pd.Mode, pd.Param.NumSubblocks)
		if pd.AudioElement != nil {
			f(3, "AudioElementID=%d", pd.AudioElement.AudioElementID)
		}
	}

	return nil
}

func (c channelLayout) toChannelLayout() ChannelLayout {
	var channelNames = map[channelMask]string{
		chmFrontLeft:          "FL",
		chmFrontRight:         "FR",
		chmFrontCenter:        "FC",
		chmLowFrequency:       "LFE",
		chmBackLeft:           "BL",
		chmBackRight:          "BR",
		chmFrontLeftOfCenter:  "FLC",
		chmFrontRightOfCenter: "FRC",
		chmBackCenter:         "BC",
		chmSideLeft:           "SL",
		chmSideRight:          "SR",
		chmTopCenter:          "TC",
		chmTopFrontLeft:       "TFL",
		chmTopFrontCenter:     "TFC",
		chmTopFrontRight:      "TFR",
		chmTopBackLeft:        "TBL",
		chmTopBackCenter:      "TBC",
		chmTopBackRight:       "TBR",
		chmLowFrequency2:      "LFE2",
		chmTopSideLeft:        "TSL",
		chmTopSideRight:       "TSR",
		chmBottomFrontCenter:  "BFC",
		chmBottomFrontLeft:    "BFL",
		chmBottomFrontRight:   "BFR",
		chmSideSurroundLeft:   "SSL",
		chmSideSurroundRight:  "SSR",
		chmBinauralLeft:       "BinL",
		chmBinauralRight:      "BinR",
		chmBottomBackLeft:     "BBL",
		chmBottomBackRight:    "BBR",
	}

	var scalableNames = []string{
		"Mono",
		"Stereo (System A - 0+2+0)",
		"5.1ch (System B - 0+5+0)",
		"5.1.2ch (System C - 2+5+0)",
		"5.1.4ch (System D - 4+5+0)",
		"7.1ch (System I - 0+7+0)",
		"7.1.2ch (System I, J front subset)",
		"7.1.4ch (System J - 4+7+0)",
		"3.1.2ch (System J front subset)",
		"Binaural",
	}

	var expandedNames = []string{
		"LFE (System J subset)",
		"Stereo-S (System J surr subset)",
		"Stereo-SS (System I side surr subset)",
		"Stereo-RS (System J rear surr subset)",
		"Stereo-TF (System J top subset)",
		"Stereo-TB (System J back subset)",
		"Top-4ch (System J subset)",
		"3.0ch LCR (System J front subset)",
		"9.1.6ch (System H subset - 9+10+3)",
		"Stereo-F (System H front subset)",
		"Stereo-Si (System H side subset)",
		"Stereo-TpSi (System H top side subset)",
		"Top-6ch (System H subset)",
		"10.2.9.3ch (System H - 9+10+3)",
		"LFE-Pair (System H subset)",
		"Bottom-3ch (System H subset)",
		"7.1.5.4ch (System J top/bottom subset)",
		"Bottom-4ch (System J subset)",
		"Top-1ch (System J subset)",
		"Top-5ch (System J subset)",
	}

	var unmappedNames = map[string]string{
		"E": "5.1.4ch SS_D + BFC (System E)",
		"F": "7.1.2ch SS_I + TpBC + LFE2 (System F)",
		"G": "9.1.4ch (System G)",
	}

	description := ""
	channelMap := make(map[string]string)

	// Identify layout based on Order type
	switch c.Order {
	case coScalable:
		if c.TableIndex >= 0 && c.TableIndex < len(scalableNames) {
			description = scalableNames[c.TableIndex]
		} else {
			description = fmt.Sprintf("Scalable Layout %d", c.TableIndex)
		}
	case coExpanded:
		if c.TableIndex >= 0 && c.TableIndex < len(expandedNames) {
			description = expandedNames[c.TableIndex]
		} else if c.TableIndex == systemE.TableIndex {
			description = unmappedNames["E"]
		} else if c.TableIndex == systemF.TableIndex {
			description = unmappedNames["F"]
		} else if c.TableIndex == systemG.TableIndex {
			description = unmappedNames["G"]
		} else {
			description = fmt.Sprintf("%d-Channel Expanded Layout %d", c.Channels, c.TableIndex)
		}

	case coAmbisonics:
		order := int(math.Floor(math.Sqrt(float64(c.Channels - 1))))
		description = fmt.Sprintf("Ambisonics Order %d (%d channels)", order, c.Channels)

	case coCustom:
		description = fmt.Sprintf("Custom %d-Channel Layout", c.Channels)
	}

	// Build detailed channel map showing which speakers are present
	channelIndex := 0
	for bit := uint(0); bit < 64; bit++ {
		mask := channelMask(1 << bit)
		if c.Mask&mask != 0 {
			if name, exists := channelNames[mask]; exists {
				channelMap[fmt.Sprintf("%d", channelIndex)] = name
				channelIndex++
			}
		}
	}

	// For custom/ambisonics layouts with explicit mapping
	if c.Map != nil {
		channelMap = make(map[string]string)
		for k, v := range *c.Map {
			if c.Order == coAmbisonics {
				// Ambisonics channel naming: ACN (Ambisonic Channel Number)
				channelMap[fmt.Sprintf("%d", k)] = fmt.Sprintf("ACN%d", v)
			} else {
				channelMap[fmt.Sprintf("%d", k)] = fmt.Sprintf("CH%d", v)
			}
		}
	}

	return ChannelLayout{
		NumChannels: uint32(c.Channels),
		Description: description,
		ChannelMask: uint64(c.Mask),
		ChannelMap:  channelMap,
	}
}

func signExtend(v uint16) int32 {
	// Sign extend from 16-bit to 32-bit
	if v&0x8000 != 0 {
		return int32(v) | ^0xFFFF
	}
	return int32(v)
}
