package hevc

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"

	"github.com/edgeware/mp4ff/avc"
	"github.com/edgeware/mp4ff/bits"
)

const (
	// PREFIX_SEI_NUT SEI Message Types
	SEIBufferingPeriodType                       = 0
	SEIPicTimingType                             = 1
	SEIPanScanRectType                           = 2
	SEIFillerPayloadType                         = 3
	SEIUserDataRegisteredITUtT35Type             = 4
	SEIUserDataUnregisteredType                  = 5
	SEIRecoveryPointType                         = 6
	SEISceneInfoType                             = 9
	SEIPictureSnapShotType                       = 15
	SEIProgressiveRefinementSegmentStartType     = 16
	SEIProgressiveRefinementSegmentStartEnd      = 17
	SEIFilmGrainCharacteristicsType              = 19
	SEIPostFilterHintType                        = 22
	SEIToneMappingInfoType                       = 23
	SEIFramePackingArrangementType               = 45
	SEIDisplayOrientationType                    = 47
	SEIGreenMetaDataType                         = 56
	SEIStructureOfPicturesInfoType               = 128
	SEIActiveParameterSetsType                   = 129
	SEIDecodingUnitInfoType                      = 130
	SEITemporalSubLayerZeroIdxType               = 131
	SEIScalableNestingType                       = 133
	SEIRegionRefreshInfoType                     = 134
	SEINoDisplayType                             = 135
	SEITimeCodeType                              = 136
	SEIMasteringDisplayColourVolumeType          = 137
	SEISegmentedRectFramePackingArrangementType  = 138
	SEITemporalMotionConstrainedTileSetsType     = 139
	SEIChromaResamplingFilterHintType            = 140
	SEIKneeFunctionInfoType                      = 141
	SEIColourRemappingInfoType                   = 142
	SEIDeinterlacedFieldIdentificationType       = 143
	SEIContentLightLevelInformationType          = 144
	SEIDependentRapIndicationType                = 145
	SEICodedRegionCompletionType                 = 146
	SEIAlternativeTransferCharacteristicsType    = 147
	SEIAmbientViewingEnvironmentType             = 148
	SEIContentColourVolumeType                   = 149
	SEIEquirectangularProjectionType             = 150
	SEICubemapProjectionType                     = 151
	SEIFisheyeVideoInfoType                      = 152
	SEISphereRotationType                        = 154
	SEIRegionwisePackingType                     = 155
	SEIOmniViewportType                          = 156
	SEIRegionalNestingType                       = 157
	SEIMctsExtractionInfoSetsType                = 158
	SEIMctsExtractionInfoNesting                 = 159
	SEILayersNotPresentType                      = 160
	SEIInterLayerConstrainedTileSetsType         = 161
	SEIBspNestingType                            = 162
	SEIBspInitialArrivalTimeType                 = 163
	SEISubBitstreamPropertyType                  = 164
	SEIAlphaChannelInfoType                      = 165
	SEIOverlayInfoType                           = 166
	SEITemporalMvPredictionConstraintsType       = 167
	SEIFrameFieldInfoType                        = 168
	SEIThreeDimensionalReferenceDisplaysInfoType = 176
	SEIDepthRepresentationInfoType               = 177
	SEIMultiviewSceneInfoType                    = 178
	SEIMultiviewAcquisitionInfoType              = 179
	SEIMultiviewViewPositionType                 = 180
	SEIAlternativeDepthInfoType                  = 181
	SEISeiManifestType                           = 200
	SEISeiPrefixIndicationType                   = 201
	SEIAnnotatedRegionsType                      = 202
	// SUFFIX_SEI_NUT SEI Message Types
	// SEIFillerPayloadType          = 3
	// SEIUserDataRegisteredITUtType = 4
	//SEIUserDataUnregisteredType = 5    same as PREFIX
	//SEIProgressiveRefinementSegmentEndType = 17
	// SEIPostFilterHintType  = 22 same as PREFIX
	SEIDecodedPictureHashType = 132
	//SEICodedRegionCompletionType = 146 same as PREFIX
)

type HEVCSEIType uint

