package avc

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/edgeware/mp4ff/bits"
)

// extendedSAR - Extended Sample Aspect Ratio Code
const extendedSAR = 255

var ErrNotSPS = errors.New("Not an SPS NAL unit")

// SPS - AVC SPS parameters
type SPS struct {
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
	SeqScalingLists                 []ScalingList
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
	VUI                             *VUIParameters
}

type ScalingList []int // 4x4 or 8x8 Scaling lists. Nil if not present

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
func ParseSPSNALUnit(data []byte, parseVUIBeyondAspectRatio bool) (*SPS, error) {

	sps := &SPS{}

	rd := bytes.NewReader(data)
	reader := bits.NewEBSPReader(rd)
	// Note! First byte is NAL Header

	nalHdr, err := reader.Read(8)
	if err != nil {
		return nil, err
	}
	nalType := GetNaluType(byte(nalHdr))
	if nalType != NALU_SPS {
		return nil, ErrNotSPS
	}

	sps.Profile, err = reader.Read(8)
	if err != nil {
		return nil, err
	}
	sps.ProfileCompatibility, err = reader.Read(8)
	if err != nil {
		return nil, err
	}
	sps.Level, err = reader.Read(8)
	if err != nil {
		return nil, err
	}
	sps.ParameterID, err = reader.ReadExpGolomb()
	if err != nil {
		return nil, err
	}
	sps.ChromaFormatIDC = 1 // Default value if no explicit value present

	if sps.Profile == 138 {
		sps.ChromaFormatIDC = 0
	}

	// The following table is from 14496-10:2014 Section 7.3.2.1.1
	switch sps.Profile {
	case 100, 110, 122, 244, 44, 83, 86, 118, 128, 138, 139, 134:
		sps.ChromaFormatIDC, err = reader.ReadExpGolomb()
		if err != nil {
			return nil, err
		}
		if sps.ChromaFormatIDC == 3 {
			sps.SeparateColourPlaneFlag, err = reader.ReadFlag()
			if err != nil {
				return nil, err
			}
		}
		sps.BitDepthLumaMinus8, err = reader.ReadExpGolomb()
		if err != nil {
			return nil, err
		}
		sps.BitDepthChromaMinus8, err = reader.ReadExpGolomb()
		if err != nil {
			return nil, err
		}
		sps.QPPrimeYZeroTransformBypassFlag, err = reader.ReadFlag()
		if err != nil {
			return nil, err
		}
		sps.SeqScalingMatrixPresentFlag, err = reader.ReadFlag()
		if err != nil {
			return nil, err
		}
		if sps.SeqScalingMatrixPresentFlag {
			nrScalingLists := 12
			if sps.ChromaFormatIDC != 3 {
				nrScalingLists = 8
			}
			sps.SeqScalingLists = make([]ScalingList, nrScalingLists)

			for i := 0; i < nrScalingLists; i++ {
				seqScalingPresent, err := reader.ReadFlag()
				if err != nil {
					return nil, err
				}
				if !seqScalingPresent {
					sps.SeqScalingLists[i] = nil
					continue
				}
				sizeOfScalingList := 16 // 4x4 for i < 6
				if i >= 6 {
					sizeOfScalingList = 64 // 8x8 for i >= 6
				}
				sps.SeqScalingLists[i], err = readScalingList(reader, sizeOfScalingList)
				if err != nil {
					return nil, err
				}
			}
		}
	default:
		// Empty
	}

	sps.Log2MaxFrameNumMinus4, err = reader.ReadExpGolomb()
	if err != nil {
		return nil, err
	}
	sps.PicOrderCntType, err = reader.ReadExpGolomb()
	if err != nil {
		return nil, err
	}
	if sps.PicOrderCntType == 0 {
		sps.Log2MaxPicOrderCntLsbMinus4, err = reader.ReadExpGolomb()
		if err != nil {
			return nil, err
		}
	} else if sps.PicOrderCntType == 1 {
		sps.DeltaPicOrderAlwaysZeroFlag, err = reader.ReadFlag()
		if err != nil {
			return nil, err
		}
		sps.OffsetForNonRefPic, err = reader.ReadExpGolomb()
		if err != nil {
			return nil, err
		}
		sps.OffsetForTopToBottomField, err = reader.ReadExpGolomb()
		if err != nil {
			return nil, err
		}
		numRefFramesInPicOrderCntCycle, err := reader.ReadExpGolomb()
		if err != nil {
			return nil, err
		}
		sps.RefFramesInPicOrderCntCycle = make([]uint, numRefFramesInPicOrderCntCycle)
		for i := 0; i < int(numRefFramesInPicOrderCntCycle); i++ {
			sps.RefFramesInPicOrderCntCycle[i], err = reader.ReadExpGolomb()
			if err != nil {
				return nil, err
			}
		}
	}

	sps.NumRefFrames, err = reader.ReadExpGolomb()
	if err != nil {
		return nil, err
	}
	sps.GapsInFrameNumValueAllowedFlag, err = reader.ReadFlag()
	if err != nil {
		return nil, err
	}

	picWidthInMbsUnitsMinus1, err := reader.ReadExpGolomb()
	if err != nil {
		return nil, err
	}
	picHeightInMbsUnitsMinus1, err := reader.ReadExpGolomb()
	if err != nil {
		return nil, err
	}

	sps.Width = (picWidthInMbsUnitsMinus1 + 1) * 16
	sps.Height = (picHeightInMbsUnitsMinus1 + 1) * 16

	sps.FrameMbsOnlyFlag, err = reader.ReadFlag()
	if err != nil {
		return nil, err
	}
	if !sps.FrameMbsOnlyFlag {
		sps.MbAdaptiveFrameFieldFlag, err = reader.ReadFlag()
		if err != nil {
			return nil, err
		}
	}
	sps.Direct8x8InferenceFlag, err = reader.ReadFlag()
	if err != nil {
		return nil, err
	}
	sps.FrameCroppingFlag, err = reader.ReadFlag()
	if err != nil {
		return nil, err
	}
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
			return nil, fmt.Errorf("Non-vaild chroma_format_idc value: %d", sps.ChromaFormatIDC)
		}

		sps.FrameCropLeftOffset, err = reader.ReadExpGolomb()
		if err != nil {
			return nil, err
		}
		sps.FrameCropRightOffset, err = reader.ReadExpGolomb()
		if err != nil {
			return nil, err
		}
		sps.FrameCropTopOffset, err = reader.ReadExpGolomb()
		if err != nil {
			return nil, err
		}
		sps.FrameCropBottomOffset, err = reader.ReadExpGolomb()
		if err != nil {
			return nil, err
		}

		frameCropWidth := sps.FrameCropLeftOffset + sps.FrameCropRightOffset
		frameCropHeight := sps.FrameCropTopOffset + sps.FrameCropBottomOffset

		sps.Width -= frameCropWidth * cropUnitX
		sps.Height -= frameCropHeight * cropUnitY
	}

	vuiParametersPresentFlag, err := reader.ReadFlag()
	if err != nil {
		return nil, err
	}
	sps.NrBytesBeforeVUI = reader.NrBytesRead()
	if vuiParametersPresentFlag {
		vui, err := parseVUI(reader, parseVUIBeyondAspectRatio)
		if err != nil {
			return nil, fmt.Errorf("parse VUI: %w", err)
		}
		sps.VUI = vui
	}
	sps.NrBytesRead = reader.NrBytesRead()

	return sps, nil
}

