package avc

import (
	"bytes"
	"errors"
	"math"

	"github.com/edgeware/mp4ff/bits"
)

var ErrNoSliceHeader = errors.New("No slice header")
var ErrInvalidSliceType = errors.New("Invalid slice type")
var ErrTooFewBytesToParse = errors.New("Too few bytes to parse symbol")
var ErrParserNotImplemented = errors.New("Parser not implemented")

type SliceType uint

func (s SliceType) String() string {
	switch s {
	case SLICE_I:
		return "I"
	case SLICE_P:
		return "P"
	case SLICE_B:
		return "B"
	default:
		return ""
	}
}

const (
	SLICE_P  = SliceType(0)
	SLICE_B  = SliceType(1)
	SLICE_I  = SliceType(2)
	SLICE_SP = SliceType(3)
	SLICE_SI = SliceType(4)
)

// SliceHeader - AVC Slice header
type SliceHeader struct {
	FirstMbInSlice              uint      // ue(v)
	SliceType                   SliceType // ue(v)
	PicParameterSetID           uint      // ue(v)
	ColourPlaneID               uint      // u(2)
	FrameNum                    uint      // u(v) - uses Log2MaxFrameNumMinus4
	FieldPicFlag                bool      // u(1)
	BottomFieldFlag             bool      // u(1)
	IDRPicID                    uint      // ue(v)
	PicOrderCntLSB              uint      // u(v) - TODO: what does 'v' depend on?
	DeltaPicOrderCntBottom      int       // se(v)
	DeltaPicOrderCnt            [2]int    // se(v)
	RedundantPicCnt             uint      // ue(v)
	DirectSpatialMVPredFlag     bool      // u(1)
	NumRefIdxActiveOverrideFlag bool      // u(1)
	NumRefIdxL0ActiveMinus1     uint      // ue(v)
	NumRefIdxL1ActiveMinus1     uint      // ue(v)
	// Ref Pic List Modification MVC not implmented
	RefPicListModification     *RefPicListModification
	PredWeightTable            *PredWeightTable
	DecRefPicMarking           *DecRefPicMarking
	CabacInitIDC               uint // ue(v)
	SliceQPDelta               int  // se(v)
	SPForSwitchFlag            bool // u(1)
	SliceQSDelta               int  // se(v)
	DisableDeblockingFilterIDC uint // ue(v)
	SliceAlphaC0OffsetDev2     int  // se(v)
	SliceBetaOffsetDev2        int  // se(v)
	SliceGroupChangeCycle      uint // u(v) - TODO: what does 'v' depend on?
}

// RefPicListModification - AVC Ref Pic list modification at slice level
type RefPicListModification struct {
	RefPicListModificationFlagL0 bool // u(1)
	RefPicListModificationFlagL1 bool // u(1)
	PerEntryParams               []PerEntryRefPicListModParams
}

// PerEntryRefPicListModParams - Ref Pic List Mod params per each entry
type PerEntryRefPicListModParams struct {
	ModificationOfPicNumsIDC uint // ue(v)
	AbsDiffPicNumMinus1      uint // ue(v)
	LongTermPicNum           uint // ue(v)
}

// PredWeightTable - AVC Prediction Weight Table in slice header
type PredWeightTable struct {
	LumaLog2WeightDenom   uint    // ue(v)
	ChromaLog2WeightDenom uint    // ue(v)
	LumaWeightL0Flag      bool    // u(1)
	LumaWeightL0          []int   // se(v)
	LumaOffsetL0          []int   // se(v)
	ChromaWeightL0Flag    bool    // u(1)
	ChromaWeightL0        [][]int // se(v)
	ChromaOffsetL0        [][]int // se(v)
	LumaWeightL1Flag      bool    // u(1)
	LumaWeightL1          []int   // se(v)
	LumaOffsetL1          []int   // se(v)
	ChromaWeightL1Flag    bool    // u(1)
	ChromaWeightL1        [][]int // se(v)
	ChromaOffsetL1        [][]int // se(v)
}

