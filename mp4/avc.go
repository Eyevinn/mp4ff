package mp4

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/edgeware/gomp4/bits"
)

const (
	AvcNalSEI    = 6
	AvcNalSPS    = 7
	AvcNalPPS    = 8
	AvcNalAUD    = 9
	EXTENDED_SAR = 255
)

// AvcNalType -
type AvcNalType uint16

func isVideoNalu(b []byte) bool {
	typ := b[0] & 0x1f
	return 1 <= typ && typ <= 5
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
		if nalType == AvcNalSPS {
			hasSPS = true
		}
		if nalType == AvcNalPPS {
			hasPPS = true
		}
		if hasSPS && hasPPS {
			return true
		}
	}
	return false
}

// AvcSPS - AVC SPS paramaeters
type AvcSPS struct {
	Profile                         uint
	ConstraintFlags                 uint
	Level                           uint
	ParameterID                     uint
	ChromaFormatIDC                 uint
	SeparateColourPlaneFlag         uint
	BitDepthLumaMinus8              uint
	BitDepthChromaMinus8            uint
	QPPrimeYZeroTransformBypassFlag uint
	SeqScalingMatrixPresentFlag     uint
	Log2MaxFrameNumMinus4           uint
	PicOrderCntType                 uint
	Log2MaxPicOrderCntLsbMinut4     uint
	DeltaPicOrderAlwaysZeroFlag     uint
	OffsetForNonRefPic              uint
	OffsetForTopToBottomField       uint
	RefFramesInPicOrderCntCycle     []uint
	NumRefFrames                    uint
	GapsInFrameNumValueAllowedFlag  uint
	FrameMbsOnlyFlag                uint
	MbAdaptiveFrameFieldFlag        uint
	Direct8x8InferenceFlag          uint
	FrameCroppingFlag               uint
	FrameCropLeftOffset             uint
	FrameCropRightOffset            uint
	FrameCropTopOffset              uint
	FrameCropBottomOffset           uint
	Width                           uint
	Height                          uint
	SampleAspectRatioWidth          uint
	SampleAspectRatioHeight         uint
}

// ParseSPS - Parse AVC SPS NAL unit
func ParseSPS(data []byte) (*AvcSPS, error) {

	sps := &AvcSPS{}

	rd := bytes.NewReader(data)
	reader := bits.NewEBSPReader(rd)

	sps.Profile = reader.MustRead(8)
	sps.ConstraintFlags = reader.MustRead(4)
	_ = reader.MustRead(4) // Reserved bits
	sps.Level = reader.MustRead(8)

	sps.ParameterID = ue(reader)

	sps.ChromaFormatIDC = 1 // Default value if value not present

	if sps.Profile == 138 {
		sps.ChromaFormatIDC = 0
	}

	// The following table is from 14496-10:2014 Section 7.3.2.1.1
	switch sps.Profile {
	case 100, 110, 122, 244, 44, 83, 86, 118, 128, 138, 139, 134:
		sps.ChromaFormatIDC = ue(reader)
		if sps.ChromaFormatIDC == 3 {
			sps.SeparateColourPlaneFlag = reader.MustRead(1)
		}
		sps.BitDepthLumaMinus8 = ue(reader)
		sps.BitDepthChromaMinus8 = ue(reader)
		sps.QPPrimeYZeroTransformBypassFlag = reader.MustRead(1)
		sps.SeqScalingMatrixPresentFlag = reader.MustRead(1)
		if sps.SeqScalingMatrixPresentFlag == 1 {
			return nil, errors.New("Not implemented: Need to handle seq_scaling_matrix_present_flag")
		}
	default:
		// Empty
	}

	sps.Log2MaxFrameNumMinus4 = ue(reader)
	sps.PicOrderCntType = ue(reader)
	if sps.PicOrderCntType == 0 {
		sps.Log2MaxPicOrderCntLsbMinut4 = ue(reader)
	} else if sps.PicOrderCntType == 1 {
		sps.DeltaPicOrderAlwaysZeroFlag = reader.MustRead(1)
		sps.OffsetForNonRefPic = ue(reader)
		sps.OffsetForTopToBottomField = ue(reader)
		numRefFramesInPicOrderCntCycle := ue(reader)
		sps.RefFramesInPicOrderCntCycle = make([]uint, numRefFramesInPicOrderCntCycle,
			numRefFramesInPicOrderCntCycle)
		for i := 0; i < int(numRefFramesInPicOrderCntCycle); i++ {
			sps.RefFramesInPicOrderCntCycle[i] = ue(reader)
		}
	}

	sps.NumRefFrames = ue(reader)
	sps.GapsInFrameNumValueAllowedFlag = reader.MustRead(1)

	picWidthInMbsUnitsMinus1 := ue(reader)
	picHeightInMbsUnitsMinus1 := ue(reader)

	sps.Width = (picWidthInMbsUnitsMinus1 + 1) * 16
	sps.Height = (picHeightInMbsUnitsMinus1 + 1) * 16

	sps.FrameMbsOnlyFlag = reader.MustRead(1)
	if sps.FrameMbsOnlyFlag == 0 {
		sps.MbAdaptiveFrameFieldFlag = reader.MustRead(1)
	}
	sps.Direct8x8InferenceFlag = reader.MustRead(1)
	sps.FrameCroppingFlag = reader.MustRead(1)
	var cropUnitX, cropUnitY uint
	if sps.FrameCroppingFlag == 1 {
		switch sps.ChromaFormatIDC {
		case 0:
			cropUnitX, cropUnitY = 1, 2-sps.FrameMbsOnlyFlag
		case 1:
			cropUnitX, cropUnitY = 2, 2*(2-sps.FrameMbsOnlyFlag)
		case 2:
			cropUnitX, cropUnitY = 2, 1*(2-sps.FrameMbsOnlyFlag)
		case 3: //This lacks one extra check?
			cropUnitX, cropUnitY = 1, 1*(2-sps.FrameMbsOnlyFlag)
		default:
			panic("Non-vaild chroma_format_idc value")
		}

		sps.FrameCropLeftOffset = ue(reader)
		sps.FrameCropRightOffset = ue(reader)
		sps.FrameCropTopOffset = ue(reader)
		sps.FrameCropBottomOffset = ue(reader)

		frameCropWidth := sps.FrameCropLeftOffset + sps.FrameCropRightOffset
		frameCropHeight := sps.FrameCropTopOffset + sps.FrameCropBottomOffset

		sps.Width -= frameCropWidth * cropUnitX
		sps.Height -= frameCropHeight * cropUnitY
	}

	vuiParametersPresentFlag := reader.MustRead(1)
	if vuiParametersPresentFlag == 1 {
		aspectRatioPresentFlag := reader.MustRead(1)
		if aspectRatioPresentFlag == 1 {
			aspectRatioIDC := reader.MustRead(8)
			if aspectRatioIDC == EXTENDED_SAR {
				sps.SampleAspectRatioWidth = reader.MustRead(16)
				sps.SampleAspectRatioHeight = reader.MustRead(16)
			} else {
				sps.SampleAspectRatioWidth, sps.SampleAspectRatioHeight = getSAR(aspectRatioIDC)
			}
		}
	}

	return sps, nil
}

// ue - Read one unsigned exponential golomb code using a bitreader
func ue(reader *bits.EBSPReader) uint {
	leadingZeroBits := 0

	for {
		b := reader.MustRead(1)
		if b == 1 {
			break
		}
		leadingZeroBits++
	}

	var res uint = (1 << leadingZeroBits) - 1
	endBits := reader.MustRead(leadingZeroBits)

	return res + endBits
}

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