func (h HEVCSEIType) String() string {
	name := ""
	switch h {
	case SEIBufferingPeriodType:
		name = "SEIBufferingPeriodType"
	case SEIPicTimingType:
		name = "SEIPicTimingType"
	case SEIPanScanRectType:
		name = "SEIPanScanRectType"
	case SEIFillerPayloadType:
		name = "SEIFillerPayloadType"
	case SEIUserDataRegisteredITUtT35Type:
		name = "SEIUserDataRegisteredITUtT35Type"
	case SEIUserDataUnregisteredType:
		name = "SEIUserDataUnregisteredType"
	case SEIRecoveryPointType:
		name = "SEIRecoveryPointType"
	case SEISceneInfoType:
		name = "SEISceneInfoType"
	case SEIPictureSnapShotType:
		name = "SEIPictureSnapShotType"
	case SEIProgressiveRefinementSegmentStartType:
		name = "SEIProgressiveRefinementSegmentStartType"
	case SEIProgressiveRefinementSegmentStartEnd:
		name = "SEIProgressiveRefinementSegmentStartEnd"
	case SEIFilmGrainCharacteristicsType:
		name = "SEIFilmGrainCharacteristicsType"
	case SEIPostFilterHintType:
		name = "SEIPostFilterHintType"
	case SEIToneMappingInfoType:
		name = "SEIToneMappingInfoType"
	case SEIFramePackingArrangementType:
		name = "SEIFramePackingArrangementType"
	case SEIDisplayOrientationType:
		name = "SEIDisplayOrientationType"
	case SEIGreenMetaDataType:
		name = "SEIGreenMetaDataType"
	case SEIStructureOfPicturesInfoType:
		name = "SEIStructureOfPicturesInfoType"
	case SEIActiveParameterSetsType:
		name = "SEIActiveParameterSetsType"
	case SEIDecodingUnitInfoType:
		name = "SEIDecodingUnitInfoType"
	case SEITemporalSubLayerZeroIdxType:
		name = "SEITemporalSubLayerZeroIdxType"
	case SEIScalableNestingType:
		name = "SEIScalableNestingType"
	case SEIRegionRefreshInfoType:
		name = "SEIRegionRefreshInfoType"
	case SEINoDisplayType:
		name = "SEINoDisplayType"
	case SEITimeCodeType:
		name = "SEITimeCodeType"
	case SEIMasteringDisplayColourVolumeType:
		name = "SEIMasteringDisplayColourVolumeType"
	case SEISegmentedRectFramePackingArrangementType:
		name = "SEISegmentedRectFramePackingArrangementType"
	case SEITemporalMotionConstrainedTileSetsType:
		name = "SEITemporalMotionConstrainedTileSetsType"
	case SEIChromaResamplingFilterHintType:
		name = "SEIChromaResamplingFilterHintType"
	case SEIKneeFunctionInfoType:
		name = "SEIKneeFunctionInfoType"
	case SEIColourRemappingInfoType:
		name = "SEIColourRemappingInfoType"
	case SEIDeinterlacedFieldIdentificationType:
		name = "SEIDeinterlacedFieldIdentificationType"
	case SEIContentLightLevelInformationType:
		name = "SEIContentLightLevelInformationType"
	case SEIDependentRapIndicationType:
		name = "SEIDependentRapIndicationType"
	case SEICodedRegionCompletionType:
		name = "SEICodedRegionCompletionType"
	case SEIAlternativeTransferCharacteristicsType:
		name = "SEIAlternativeTransferCharacteristicsType"
	case SEIAmbientViewingEnvironmentType:
		name = "SEIAmbientViewingEnvironmentType"
	case SEIContentColourVolumeType:
		name = "SEIContentColourVolumeType"
	case SEIEquirectangularProjectionType:
		name = "SEIEquirectangularProjectionType"
	case SEICubemapProjectionType:
		name = "SEICubemapProjectionType"
	case SEIFisheyeVideoInfoType:
		name = "SEIFisheyeVideoInfoType"
	case SEISphereRotationType:
		name = "SEISphereRotationType"
	case SEIRegionwisePackingType:
		name = "SEIRegionwisePackingType"
	case SEIOmniViewportType:
		name = "SEIOmniViewportType"
	case SEIRegionalNestingType:
		name = "SEIRegionalNestingType"
	case SEIMctsExtractionInfoSetsType:
		name = "SEIMctsExtractionInfoSetsType"
	case SEIMctsExtractionInfoNesting:
		name = "SEIMctsExtractionInfoNesting"
	case SEILayersNotPresentType:
		name = "SEILayersNotPresentType"
	case SEIInterLayerConstrainedTileSetsType:
		name = "SEIInterLayerConstrainedTileSetsType"
	case SEIBspNestingType:
		name = "SEIBspNestingType"
	case SEIBspInitialArrivalTimeType:
		name = "SEIBspInitialArrivalTimeType"
	case SEISubBitstreamPropertyType:
		name = "SEISubBitstreamPropertyType"
	case SEIAlphaChannelInfoType:
		name = "SEIAlphaChannelInfoType"
	case SEIOverlayInfoType:
		name = "SEIOverlayInfoType"
	case SEITemporalMvPredictionConstraintsType:
		name = "SEITemporalMvPredictionConstraintsType"
	case SEIFrameFieldInfoType:
		name = "SEIFrameFieldInfoType"
	case SEIThreeDimensionalReferenceDisplaysInfoType:
		name = "SEIThreeDimensionalReferenceDisplaysInfoType"
	case SEIDepthRepresentationInfoType:
		name = "SEIDepthRepresentationInfoType"
	case SEIMultiviewSceneInfoType:
		name = "SEIMultiviewSceneInfoType"
	case SEIMultiviewAcquisitionInfoType:
		name = "SEIMultiviewAcquisitionInfoType"
	case SEIMultiviewViewPositionType:
		name = "SEIMultiviewViewPositionType"
	case SEIAlternativeDepthInfoType:
		name = "SEIAlternativeDepthInfoType"
	case SEISeiManifestType:
		name = "SEISeiManifestType"
	case SEISeiPrefixIndicationType:
		name = "SEISeiPrefixIndicationType"
	case SEIAnnotatedRegionsType:
		name = "SEIAnnotatedRegionsType"
	case SEIDecodedPictureHashType: // Only in SUFFIX_SEI_NUT
		name = "SEIDecodedPictureHashType"
	default:
		name = "Reserved HEVC SEI type"
	}
	return fmt.Sprintf("%s (%d)", name, h)
}