// DecRefPicMarking - Decoded Reference Picture Marking Syntax
type DecRefPicMarking struct {
	NoOutputOfPriorPicsFlag       bool // u(1)
	LongTermReferenceFlag         bool // u(1)
	AdaptiveRefPicMarkingModeFlag bool // u(1)
	AdaptiveRefPicMarkingParams   []AdaptiveMemCtrlDecRefPicMarkingParams
}

// AdaptiveMemCtrlDecRefPicMarkingParams - Used as explained in 8.2.5.4
type AdaptiveMemCtrlDecRefPicMarkingParams struct {
	MemoryManagementControlOperation uint // ue(v)
	DifferenceOfPicNumsMinus1        uint // ue(v)
	LongTermPicNum                   uint // ue(v)
	LongTermFrameIdx                 uint // ue(v)
	MaxLongTermFrameIdxPlus1         uint // ue(V)
}

// GetSliceTypeFromNALU - parse slice header to get slice type in interval 0 to 4
// This function is no longer necessary after the ParseSliceHeader is added
func GetSliceTypeFromNALU(data []byte) (sliceType SliceType, err error) {

	if len(data) <= 1 {
		err = ErrTooFewBytesToParse
		return
	}

	naluType := GetNaluType(data[0])
	switch naluType {
	case 1, 2, 5, 19:
		// slice_layer_without_partitioning_rbsp
		// slice_data_partition_a_layer_rbsp

	default:
		err = ErrNoSliceHeader
		return
	}
	r := bits.NewEBSPReader(bytes.NewReader((data[1:])))

	// first_mb_in_slice
	if _, err = r.ReadExpGolomb(); err != nil {
		return
	}

	// slice_type
	var st uint
	if st, err = r.ReadExpGolomb(); err != nil {
		return
	}
	sliceType = SliceType(st)
	if sliceType > 9 {
		err = ErrInvalidSliceType
		return
	}

	if sliceType >= 5 {
		sliceType -= 5 // The same type is repeated twice to tell if all slices in picture are the same
	}
	return
}