// parseVUI - parse VUI (Visual Usability Information)
// if parseVUIBeyondAspectRatio is false, stop after AspectRatio has been parsed
func parseVUI(reader *bits.EBSPReader, parseVUIBeyondAspectRatio bool) (*VUIParameters, error) {
	vui := &VUIParameters{}
	var err error
	aspectRatioInfoPresentFlag, err := reader.ReadFlag()
	if err != nil {
		return nil, err
	}
	if aspectRatioInfoPresentFlag {
		aspectRatioIDC, err := reader.Read(8)
		if err != nil {
			return nil, err
		}
		if aspectRatioIDC == extendedSAR {
			vui.SampleAspectRatioWidth, err = reader.Read(16)
			if err != nil {
				return nil, err
			}
			vui.SampleAspectRatioHeight, err = reader.Read(16)
			if err != nil {
				return nil, err
			}
		} else {
			vui.SampleAspectRatioWidth, vui.SampleAspectRatioHeight, err = getSAR(aspectRatioIDC)
			if err != nil {
				return nil, err
			}
		}
	}
	if !parseVUIBeyondAspectRatio {
		return vui, nil
	}
	vui.OverscanInfoPresentFlag, err = reader.ReadFlag()
	if err != nil {
		return nil, err
	}
	if vui.OverscanInfoPresentFlag {
		vui.OverscanAppropriateFlag, err = reader.ReadFlag()
		if err != nil {
			return nil, err
		}
	}
	vui.VideoSignalTypePresentFlag, err = reader.ReadFlag()
	if err != nil {
		return nil, err
	}
	if vui.VideoSignalTypePresentFlag {
		vui.VideoFormat, err = reader.Read(3)
		if err != nil {
			return nil, err
		}
		vui.VideoFullRangeFlag, err = reader.ReadFlag()
		if err != nil {
			return nil, err
		}
		vui.ColourDescriptionFlag, err = reader.ReadFlag()
		if err != nil {
			return nil, err
		}
		if vui.ColourDescriptionFlag {
			vui.ColourPrimaries, err = reader.Read(8)
			if err != nil {
				return nil, err
			}
			vui.TransferCharacteristics, err = reader.Read(8)
			if err != nil {
				return nil, err
			}
			vui.MatrixCoefficients, err = reader.Read(8)
			if err != nil {
				return nil, err
			}
		}
	}
	vui.ChromaLocInfoPresentFlag, err = reader.ReadFlag()
	if err != nil {
		return nil, err
	}
	if vui.ChromaLocInfoPresentFlag {
		vui.ChromaSampleLocTypeTopField, err = reader.ReadExpGolomb()
		if err != nil {
			return nil, err
		}
		vui.ChromaSampleLocTypeBottomField, err = reader.ReadExpGolomb()
		if err != nil {
			return nil, err
		}
	}
	vui.TimingInfoPresentFlag, err = reader.ReadFlag()
	if err != nil {
		return nil, err
	}
	if vui.TimingInfoPresentFlag {
		vui.NumUnitsInTick, err = reader.Read(32)
		if err != nil {
			return nil, err
		}
		vui.TimeScale, err = reader.Read(32)
		if err != nil {
			return nil, err
		}
		vui.FixedFrameRateFlag, err = reader.ReadFlag()
		if err != nil {
			return nil, err
		}
	}
	vui.NalHrdParametersPresentFlag, err = reader.ReadFlag()
	if err != nil {
		return nil, err
	}
	if vui.NalHrdParametersPresentFlag {
		hrdParams, err := parseHrdParameters(reader)
		if err != nil {
			return nil, fmt.Errorf("parse NalHrdParameters: %w", err)
		}
		vui.NalHrdParameters = hrdParams
	}
	vui.VclHrdParametersPresentFlag, err = reader.ReadFlag()
	if err != nil {
		return nil, err
	}
	if vui.VclHrdParametersPresentFlag {
		hrdParams, err := parseHrdParameters(reader)
		if err != nil {
			return nil, fmt.Errorf("parse VclHrdParameters: %w", err)
		}
		vui.VclHrdParameters = hrdParams
	}
	if vui.NalHrdParametersPresentFlag || vui.VclHrdParametersPresentFlag {
		vui.LowDelayHrdFlag, err = reader.ReadFlag()
		if err != nil {
			return nil, err
		}
	}
	vui.PicStructPresentFlag, err = reader.ReadFlag()
	if err != nil {
		return nil, err
	}
	vui.BitstreamRestrictionFlag, err = reader.ReadFlag()
	if err != nil {
		return nil, err
	}
	if vui.BitstreamRestrictionFlag {
		vui.MotionVectorsOverPicBoundariesFlag, err = reader.ReadFlag()
		if err != nil {
			return nil, err
		}
		vui.MaxBytesPerPicDenom, err = reader.ReadExpGolomb()
		if err != nil {
			return nil, err
		}
		vui.MaxBitsPerMbDenom, err = reader.ReadExpGolomb()
		if err != nil {
			return nil, err
		}
		vui.Log2MaxMvLengthHorizontal, err = reader.ReadExpGolomb()
		if err != nil {
			return nil, err
		}
		vui.Log2MaxMvLengthVertical, err = reader.ReadExpGolomb()
		if err != nil {
			return nil, err
		}
		vui.MaxNumReorderFrames, err = reader.ReadExpGolomb()
		if err != nil {
			return nil, err
		}
		vui.MaxDecFrameBuffering, err = reader.ReadExpGolomb()
		if err != nil {
			return nil, err
		}
	}

	return vui, nil
}

