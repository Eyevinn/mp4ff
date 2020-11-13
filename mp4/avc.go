package mp4

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/edgeware/mp4ff/bits"
)

var ErrNotSPS = errors.New("Not an SPS NAL unit")

const (
	// NALU_SEI - Supplementary Enhancement Information NAL Unit
	NALU_SEI = 6
	// NALU_SSP - SequenceParameterSet NAL Unit
	NALU_SPS = 7
	// NALU_PPS - PictureParameterSet NAL Unit
	NALU_PPS = 8
	// NALU_AUD - AccessUnitDelimiter NAL Unit
	NALU_AUD = 9
	// NALU_FILL - Filler NAL Unit
	NALU_FILL = 12
	// ExtendedSAR - Extended Sample Aspect Ratio Code
	ExtendedSAR = 255
)

// AvcNalType -
type AvcNalType uint16

func (a AvcNalType) String() string {
	switch a {
	case NALU_SEI:
		return "SEI"
	case NALU_SPS:
		return "SPS"
	case NALU_PPS:
		return "PPS"
	case NALU_AUD:
		return "AUD"
	default:
		return "other"
	}
}

// Get NalType from NAL Header byte
func GetNalType(nalHeader byte) AvcNalType {
	return AvcNalType(nalHeader & 0x1f)
}

// FindAvcNalTypes - find list of nal types
func FindAvcNalTypes(b []byte) []AvcNalType {
	var pos uint32 = 0
	nalList := make([]AvcNalType, 0)
	length := len(b)
	if length < 4 {
		return nalList
	}
	for pos < uint32(length-4) {
		nalLength := binary.BigEndian.Uint32(b[pos : pos+4])
		pos += 4
		nalType := AvcNalType(b[pos] & 0x1f)
		nalList = append(nalList, nalType)
		pos += nalLength
	}
	return nalList
}

// HasAvcParameterSets - Check if H.264 SPS and PPS are present
func HasAvcParameterSets(b []byte) bool {
	nalTypeList := FindAvcNalTypes(b)
	hasSPS := false
	hasPPS := false
	for _, nalType := range nalTypeList {
		if nalType == NALU_SPS {
			hasSPS = true
		}
		if nalType == NALU_PPS {
			hasPPS = true
		}
		if hasSPS && hasPPS {
			return true
		}
	}
	return false
}

// AvcSPS - AVC SPS parameters
type AvcSPS struct {
	Profile                         uint
	ProfileCompatibility            uint
	Level                           uint
	ParameterID                     uint
	ChromaFormatIDC                 uint
	SeparateColourPlaneFlag         bool
	BitDepthLumaMinus8              uint
	BitDepthChromaMinus8            uint
	QPPrimeYZeroTransformBypassFlag bool
	SeqScalingMatrixPresentFlag     bool
	SeqScalings                     *SeqScalings
	Log2MaxFrameNumMinus4           uint
	PicOrderCntType                 uint
	Log2MaxPicOrderCntLsbMinus4     uint
	DeltaPicOrderAlwaysZeroFlag     bool
	OffsetForNonRefPic              uint
	OffsetForTopToBottomField       uint
	RefFramesInPicOrderCntCycle     []uint
	NumRefFrames                    uint
	GapsInFrameNumValueAllowedFlag  bool
	FrameMbsOnlyFlag                bool
	MbAdaptiveFrameFieldFlag        bool
	Direct8x8InferenceFlag          bool
	FrameCroppingFlag               bool
	FrameCropLeftOffset             uint
	FrameCropRightOffset            uint
	FrameCropTopOffset              uint
	FrameCropBottomOffset           uint
	Width                           uint
	Height                          uint
	NrBytesBeforeVUI                int
	NrBytesRead                     int
	VUI                             VUIParameters
}

type SeqScalings struct {
	SeqScalingLists []SeqScalingList
}
type SeqScalingList struct {
	SeqScalingListPresentFlag bool
	ScalingLists              []int
}