// ParseSliceHeader - Parse AVC Slice Header starting with NAL header
func ParseSliceHeader(data []byte, sps *SPS, pps *PPS) (*SliceHeader, int, error) {
	avcsh := &SliceHeader{}
	var err error

	rd := bytes.NewReader(data)
	r := bits.NewEBSPReader(rd)

	// Note! First byte is NAL Header
	nalHdr, err := r.Read(8)
	if err != nil {
		return nil, r.NrBytesRead(), err
	}
	nalType := GetNaluType(byte(nalHdr))
	if !sliceHeaderExpected(nalType) {
		return nil, r.NrBytesRead(), ErrNoSliceHeader
	}

	avcsh.FirstMbInSlice, err = r.ReadExpGolomb()
	if err != nil {
		return nil, r.NrBytesRead(), err
	}
	sliceType, err := r.ReadExpGolomb()
	if err != nil {
		return nil, r.NrBytesRead(), err
	}
	avcsh.SliceType, err = setSliceType(sliceType)
	if err != nil {
		return nil, r.NrBytesRead(), err
	}

	avcsh.PicParameterSetID, err = r.ReadExpGolomb()
	if err != nil {
		return nil, r.NrBytesRead(), err
	}
	if sps.SeparateColourPlaneFlag {
		avcsh.ColourPlaneID, err = r.Read(2)
		if err != nil {
			return nil, r.NrBytesRead(), err
		}
	}
	avcsh.FrameNum, err = r.Read(int(sps.Log2MaxFrameNumMinus4 + 4))
	if err != nil {
		return nil, r.NrBytesRead(), err
	}
	if !sps.FrameMbsOnlyFlag {
		avcsh.FieldPicFlag, err = r.ReadFlag()
		if err != nil {
			return nil, r.NrBytesRead(), err
		}
		if avcsh.FieldPicFlag {
			avcsh.BottomFieldFlag, err = r.ReadFlag()
			if err != nil {
				return nil, r.NrBytesRead(), err
			}
		}
	}
	if getIDRPicFlag(nalType) {
		avcsh.IDRPicID, err = r.ReadExpGolomb()
		if err != nil {
			return nil, r.NrBytesRead(), err
		}
	}

	if sps.PicOrderCntType == 0 {
		avcsh.PicOrderCntLSB, err = r.Read(int(sps.Log2MaxPicOrderCntLsbMinus4 + 4))
		if err != nil {
			return nil, r.NrBytesRead(), err
		}
		if pps.BottomFieldPicOrderInFramePresentFlag && !avcsh.FieldPicFlag {
			avcsh.DeltaPicOrderCntBottom, err = r.ReadSignedGolomb()
			if err != nil {
				return nil, r.NrBytesRead(), err
			}
		}
	}
	if sps.PicOrderCntType == 1 && !sps.DeltaPicOrderAlwaysZeroFlag {
		avcsh.DeltaPicOrderCnt[0], err = r.ReadSignedGolomb()
		if err != nil {
			return nil, r.NrBytesRead(), err
		}
		if pps.BottomFieldPicOrderInFramePresentFlag && !avcsh.FieldPicFlag {
			avcsh.DeltaPicOrderCnt[1], err = r.ReadSignedGolomb()
			if err != nil {
				return nil, r.NrBytesRead(), err
			}
		}
	}
	if pps.RedundantPicCntPresentFlag {
		avcsh.RedundantPicCnt, err = r.ReadExpGolomb()
		if err != nil {
			return nil, r.NrBytesRead(), err
		}
	}

	if avcsh.SliceType == SLICE_B {
		avcsh.DirectSpatialMVPredFlag, err = r.ReadFlag()
		if err != nil {
			return nil, r.NrBytesRead(), err
		}
	}
	if avcsh.SliceType == SLICE_P || avcsh.SliceType == SLICE_SP || avcsh.SliceType == SLICE_B {
		avcsh.NumRefIdxActiveOverrideFlag, err = r.ReadFlag()
		if err != nil {
			return nil, r.NrBytesRead(), err
		}
	}
	if avcsh.NumRefIdxActiveOverrideFlag {
		avcsh.NumRefIdxL0ActiveMinus1, err = r.ReadExpGolomb()
		if err != nil {
			return nil, r.NrBytesRead(), err
		}
		if avcsh.SliceType == SLICE_B {
			avcsh.NumRefIdxL1ActiveMinus1, err = r.ReadExpGolomb()
			if err != nil {
				return nil, r.NrBytesRead(), err
			}
		}
	}

	if nalType == NaluType(20) || nalType == NaluType(21) {
		// MVC not implemented
		return nil, r.NrBytesRead(), ErrParserNotImplemented
	} else {
		avcsh.RefPicListModification, err = ParseRefPicListModification(r, avcsh)
		if err != nil {
			return nil, r.NrBytesRead(), err
		}
	}
	if (pps.WeightedPredFlag && (avcsh.SliceType == SLICE_P || avcsh.SliceType == SLICE_SP)) ||
		(pps.WeightedBipredIDC == 1 && avcsh.SliceType == SLICE_B) {
		avcsh.PredWeightTable, err = ParsePredWeightTable(r, sps, avcsh)
		if err != nil {
			return nil, r.NrBytesRead(), err
		}
	}
	if GetNalRefIDC(byte(nalHdr)) != 0 {
		avcsh.DecRefPicMarking, err = ParseDecRefPicMarking(r, nalType)
		if err != nil {
			return nil, r.NrBytesRead(), err
		}
	}

	if pps.EntropyCodingModeFlag && avcsh.SliceType != SLICE_I && avcsh.SliceType != SLICE_SI {
		avcsh.CabacInitIDC, err = r.ReadExpGolomb()
		if err != nil {
			return nil, r.NrBytesRead(), err
		}
	}

	avcsh.SliceQPDelta, err = r.ReadSignedGolomb()
	if err != nil {
		return nil, r.NrBytesRead(), err
	}
	if avcsh.SliceType == SLICE_SP || avcsh.SliceType == SLICE_SI {
		if avcsh.SliceType == SLICE_SP {
			avcsh.SPForSwitchFlag, err = r.ReadFlag()
			if err != nil {
				return nil, r.NrBytesRead(), err
			}
		}
		avcsh.SliceQSDelta, err = r.ReadSignedGolomb()
		if err != nil {
			return nil, r.NrBytesRead(), err
		}
	}

	if pps.DeblockingFilterControlPresentFlag {
		avcsh.DisableDeblockingFilterIDC, err = r.ReadExpGolomb()
		if err != nil {
			return nil, r.NrBytesRead(), err
		}
		if avcsh.DisableDeblockingFilterIDC != 1 {
			avcsh.SliceAlphaC0OffsetDev2, err = r.ReadSignedGolomb()
			if err != nil {
				return nil, r.NrBytesRead(), err
			}
			avcsh.SliceBetaOffsetDev2, err = r.ReadSignedGolomb()
			if err != nil {
				return nil, r.NrBytesRead(), err
			}
		}
	}

	if pps.NumSliceGroupsMinus1 > 0 && pps.SliceGroupMapType >= 3 && pps.SliceGroupMapType <= 5 {
		// based on equation 7-35 H.264 spec
		sgccNumBits := math.Ceil(math.Log2(float64((pps.PicSizeInMapUnitsMinus1+1)/(pps.SliceGroupChangeRateMinus1+1) + 1)))
		avcsh.SliceGroupChangeCycle, err = r.Read(int(sgccNumBits))
		if err != nil {
			return nil, r.NrBytesRead(), err
		}
	}

	return avcsh, r.NrBytesRead(), nil
}