func parseHrdParameters(r *bits.EBSPReader) (*HrdParameters, error) {
	hp := &HrdParameters{}
	var err error
	hp.CpbCountMinus1, err = r.ReadExpGolomb()
	if err != nil {
		return nil, err
	}

	hp.BitRateScale, err = r.Read(4)
	if err != nil {
		return nil, err
	}
	hp.CpbSizeScale, err = r.Read(4)
	if err != nil {
		return nil, err
	}
	for schedSelIdx := uint(0); schedSelIdx <= hp.CpbCountMinus1; schedSelIdx++ {
		ce := CpbEntry{}
		ce.BitRateValueMinus1, err = r.ReadExpGolomb()
		if err != nil {
			return nil, err
		}
		ce.CpbSizeValueMinus1, err = r.ReadExpGolomb()
		if err != nil {
			return nil, err
		}
		ce.CbrFlag, err = r.ReadFlag()
		if err != nil {
			return nil, err
		}
		hp.CpbEntries = append(hp.CpbEntries, ce)
	}
	hp.InitialCpbRemovalDelayLengthMinus1, err = r.Read(5)
	if err != nil {
		return nil, err
	}
	hp.CpbRemovalDelayLengthMinus1, err = r.Read(5)
	if err != nil {
		return nil, err
	}
	hp.DpbOutpuDelayLengthMinus1, err = r.Read(5)
	if err != nil {
		return nil, err
	}
	hp.TimeOffsetLength, err = r.Read(5)
	if err != nil {
		return nil, err
	}
	return hp, err
}