// VUIParameters - extra parameters according to 14496-10, E.1
type VUIParameters struct {
	SampleAspectRatioWidth             uint
	SampleAspectRatioHeight            uint
	OverscanInfoPresentFlag            bool
	OverscanAppropriateFlag            bool
	VideoSignalTypePresentFlag         bool
	VideoFormat                        uint
	VideoFullRangeFlag                 bool
	ColourDescriptionFlag              bool
	ColourPrimaries                    uint
	TransferCharacteristics            uint
	MatrixCoefficients                 uint
	ChromaLocInfoPresentFlag           bool
	ChromaSampleLocTypeTopField        uint
	ChromaSampleLocTypeBottomField     uint
	TimingInfoPresentFlag              bool
	NumUnitsInTick                     uint
	TimeScale                          uint
	FixedFrameRateFlag                 bool
	NalHrdParametersPresentFlag        bool
	NalHrdParameters                   *HrdParameters
	VclHrdParametersPresentFlag        bool
	VclHrdParameters                   *HrdParameters
	LowDelayHrdFlag                    bool // Only present with HrdParameters
	PicStructPresentFlag               bool
	BitstreamRestrictionFlag           bool
	MotionVectorsOverPicBoundariesFlag bool
	MaxBytesPerPicDenom                uint
	MaxBitsPerMbDenom                  uint
	Log2MaxMvLengthHorizontal          uint
	Log2MaxMvLengthVertical            uint
	MaxNumReorderFrames                uint
	MaxDecFrameBuffering               uint
}

// HrdParameters inside VUI
type HrdParameters struct {
	CpbCountMinus1                     uint
	BitRateScale                       uint
	CpbSizeScale                       uint
	CpbEntries                         []CpbEntry
	InitialCpbRemovalDelayLengthMinus1 uint
	CpbRemovalDelayLengthMinus1        uint
	DpbOutpuDelayLengthMinus1          uint
	TimeOffsetLength                   uint
}

// CpbEntry inside HrdParameters
type CpbEntry struct {
	BitRateValueMinus1 uint
	CpbSizeValueMinus1 uint
	CbrFlag            bool
}