// ParseRefPicListModification - AVC Ref Pic list modification parser using bits r
func ParseRefPicListModification(r *bits.EBSPReader, avcsh *SliceHeader) (*RefPicListModification, error) {
	rplm := &RefPicListModification{}
	var err error

	if avcsh.SliceType%5 != 2 && avcsh.SliceType%5 != 4 {
		rplm.RefPicListModificationFlagL0, err = r.ReadFlag()
		if err != nil {
			return nil, err
		}

		if rplm.RefPicListModificationFlagL0 {
			for mopni := 0; mopni != 3; {
				rplmEntry := PerEntryRefPicListModParams{}
				rplmEntry.ModificationOfPicNumsIDC, err = r.ReadExpGolomb()
				if err != nil {
					return nil, err
				}
				if rplmEntry.ModificationOfPicNumsIDC == 0 || rplmEntry.ModificationOfPicNumsIDC == 1 {
					rplmEntry.AbsDiffPicNumMinus1, err = r.ReadExpGolomb()
					if err != nil {
						return nil, err
					}
				} else if rplmEntry.ModificationOfPicNumsIDC == 2 {
					rplmEntry.LongTermPicNum, err = r.ReadExpGolomb()
					if err != nil {
						return nil, err
					}
				}
				rplm.PerEntryParams = append(rplm.PerEntryParams, rplmEntry)
			}
		}
	}

	if avcsh.SliceType%5 == 1 {
		rplm.RefPicListModificationFlagL1, err = r.ReadFlag()
		if err != nil {
			return nil, err
		}

		if rplm.RefPicListModificationFlagL0 {
			for mopni := 0; mopni != 3; {
				rplmEntry := PerEntryRefPicListModParams{}
				rplmEntry.ModificationOfPicNumsIDC, err = r.ReadExpGolomb()
				if err != nil {
					return nil, err
				}
				if rplmEntry.ModificationOfPicNumsIDC == 0 || rplmEntry.ModificationOfPicNumsIDC == 1 {
					rplmEntry.AbsDiffPicNumMinus1, err = r.ReadExpGolomb()
					if err != nil {
						return nil, err
					}
				} else if rplmEntry.ModificationOfPicNumsIDC == 2 {
					rplmEntry.LongTermPicNum, err = r.ReadExpGolomb()
					if err != nil {
						return nil, err
					}
				}
				rplm.PerEntryParams = append(rplm.PerEntryParams, rplmEntry)
			}
		}
	}

	return rplm, nil
}