// DecodeSEIMessage decodes an SEIMessage
func DecodeSEIMessage(sd *avc.SEIData) (avc.SEIMessage, error) {
	switch sd.Type() {
	case SEITimeCodeType:
		return DecodeTimeCodeSEI(sd)
	case SEIMasteringDisplayColourVolumeType:
		return DecodeMasteringDisplayColourVolumeSEI(sd)
	case SEIContentLightLevelInformationType:
		return DecodeContentLightLevelInformationSEI(sd)
	default:
		return DecodeGeneralSEI(sd), nil
	}
}

// MasteringDisplayColourVolumeSEI is HEVC SEI Message 137.
// Defined in ISO/IEC 23008-2 D.2.28
type MasteringDisplayColourVolumeSEI struct {
	DisplayPrimariesX            [3]uint16
	DisplayPrimariesY            [3]uint16
	WhitePointX                  uint16
	WhitePointY                  uint16
	MaxDisplayMasteringLuminance uint32
	MinDisplayMasteringLuminance uint32
}

func (m MasteringDisplayColourVolumeSEI) Type() uint {
	return SEIMasteringDisplayColourVolumeType
}

func (m MasteringDisplayColourVolumeSEI) Size() uint {
	return 24
}

func (m MasteringDisplayColourVolumeSEI) Payload() []byte {
	pl := make([]byte, m.Size())
	pos := 0
	for i := 0; i < 3; i++ {
		binary.BigEndian.PutUint16(pl[pos:pos+2], m.DisplayPrimariesX[i])
		pos += 2
		binary.BigEndian.PutUint16(pl[pos:pos+2], m.DisplayPrimariesY[i])
		pos += 2
	}
	binary.BigEndian.PutUint16(pl[pos:pos+2], m.WhitePointX)
	pos += 2
	binary.BigEndian.PutUint16(pl[pos:pos+2], m.WhitePointY)
	pos += 2
	binary.BigEndian.PutUint32(pl[pos:pos+4], m.MaxDisplayMasteringLuminance)
	pos += 4
	binary.BigEndian.PutUint32(pl[pos:pos+4], m.MinDisplayMasteringLuminance)
	return pl
}

func (m MasteringDisplayColourVolumeSEI) String() string {
	msgType := HEVCSEIType(m.Type()).String()
	return fmt.Sprintf("%s %dB: primaries=(%d, %d) (%d, %d) (%d, %d), whitePoint=(%d, %d), maxLum=%d, minLum=%d",
		msgType, m.Size(),
		m.DisplayPrimariesX[0], m.DisplayPrimariesY[0],
		m.DisplayPrimariesX[1], m.DisplayPrimariesY[1],
		m.DisplayPrimariesX[2], m.DisplayPrimariesY[2],
		m.WhitePointX, m.WhitePointY,
		m.MaxDisplayMasteringLuminance, m.MinDisplayMasteringLuminance)
}