// ParseSPSNALUnit - Parse AVC SPS NAL unit starting with NAL header
func ParseSPSNALUnit(data []byte, parseVUIBeyondAspectRatio bool) (*AvcSPS, error) {

	sps := &AvcSPS{}

	rd := bytes.NewReader(data)
	reader := bits.NewEBSPReader(rd)
	// Note! First byte is NAL Header

	nalHdr, err := reader.Read(8)
	if err != nil {
		return nil, err
	}
	nalType := nalHdr & 0x1f
	if nalType != NALU_SPS {
		return nil, ErrNotSPS
	}

	sps.Profile = reader.MustRead(8)
	sps.ProfileCompatibility = reader.MustRead(8)
	sps.Level = reader.MustRead(8)

	sps.ParameterID = reader.MustReadExpGolomb()

	sps.ChromaFormatIDC = 1 // Default value if no explicit value present

	if sps.Profile == 138 {
		sps.ChromaFormatIDC = 0
	}

	// The following table is from 14496-10:2014 Section 7.3.2.1.1
	switch sps.Profile {
	case 100, 110, 122, 244, 44, 83, 86, 118, 128, 138, 139, 134:
		sps.ChromaFormatIDC = reader.MustReadExpGolomb()
		if sps.ChromaFormatIDC == 3 {
			sps.SeparateColourPlaneFlag = reader.MustReadFlag()
		}
		sps.BitDepthLumaMinus8 = reader.MustReadExpGolomb()
		sps.BitDepthChromaMinus8 = reader.MustReadExpGolomb()
		sps.QPPrimeYZeroTransformBypassFlag = reader.MustReadFlag()
		sps.SeqScalingMatrixPresentFlag = reader.MustReadFlag()
		if sps.SeqScalingMatrixPresentFlag {
			sps.SeqScalings = &SeqScalings{}
			length := 12 // Default
			if sps.ChromaFormatIDC == 3 {
				length = 8
			}
			for i := 0; i < length; i++ {
				if i < 6 {
					sm := make([]int, 16) // 4x4 scaling matrix
					lastScale := 8
					nextScale := 8
					for j := 0; j < 16; j++ {
						if nextScale != 0 {
							deltaScale := reader.MustReadSignedGolomb()
							nextScale = (lastScale + deltaScale + 256) % 256
						}
						if nextScale == 0 {
							sm[j] = lastScale
						} else {
							sm[j] = nextScale
						}
						lastScale = sm[j]
					}
				} else {
					sm := make([]int, 64) // 8x8 scaling matrix
					lastScale := 8
					nextScale := 8
					for j := 0; j < 64; j++ {
						if nextScale != 0 {
							deltaScale := reader.MustReadSignedGolomb()
							nextScale = (lastScale + deltaScale + 256) % 256
						}
						if nextScale == 0 {
							sm[j] = lastScale
						} else {
							sm[j] = nextScale
						}
						lastScale = sm[j]
					}
				}
			}
		}
	default:
		// Empty
	}

	sps.Log2MaxFrameNumMinus4 = reader.MustReadExpGolomb()
	sps.PicOrderCntType = reader.MustReadExpGolomb()
	if sps.PicOrderCntType == 0 {
		sps.Log2MaxPicOrderCntLsbMinus4 = reader.MustReadExpGolomb()
	} else if sps.PicOrderCntType == 1 {
		sps.DeltaPicOrderAlwaysZeroFlag = reader.MustReadFlag()
		sps.OffsetForNonRefPic = reader.MustReadExpGolomb()
		sps.OffsetForTopToBottomField = reader.MustReadExpGolomb()
		numRefFramesInPicOrderCntCycle := reader.MustReadExpGolomb()
		sps.RefFramesInPicOrderCntCycle = make([]uint, numRefFramesInPicOrderCntCycle)
		for i := 0; i < int(numRefFramesInPicOrderCntCycle); i++ {
			sps.RefFramesInPicOrderCntCycle[i] = reader.MustReadExpGolomb()
		}
	}

	sps.NumRefFrames = reader.MustReadExpGolomb()
	sps.GapsInFrameNumValueAllowedFlag = reader.MustReadFlag()

	picWidthInMbsUnitsMinus1 := reader.MustReadExpGolomb()
	picHeightInMbsUnitsMinus1 := reader.MustReadExpGolomb()

	sps.Width = (picWidthInMbsUnitsMinus1 + 1) * 16
	sps.Height = (picHeightInMbsUnitsMinus1 + 1) * 16

	sps.FrameMbsOnlyFlag = reader.MustReadFlag()
	if !sps.FrameMbsOnlyFlag {
		sps.MbAdaptiveFrameFieldFlag = reader.MustReadFlag()
	}
	sps.Direct8x8InferenceFlag = reader.MustReadFlag()
	sps.FrameCroppingFlag = reader.MustReadFlag()
	var cropUnitX, cropUnitY uint
	var frameMbsOnly uint = 0
	if sps.FrameMbsOnlyFlag {
		frameMbsOnly = 1
	}
	if sps.FrameCroppingFlag {
		switch sps.ChromaFormatIDC {
		case 0:
			cropUnitX, cropUnitY = 1, 2-frameMbsOnly
		case 1:
			cropUnitX, cropUnitY = 2, 2*(2-frameMbsOnly)
		case 2:
			cropUnitX, cropUnitY = 2, 1*(2-frameMbsOnly)
		case 3: //This lacks one extra check?
			cropUnitX, cropUnitY = 1, 1*(2-frameMbsOnly)
		default:
			panic("Non-vaild chroma_format_idc value")
		}

		sps.FrameCropLeftOffset = reader.MustReadExpGolomb()
		sps.FrameCropRightOffset = reader.MustReadExpGolomb()
		sps.FrameCropTopOffset = reader.MustReadExpGolomb()
		sps.FrameCropBottomOffset = reader.MustReadExpGolomb()

		frameCropWidth := sps.FrameCropLeftOffset + sps.FrameCropRightOffset
		frameCropHeight := sps.FrameCropTopOffset + sps.FrameCropBottomOffset

		sps.Width -= frameCropWidth * cropUnitX
		sps.Height -= frameCropHeight * cropUnitY
	}

	vuiParametersPresentFlag := reader.MustReadFlag()
	sps.NrBytesBeforeVUI = reader.NrBytesRead()
	if vuiParametersPresentFlag {
		err := parseVUI(reader, &sps.VUI, parseVUIBeyondAspectRatio)
		if err != nil {
			return nil, err
		}

	}
	sps.NrBytesRead = reader.NrBytesRead()

	return sps, nil
}

