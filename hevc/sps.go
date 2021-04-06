package hevc

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/edgeware/mp4ff/bits"
)

// SPS - HEVC SPS parameters
// ISO/IEC 23008-2 Sec. 7.3.2.2
type SPS struct {
	VpsID                                byte
	MaxSubLayersMinus1                   byte
	TemporalIdNestingFlag                bool
	ProfileTierLevel                     ProfileTierLevel
	SpsID                                byte
	ChromaFormatIDC                      byte
	SeparateColourPlaneFlag              bool
	ConformanceWindowFlag                bool
	PicWidthInLumaSamples                uint32
	PicHeightInLumaSamples               uint32
	ConformanceWindow                    ConformanceWindow
	BitDepthLumaMinus8                   byte
	BitDepthChromaMinus8                 byte
	Log2MaxPicOrderCntLsbMinus4          byte
	SubLayerOrderingInfoPresentFlag      bool
	SubLayeringOrderingInfos             []SubLayerOrderingInfo
	Log2MinLumaCodingBlockSizeMinus3     byte
	Log2DiffMaxMinLumaCodingBlockSize    byte
	Log2MinLumaTransformBlockSizeMinus2  byte
	Log2DiffMaxMinLumaTransformBlockSize byte
	MaxTransformHierarchyDepthInter      byte
	MaxTransformHierarchyDepthIntra      byte
	ScalingListEnabledFlag               bool
	ScalingListDataPresentFlag           bool
	AmpEnabledFlag                       bool
	SampleAdaptiveOffsetEnabledFlag      bool
	PCMEnabledFlag                       bool
	NumShortTermRefPicSets               byte
	LongTermRefPicsPresentFlag           bool
	SpsTemporalMvpEnabledFlag            bool
	StrongIntraSmoothingEnabledFlag      bool
	VUIParametersPresentFlag             bool
}

// ISO/IEC 23008-2 Section 7.3.3
type ProfileTierLevel struct {
	GeneralProfileSpace              byte
	GeneralTierFlag                  bool
	GeneralProfileIDC                byte
	GeneralProfileCompatibilityFlags uint32
	GeneralConstraintIndicatorFlags  uint64 // 48 bits
	GeneralProgressiveSourceFlag     bool
	GeneralInterlacedSourceFlag      bool
	GeneralNonPackedConstraintFlag   bool
	GeneralFrameOnlyConstraintFlag   bool
	// 43 + 1 bits of info
	GeneralLevelIDC byte
	// Sublayer stuff

}

type ConformanceWindow struct {
	LeftOffset   uint32
	RightOffset  uint32
	TopOffset    uint32
	BottomOffset uint32
}

type SubLayerOrderingInfo struct {
	MaxDecPicBufferingMinus1 byte
	MaxNumReorderPics        byte
	MaxLatencyIncreasePlus1  byte
}

// ParseSPSNALUnit - Parse HEVC SPS NAL unit starting with NAL unit header
func ParseSPSNALUnit(data []byte) (*SPS, error) {

	sps := &SPS{}

	rd := bytes.NewReader(data)
	r := bits.NewAccErrEBSPReader(rd)
	// Note! First two bytes are NALU Header

	naluHdrBits := r.Read(16)
	naluType := GetNaluType(byte(naluHdrBits >> 8))
	if naluType != NALU_SPS {
		return nil, fmt.Errorf("NALU type is %s not SPS", naluType)
	}
	sps.VpsID = byte(r.Read(4))
	sps.MaxSubLayersMinus1 = byte(r.Read(3))
	sps.TemporalIdNestingFlag = r.ReadFlag()
	sps.ProfileTierLevel.GeneralProfileSpace = byte(r.Read(2))
	sps.ProfileTierLevel.GeneralTierFlag = r.ReadFlag()
	sps.ProfileTierLevel.GeneralProfileIDC = byte(r.Read(5))
	sps.ProfileTierLevel.GeneralProfileCompatibilityFlags = uint32(r.Read(32))
	sps.ProfileTierLevel.GeneralConstraintIndicatorFlags = uint64(r.Read(48))
	sps.ProfileTierLevel.GeneralLevelIDC = byte(r.Read(8))
	if sps.MaxSubLayersMinus1 != 0 {
		return sps, nil // Cannot parse any further
	}
	sps.SpsID = byte(r.ReadExpGolomb())
	sps.ChromaFormatIDC = byte(r.ReadExpGolomb())
	if sps.ChromaFormatIDC == 3 {
		sps.SeparateColourPlaneFlag = r.ReadFlag()
	}
	sps.PicWidthInLumaSamples = uint32(r.ReadExpGolomb())
	sps.PicHeightInLumaSamples = uint32(r.ReadExpGolomb())
	sps.ConformanceWindowFlag = r.ReadFlag()
	if sps.ConformanceWindowFlag {
		sps.ConformanceWindow = ConformanceWindow{
			LeftOffset:   uint32(r.ReadExpGolomb()),
			RightOffset:  uint32(r.ReadExpGolomb()),
			TopOffset:    uint32(r.ReadExpGolomb()),
			BottomOffset: uint32(r.ReadExpGolomb()),
		}
	}
	sps.BitDepthLumaMinus8 = byte(r.ReadExpGolomb())
	sps.BitDepthChromaMinus8 = byte(r.ReadExpGolomb())
	sps.Log2MaxPicOrderCntLsbMinus4 = byte(r.ReadExpGolomb())
	sps.SubLayerOrderingInfoPresentFlag = r.ReadFlag()
	startValue := byte(0)
	if sps.SubLayerOrderingInfoPresentFlag {
		startValue = sps.MaxSubLayersMinus1
	}
	for i := startValue; i <= sps.MaxSubLayersMinus1; i++ {
		sps.SubLayeringOrderingInfos = append(
			sps.SubLayeringOrderingInfos,
			SubLayerOrderingInfo{
				MaxDecPicBufferingMinus1: byte(r.ReadExpGolomb()),
				MaxNumReorderPics:        byte(r.ReadExpGolomb()),
				MaxLatencyIncreasePlus1:  byte(r.ReadExpGolomb()),
			})
	}
	sps.Log2MinLumaCodingBlockSizeMinus3 = byte(r.ReadExpGolomb())
	sps.Log2DiffMaxMinLumaCodingBlockSize = byte(r.ReadExpGolomb())
	sps.Log2MinLumaTransformBlockSizeMinus2 = byte(r.ReadExpGolomb())
	sps.Log2DiffMaxMinLumaTransformBlockSize = byte(r.ReadExpGolomb())
	sps.MaxTransformHierarchyDepthInter = byte(r.ReadExpGolomb())
	sps.MaxTransformHierarchyDepthIntra = byte(r.ReadExpGolomb())
	sps.ScalingListEnabledFlag = r.ReadFlag()
	if sps.ScalingListEnabledFlag {
		sps.ScalingListDataPresentFlag = r.ReadFlag()
		if sps.ScalingListDataPresentFlag {
			return sps, r.AccError() // Doesn't get any further now
		}
	}
	sps.AmpEnabledFlag = r.ReadFlag()
	sps.SampleAdaptiveOffsetEnabledFlag = r.ReadFlag()
	sps.PCMEnabledFlag = r.ReadFlag()
	if sps.PCMEnabledFlag {
		return sps, r.AccError() // Doesn't get any further now
	}
	sps.NumShortTermRefPicSets = byte(r.ReadExpGolomb())
	if sps.NumShortTermRefPicSets != 0 {
		return sps, r.AccError() // Doesn't get any further for now
	}
	sps.LongTermRefPicsPresentFlag = r.ReadFlag()
	if sps.LongTermRefPicsPresentFlag {
		return sps, r.AccError() // Does't get any further for now
	}
	sps.SpsTemporalMvpEnabledFlag = r.ReadFlag()
	sps.StrongIntraSmoothingEnabledFlag = r.ReadFlag()
	sps.VUIParametersPresentFlag = r.ReadFlag()

	return sps, r.AccError()
}