// ParsePredWeightTable - AVC Slice Prediction Weight Table parser using bits r
func ParsePredWeightTable(r *bits.EBSPReader, sps *SPS, avcsh *SliceHeader) (*PredWeightTable, error) {
	pwt := &PredWeightTable{}
	var err error

	pwt.LumaLog2WeightDenom, err = r.ReadExpGolomb()
	if err != nil {
		return nil, err
	}

	if getChromaArrayType(sps) != 0 {
		pwt.ChromaLog2WeightDenom, err = r.ReadExpGolomb()
		if err != nil {
			return nil, err
		}
	}

	for i := uint(0); i <= avcsh.NumRefIdxL0ActiveMinus1; i++ {
		pwt.LumaWeightL0Flag, err = r.ReadFlag()
		if err != nil {
			return nil, err
		}
		if pwt.LumaWeightL0Flag {
			lumaWeightL0, err := r.ReadSignedGolomb()
			if err != nil {
				return nil, err
			}
			lumaOffsetL0, err := r.ReadSignedGolomb()
			if err != nil {
				return nil, err
			}
			pwt.LumaWeightL0 = append(pwt.LumaWeightL0, lumaWeightL0)
			pwt.LumaOffsetL0 = append(pwt.LumaWeightL0, lumaOffsetL0)
		}

		if getChromaArrayType(sps) != 0 {
			pwt.ChromaWeightL0Flag, err = r.ReadFlag()
			if err != nil {
				return nil, err
			}
			if pwt.ChromaWeightL0Flag {
				var chromaWeightL0, chromaOffsetL0 []int
				for j := 0; j < 2; j++ {
					chromaWeight, err := r.ReadSignedGolomb()
					if err != nil {
						return nil, err
					}
					chromaOffset, err := r.ReadSignedGolomb()
					if err != nil {
						return nil, err
					}
					chromaWeightL0 = append(chromaWeightL0, chromaWeight)
					chromaOffsetL0 = append(chromaOffsetL0, chromaOffset)
				}
				pwt.ChromaWeightL0 = append(pwt.ChromaWeightL0, chromaWeightL0)
				pwt.ChromaOffsetL0 = append(pwt.ChromaWeightL0, chromaOffsetL0)
			}
		}
	}

	if avcsh.SliceType%5 == 1 {
		for i := uint(0); i <= avcsh.NumRefIdxL1ActiveMinus1; i++ {
			pwt.LumaWeightL1Flag, err = r.ReadFlag()
			if err != nil {
				return nil, err
			}
			if pwt.LumaWeightL1Flag {
				lumaWeightL1, err := r.ReadSignedGolomb()
				if err != nil {
					return nil, err
				}
				lumaOffsetL1, err := r.ReadSignedGolomb()
				if err != nil {
					return nil, err
				}
				pwt.LumaWeightL1 = append(pwt.LumaWeightL1, lumaWeightL1)
				pwt.LumaOffsetL1 = append(pwt.LumaWeightL1, lumaOffsetL1)
			}

			if getChromaArrayType(sps) != 0 {
				pwt.ChromaWeightL1Flag, err = r.ReadFlag()
				if err != nil {
					return nil, err
				}
				if pwt.ChromaWeightL1Flag {
					var chromaWeightL1, chromaOffsetL1 []int
					for j := 0; j < 2; j++ {
						chromaWeight, err := r.ReadSignedGolomb()
						if err != nil {
							return nil, err
						}
						chromaOffset, err := r.ReadSignedGolomb()
						if err != nil {
							return nil, err
						}
						chromaWeightL1 = append(chromaWeightL1, chromaWeight)
						chromaOffsetL1 = append(chromaOffsetL1, chromaOffset)
					}
					pwt.ChromaWeightL1 = append(pwt.ChromaWeightL1, chromaWeightL1)
					pwt.ChromaOffsetL1 = append(pwt.ChromaWeightL1, chromaOffsetL1)
				}
			}
		}
	}

	return pwt, nil
}