// parseVUI - parse VUI (Visual Usability Information)
// if parseVUIBeyondAspectRatio is false, stop after AspectRatio has been parsed
func parseVUI(reader *bits.EBSPReader, vui *VUIParameters, parseVUIBeyondAspectRatio bool) error {
	aspectRatioInfoPresentFlag := reader.MustReadFlag()
	if aspectRatioInfoPresentFlag {
		aspectRatioIDC := reader.MustRead(8)
		if aspectRatioIDC == ExtendedSAR {
			vui.SampleAspectRatioWidth = reader.MustRead(16)
			vui.SampleAspectRatioHeight = reader.MustRead(16)
		} else {
			vui.SampleAspectRatioWidth, vui.SampleAspectRatioHeight = getSAR(aspectRatioIDC)
		}
	}
	if !parseVUIBeyondAspectRatio {
		return nil
	}
	vui.OverscanInfoPresentFlag = reader.MustReadFlag()
	if vui.OverscanInfoPresentFlag {
		vui.OverscanAppropriateFlag = reader.MustReadFlag()
	}
	vui.VideoSignalTypePresentFlag = reader.MustReadFlag()
	if vui.VideoSignalTypePresentFlag {
		vui.VideoFormat = reader.MustRead(3)
		vui.VideoFullRangeFlag = reader.MustReadFlag()
		vui.ColourDescriptionFlag = reader.MustReadFlag()
		if vui.ColourDescriptionFlag {
			vui.ColourPrimaries = reader.MustRead(8)
			vui.TransferCharacteristics = reader.MustRead(8)
			vui.MatrixCoefficients = reader.MustRead(8)
		}
	}
	vui.ChromaLocInfoPresentFlag = reader.MustReadFlag()
	if vui.ChromaLocInfoPresentFlag {
		vui.ChromaSampleLocTypeTopField = reader.MustReadExpGolomb()
		vui.ChromaSampleLocTypeBottomField = reader.MustReadExpGolomb()
	}
	vui.TimingInfoPresentFlag = reader.MustReadFlag()
	if vui.TimingInfoPresentFlag {
		vui.NumUnitsInTick = reader.MustRead(32)
		vui.TimeScale = reader.MustRead(32)
		vui.FixedFrameRateFlag = reader.MustReadFlag()
	}
	vui.NalHrdParametersPresentFlag = reader.MustReadFlag()
	if vui.NalHrdParametersPresentFlag {
		vui.NalHrdParameters = parseHrdParameters(reader)
	}
	vui.VclHrdParametersPresentFlag = reader.MustReadFlag()
	if vui.VclHrdParametersPresentFlag {
		vui.VclHrdParameters = parseHrdParameters(reader)
	}
	if vui.NalHrdParametersPresentFlag || vui.VclHrdParametersPresentFlag {
		vui.LowDelayHrdFlag = reader.MustReadFlag()
	}
	vui.PicStructPresentFlag = reader.MustReadFlag()
	vui.BitstreamRestrictionFlag = reader.MustReadFlag()
	if vui.BitstreamRestrictionFlag {
		vui.MotionVectorsOverPicBoundariesFlag = reader.MustReadFlag()
		vui.MaxBytesPerPicDenom = reader.MustReadExpGolomb()
		vui.MaxBitsPerMbDenom = reader.MustReadExpGolomb()
		vui.Log2MaxMvLengthHorizontal = reader.MustReadExpGolomb()
		vui.Log2MaxMvLengthVertical = reader.MustReadExpGolomb()
		vui.MaxNumReorderFrames = reader.MustReadExpGolomb()
		vui.MaxDecFrameBuffering = reader.MustReadExpGolomb()
	}

	return nil
}

func parseHrdParameters(r *bits.EBSPReader) *HrdParameters {
	hp := &HrdParameters{}
	hp.CpbCountMinus1 = r.MustReadExpGolomb()
	hp.BitRateScale = r.MustRead(4)
	hp.CpbSizeScale = r.MustRead(4)
	for schedSelIdx := uint(0); schedSelIdx <= hp.CpbCountMinus1; schedSelIdx++ {
		ce := CpbEntry{}
		ce.BitRateValueMinus1 = r.MustReadExpGolomb()
		ce.CpbSizeValueMinus1 = r.MustReadExpGolomb()
		ce.CbrFlag = r.MustReadFlag()
		hp.CpbEntries = append(hp.CpbEntries, ce)
	}
	hp.InitialCpbRemovalDelayLengthMinus1 = r.MustRead(5)
	hp.CpbRemovalDelayLengthMinus1 = r.MustRead(5)
	hp.DpbOutpuDelayLengthMinus1 = r.MustRead(5)
	hp.TimeOffsetLength = r.MustRead(5)
	return hp
}

// ConstraintFlags - return the four ConstraintFlag bits
func (a *AvcSPS) ConstraintFlags() byte {
	return byte(a.ProfileCompatibility >> 4)
}

// getSAR - get Sample Aspect Ratio
func getSAR(index uint) (uint, uint) {
	if index < 1 || index > 16 {
		panic(fmt.Sprintf("Bad index %d to SAR", index))
	}
	aspectRatioTable := [][]uint{
		{1, 1}, {12, 11}, {10, 11}, {16, 11},
		{40, 33}, {24, 11}, {20, 11}, {32, 11},
		{80, 33}, {18, 11}, {15, 11}, {64, 33},
		{160, 99}, {4, 3}, {3, 2}, {2, 1}}
	return aspectRatioTable[index-1][0], aspectRatioTable[index-1][1]
}