// DecodeUserDataUnregisteredSEI - Decode an unregistered SEI message (type 5)
func DecodeMasteringDisplayColourVolumeSEI(sd *avc.SEIData) (avc.SEIMessage, error) {
	m := MasteringDisplayColourVolumeSEI{}
	data := sd.Payload()
	if len(data) != int(m.Size()) {
		return nil, fmt.Errorf("sei message size mismatch: %d instead of %d", len(data), m.Size())
	}
	pos := 0
	for i := 0; i < 3; i++ {
		m.DisplayPrimariesX[i] = binary.BigEndian.Uint16(data[pos:])
		pos += 2
		m.DisplayPrimariesY[i] = binary.BigEndian.Uint16(data[pos:])
		pos += 2
	}
	m.WhitePointX = binary.BigEndian.Uint16(data[pos:])
	pos += 2
	m.WhitePointY = binary.BigEndian.Uint16(data[pos:])
	pos += 2
	m.MaxDisplayMasteringLuminance = binary.BigEndian.Uint32(data[pos:])
	pos += 4
	m.MinDisplayMasteringLuminance = binary.BigEndian.Uint32(data[pos:])
	return &m, nil
}

// ContentLightLevelInformationSEI is HEVC SEI Message 144.
// Defined in ISO/IEC 23008-2 D.2.35
type ContentLightLevelInformationSEI struct {
	MaxContentLightLevel    uint16
	MaxPicAverageLightLevel uint16
}

func (c ContentLightLevelInformationSEI) Type() uint {
	return SEIContentLightLevelInformationType
}

func (c ContentLightLevelInformationSEI) Size() uint {
	return 4
}

func (c ContentLightLevelInformationSEI) Payload() []byte {
	pl := make([]byte, c.Size())
	binary.BigEndian.PutUint16(pl[:2], c.MaxContentLightLevel)
	binary.BigEndian.PutUint16(pl[2:4], c.MaxPicAverageLightLevel)
	return pl
}

func (c ContentLightLevelInformationSEI) String() string {
	msgType := HEVCSEIType(c.Type()).String()
	return fmt.Sprintf("%s %dB: maxContentLightLevel=%d, maxPicAverageLightLevel=%d",
		msgType, c.Size(), c.MaxContentLightLevel, c.MaxPicAverageLightLevel)
}

// DecodeContentLightLevelInformationSEI decodes HEVC SEI 144.
func DecodeContentLightLevelInformationSEI(sd *avc.SEIData) (avc.SEIMessage, error) {
	c := ContentLightLevelInformationSEI{}
	data := sd.Payload()
	if len(data) != int(c.Size()) {
		return nil, fmt.Errorf("sei message size mismatch: %d instead of %d", len(data), c.Size())
	}
	c.MaxContentLightLevel = binary.BigEndian.Uint16(data[:2])
	c.MaxPicAverageLightLevel = binary.BigEndian.Uint16(data[2:4])
	return &c, nil
}

type TimeCodeSEI struct {
	Clocks []ClockTS
}

type ClockTS struct {
	timeOffsetValue     uint32
	nFrames             uint16
	hours               int16
	minutes             int16
	seconds             int16
	clockTimeStampFlag  bool
	unitsFieldBasedFlag bool
	fullTimeStampFlag   bool
	discontinuityFlag   bool
	cntDroppedFlag      bool
	countingType        byte
	timeOffsetLength    byte
}

func (c ClockTS) Time() string {
	return fmt.Sprintf("%d:%d:%d", c.hours, c.minutes, c.seconds)
}

func DecodeTimeCodeSEI(sd *avc.SEIData) (avc.SEIMessage, error) {
	buf := bytes.NewBuffer(sd.Payload())
	br := bits.NewAccErrReader(buf)
	numClockTS := int(br.Read(2))
	tc := TimeCodeSEI{make([]ClockTS, 0, numClockTS)}
	for i := 0; i < numClockTS; i++ {
		c := ClockTS{hours: -1, minutes: -1, seconds: -1}
		c.clockTimeStampFlag = br.ReadFlag()
		if c.clockTimeStampFlag {
			c.unitsFieldBasedFlag = br.ReadFlag()
			c.countingType = byte(br.Read(5))
			c.fullTimeStampFlag = br.ReadFlag()
			c.discontinuityFlag = br.ReadFlag()
			c.cntDroppedFlag = br.ReadFlag()
			c.nFrames = uint16(br.Read(9))
			if c.fullTimeStampFlag {
				c.seconds = int16(br.Read(6))
				c.minutes = int16(br.Read(6))
				c.hours = int16(br.Read(5))
			} else {
				if br.ReadFlag() {
					c.seconds = int16(br.Read(6))
					if br.ReadFlag() {
						c.minutes = int16(br.Read(6))
						if br.ReadFlag() {
							c.hours = int16(br.Read(5))
						}
					}
				}
			}
			c.timeOffsetLength = byte(br.Read(5))
			if c.timeOffsetLength > 0 {
				c.timeOffsetValue = uint32(br.Read(int(c.timeOffsetLength)))
			}
		}
		tc.Clocks = append(tc.Clocks, c)
	}
	return &tc, br.AccError()
}