// ImageSize - calculated width and height using ConformanceWindow
func (s *SPS) ImageSize() (width, height uint32) {
	encWidth, encHeight := s.PicWidthInLumaSamples, s.PicHeightInLumaSamples
	var subWidthC, subHeightC uint32 = 1, 1
	switch s.ChromaFormatIDC {
	case 1: // 4:2:0
		subWidthC, subHeightC = 2, 2
	case 2: // 4:2:2
		subWidthC = 2
	}
	width = encWidth - (s.ConformanceWindow.LeftOffset+s.ConformanceWindow.RightOffset)*subWidthC
	height = encHeight - (s.ConformanceWindow.TopOffset+s.ConformanceWindow.BottomOffset)*subHeightC
	return width, height
}

// CodecString returns string based on SPS fields.
// ISO/IEC 14496-15:2014 Annex E.
func (s *SPS) CodecString() string {

	fields := []string{"hvc1"}
	generalProfileSpace := ""
	switch s.ProfileTierLevel.GeneralProfileSpace {
	case 1:
		generalProfileSpace = "A"
	case 2:
		generalProfileSpace = "B"
	case 3:
		generalProfileSpace = "C"
	}

	fields = append(fields, fmt.Sprintf("%s%d", generalProfileSpace, s.ProfileTierLevel.GeneralProfileIDC))

	profileCompatibilityFlags := reverseBitsAndHexEncode(s.ProfileTierLevel.GeneralProfileCompatibilityFlags)
	fields = append(fields, profileCompatibilityFlags)

	profileGeneralTier := "L"
	if s.ProfileTierLevel.GeneralTierFlag {
		profileGeneralTier = "H"
	}

	fields = append(fields, fmt.Sprintf("%s%d", profileGeneralTier, s.ProfileTierLevel.GeneralLevelIDC))

	constraints := make([]byte, 8)
	binary.BigEndian.PutUint64(constraints, s.ProfileTierLevel.GeneralConstraintIndicatorFlags)
	end := 8
	for {
		if constraints[end-1] != 0 {
			break
		}
		end--
	}
	fields = append(fields, trimLeadingZero(fmt.Sprintf("%x", constraints[0:end])))

	return strings.Join(fields, ".")

}

// reverseBitsAndHexEncode encodes the 32 bits input, but in reverse bit order
// ISO/IEC 23008‐2
func reverseBitsAndHexEncode(x uint32) string {

	x = ((x & 0x55555555) << 1) | ((x & 0xAAAAAAAA) >> 1)
	x = ((x & 0x33333333) << 2) | ((x & 0xCCCCCCCC) >> 2)
	x = ((x & 0x0F0F0F0F) << 4) | ((x & 0xF0F0F0F0) >> 4)

	bs := []byte{
		uint8(x & 0xFF),
		uint8((x >> 8) & 0xFF),
		uint8((x >> 16) & 0xFF),
		uint8((x >> 24) & 0xFF),
	}

	return trimLeadingZero(fmt.Sprintf("%02x", bs))
}

// trimLeadingZero trims leading zero in hex string
func trimLeadingZero(in string) string {
	for i, c := range in {
		if c != '0' {
			return in[i:]
		}
	}
	return "0"
}