// ConstraintFlags - return the four ConstraintFlag bits
func (a *SPS) ConstraintFlags() byte {
	return byte(a.ProfileCompatibility >> 4)
}

// getSAR - get Sample Aspect Ratio
func getSAR(index uint) (uint, uint, error) {
	if index < 1 || index > 16 {
		return 0, 0, fmt.Errorf("SAR bad index %d", index)
	}
	aspectRatioTable := [][]uint{
		{1, 1}, {12, 11}, {10, 11}, {16, 11},
		{40, 33}, {24, 11}, {20, 11}, {32, 11},
		{80, 33}, {18, 11}, {15, 11}, {64, 33},
		{160, 99}, {4, 3}, {3, 2}, {2, 1}}
	return aspectRatioTable[index-1][0], aspectRatioTable[index-1][1], nil
}

func readScalingList(reader *bits.EBSPReader, sizeOfScalingList int) (ScalingList, error) {
	scalingList := make([]int, sizeOfScalingList)
	lastScale := 8
	nextScale := 8
	for j := 0; j < sizeOfScalingList; j++ {
		if nextScale != 0 {
			deltaScale, err := reader.ReadSignedGolomb()
			if err != nil {
				return nil, err
			}
			nextScale = (lastScale + deltaScale + 256) % 256
		}
		if nextScale == 0 {
			scalingList[j] = lastScale
		} else {
			scalingList[j] = nextScale
		}
		lastScale = scalingList[j]
	}
	return scalingList, nil
}