// Type - SEI payload type
func (s *TimeCodeSEI) Type() uint {
	return SEITimeCodeType
}

// Payload - SEI raw rbsp payload
func (s *TimeCodeSEI) Payload() []byte {
	sw := bits.NewFixedSliceWriter(int(s.Size()))
	sw.WriteBits(uint(len(s.Clocks)), 2)
	for _, c := range s.Clocks {
		sw.WriteFlag(c.clockTimeStampFlag)
		if c.clockTimeStampFlag {
			sw.WriteFlag(c.unitsFieldBasedFlag)
			sw.WriteBits(uint(c.countingType), 5)
			sw.WriteFlag(c.fullTimeStampFlag)
			sw.WriteFlag(c.discontinuityFlag)
			sw.WriteFlag(c.cntDroppedFlag)
			sw.WriteBits(uint(c.nFrames), 9)
			if c.fullTimeStampFlag {
				sw.WriteBits(uint(c.seconds), 6)
				sw.WriteBits(uint(c.minutes), 6)
				sw.WriteBits(uint(c.hours), 5)
			} else {
				sw.WriteFlag(c.seconds >= 0)
				if c.seconds >= 0 {
					sw.WriteBits(uint(c.seconds), 6)
					sw.WriteFlag(c.minutes >= 0)
					if c.minutes >= 0 {
						sw.WriteBits(uint(c.minutes), 6)
						sw.WriteFlag(c.hours >= 0)
						if c.hours >= 0 {
							sw.WriteBits(uint(c.hours), 5)
						}
					}
				}
			}
			sw.WriteBits(uint(c.timeOffsetLength), 5)
			if c.timeOffsetLength > 0 {
				sw.WriteBits(uint(c.timeOffsetLength), int(c.timeOffsetLength))
			}
		}
	}
	sw.WriteFlag(true) // Final 1 and then byte align
	sw.FlushBits()
	return sw.Bytes()
}

// String returns string representation of TimeCodeSEI.
func (s *TimeCodeSEI) String() string {
	msgType := HEVCSEIType(s.Type())
	msg := fmt.Sprintf("%s, size=%d, time=%s", msgType, s.Size(), s.Clocks[0].Time())
	if len(s.Clocks) > 1 {
		for i := 1; i < len(s.Clocks); i++ {
			msg += fmt.Sprintf(", time=%s", s.Clocks[i].Time())
		}
	}
	return msg
}

// Size - size in bytes of raw SEI message rbsp payload
func (s *TimeCodeSEI) Size() uint {
	nrBits := 2
	for _, c := range s.Clocks {
		nrBits++
		if c.clockTimeStampFlag {
			nrBits += 18
			if c.fullTimeStampFlag {
				nrBits += 17
			} else {
				nrBits++
				if c.seconds >= 0 {
					nrBits += 6 + 1
					if c.minutes >= 0 {
						nrBits += 6 + 1
						if c.hours >= 0 {
							nrBits += 5
						}
					}
				}
			}
			nrBits += 5
			nrBits += int(c.timeOffsetLength)
		}
	}
	return uint((nrBits + 7) / 8)
}

// Type - SEI payload type

func DecodeGeneralSEI(sd *avc.SEIData) avc.SEIMessage {
	return &SEIData{
		sd.Type(),
		sd.Payload(),
	}
}

// SEIData - raw parsed SEI message with rbsp data
type SEIData struct {
	payloadType uint
	payload     []byte
}

// Type - SEI payload type
func (s *SEIData) Type() uint {
	return s.payloadType
}

// Payload - SEI raw rbsp payload
func (s *SEIData) Payload() []byte {
	return s.payload
}

// String - print up to 100 bytes of payload
func (s *SEIData) String() string {
	msgType := HEVCSEIType(s.Type())
	return fmt.Sprintf("%s, size=%d, %q", msgType, s.Size(), hex.EncodeToString(s.payload))
}

// Size - size in bytes of raw SEI message rbsp payload
func (s *SEIData) Size() uint {
	return uint(len(s.payload))
}