// ParseDecRefPicMarking - AVC Slice Decoded Reference Picture Marking parser using bits r
func ParseDecRefPicMarking(r *bits.EBSPReader, naluType NaluType) (*DecRefPicMarking, error) {
	rpm := &DecRefPicMarking{}
	var err error

	if getIDRPicFlag(naluType) {
		rpm.NoOutputOfPriorPicsFlag, err = r.ReadFlag()
		if err != nil {
			return nil, err
		}
		rpm.LongTermReferenceFlag, err = r.ReadFlag()
		if err != nil {
			return nil, err
		}
	} else {
		rpm.AdaptiveRefPicMarkingModeFlag, err = r.ReadFlag()
		if err != nil {
			return nil, err
		}

		if rpm.AdaptiveRefPicMarkingModeFlag {
			for mmco := 1; mmco != 0; {
				arpmParams := AdaptiveMemCtrlDecRefPicMarkingParams{}
				arpmParams.MemoryManagementControlOperation, err = r.ReadExpGolomb()
				if err != nil {
					return nil, err
				}
				mmco := arpmParams.MemoryManagementControlOperation

				if mmco == 1 || mmco == 3 {
					arpmParams.DifferenceOfPicNumsMinus1, err = r.ReadExpGolomb()
					if err != nil {
						return nil, err
					}
				}
				if mmco == 2 {
					arpmParams.LongTermPicNum, err = r.ReadExpGolomb()
					if err != nil {
						return nil, err
					}
				}
				if mmco == 3 || mmco == 6 {
					arpmParams.LongTermFrameIdx, err = r.ReadExpGolomb()
					if err != nil {
						return nil, err
					}
				}
				if mmco == 4 {
					arpmParams.MaxLongTermFrameIdxPlus1, err = r.ReadExpGolomb()
					if err != nil {
						return nil, err
					}
				}

				rpm.AdaptiveRefPicMarkingParams = append(rpm.AdaptiveRefPicMarkingParams, arpmParams)
			}
		}

	}

	return rpm, nil
}

// getIDRPicFlag - Sets IDR Pic flag based on H.264 Spec equation 7-1
// The equation sets value as 1/0, but uses like a bool in the tabular syntax
func getIDRPicFlag(naluType NaluType) bool {
	if naluType == NALU_IDR {
		return true
	}
	return false
}

// getChromaArrayType - Sets ChromaArrayType based on the SeparateColourPlaneFlag from SPS
// this is based on H.264 spec 7.4.2.1.1
func getChromaArrayType(sps *SPS) uint {
	if sps.SeparateColourPlaneFlag {
		return 0
	}
	return sps.ChromaFormatIDC
}

// sliceHeaderExpected - Tells if a slice header should be expected: Nal types: 1,2,5,19
func sliceHeaderExpected(naluType NaluType) bool {
	switch naluType {
	case 1, 2, 5, 19:
		// slice_layer_without_partitioning_rbsp
		// slice_data_partition_a_layer_rbsp
		return true
	default:
		return false
	}
}

// setSliceType validates the value and sets the type based on numerical value
func setSliceType(sliceType uint) (SliceType, error) {
	if sliceType > 9 {
		return SliceType(sliceType), ErrInvalidSliceType
	}

	if sliceType >= 5 {
		sliceType -= 5 // The same type is repeated twice to tell if all slices in picture are the same
	}
	return SliceType(sliceType), nil
}
