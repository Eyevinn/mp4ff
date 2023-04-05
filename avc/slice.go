package avc

import (
	"bytes"
	"errors"
	"fmt"
	"math"

	"github.com/Eyevinn/mp4ff/bits"
)

// Errors for parsing and handling AVC slices
var (
	ErrNoSliceHeader      = errors.New("No slice header")
	ErrInvalidSliceType   = errors.New("Invalid slice type")
	ErrTooFewBytesToParse = errors.New("Too few bytes to parse symbol")
)

// SliceType - AVC slice type
type SliceType uint

func (s SliceType) String() string {
	switch s {
	case SLICE_I:
		return "I"
	case SLICE_P:
		return "P"
	case SLICE_B:
		return "B"
	case SLICE_SI:
		return "SI"
	case SLICE_SP:
		return "SP"
	default:
		return ""
	}
}

// AVC slice types
const (
	SLICE_P  = SliceType(0)
	SLICE_B  = SliceType(1)
	SLICE_I  = SliceType(2)
	SLICE_SP = SliceType(3)
	SLICE_SI = SliceType(4)
)

// GetSliceTypeFromNALU - parse slice header to get slice type in interval 0 to 4
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

type SliceHeader struct {
	SliceType                     SliceType
	FirstMBInSlice                uint32
	PicParamID                    uint32
	SeqParamID                    uint32
	ColorPlaneID                  uint32
	FrameNum                      uint32
	IDRPicID                      uint32
	PicOrderCntLsb                uint32
	DeltaPicOrderCntBottom        int32
	DeltaPicOrderCnt              [2]int32
	RedundantPicCnt               uint32
	NumRefIdxL0ActiveMinus1       uint32
	NumRefIdxL1ActiveMinus1       uint32
	ModificationOfPicNumsIDC      uint32
	AbsDiffPicNumMinus1           uint32
	LongTermPicNum                uint32
	LumaLog2WeightDenom           uint32
	ChromaLog2WeightDenom         uint32
	DifferenceOfPicNumsMinus1     uint32
	LongTermFramIdx               uint32
	MaxLongTermFrameIdxPlus1      uint32
	CabacInitIDC                  uint32
	SliceQPDelta                  int32
	SliceQSDelta                  int32
	DisableDeblockingFilterIDC    uint32
	SliceAlphaC0OffsetDiv2        int32
	SliceBetaOffsetDiv2           int32
	SliceGroupChangeCycle         uint32
	Size                          uint32
	FieldPicFlag                  bool
	BottomFieldFlag               bool
	DirectSpatialMvPredFlag       bool
	NumRefIdxActiveOverrideFlag   bool
	RefPicListModificationL0Flag  bool
	RefPicListModificationL1Flag  bool
	NoOutputOfPriorPicsFlag       bool
	LongTermReferenceFlag         bool
	SPForSwitchFlag               bool
	AdaptiveRefPicMarkingModeFlag bool
}

func ParseSliceHeader(nalu []byte, spsMap map[uint32]*SPS, ppsMap map[uint32]*PPS) (*SliceHeader, error) {
	sh := SliceHeader{}
	buf := bytes.NewBuffer(nalu)
	r := bits.NewAccErrEBSPReader(buf)
	nalHdr := r.Read(8)
	naluType := GetNaluType(byte(nalHdr))
	switch naluType {
	case 1, 2, 5, 19:
		// slice_layer_without_partitioning_rbsp
		// slice_data_partition_a_layer_rbsp
	default:
		err := ErrNoSliceHeader
		return nil, err
	}
	nalRefIDC := (nalHdr >> 5) & 0x3
	sh.FirstMBInSlice = uint32(r.ReadExpGolomb())
	sh.SliceType = SliceType(r.ReadExpGolomb())
	sh.PicParamID = uint32(r.ReadExpGolomb())
	pps, ok := ppsMap[sh.PicParamID]
	if !ok {
		return nil, fmt.Errorf("pps ID %d unknown", sh.PicParamID)
	}
	spsID := pps.PicParameterSetID
	sps, ok := spsMap[uint32(spsID)]
	if !ok {
		return nil, fmt.Errorf("sps ID %d unknown", spsID)
	}
	if sps.SeparateColourPlaneFlag {
		sh.ColorPlaneID = uint32(r.Read(2))
	}
	sh.FrameNum = uint32(r.Read(int(sps.Log2MaxFrameNumMinus4 + 4)))
	if !sps.FrameMbsOnlyFlag {
		sh.FieldPicFlag = r.ReadFlag()
		if sh.FieldPicFlag {
			sh.BottomFieldFlag = r.ReadFlag()
		}
	}
	if naluType == NALU_IDR {
		sh.IDRPicID = uint32(r.ReadExpGolomb())
	}
	if sps.PicOrderCntType == 0 {
		sh.PicOrderCntLsb = uint32(r.Read(int(sps.Log2MaxPicOrderCntLsbMinus4 + 4)))
		if pps.BottomFieldPicOrderInFramePresentFlag && !sh.FieldPicFlag {
			sh.DeltaPicOrderCntBottom = int32(r.ReadSignedGolomb())
		}
	} else if sps.PicOrderCntType == 1 && !sps.DeltaPicOrderAlwaysZeroFlag {
		sh.DeltaPicOrderCnt[0] = int32(r.ReadSignedGolomb())
		if pps.BottomFieldPicOrderInFramePresentFlag && !sh.FieldPicFlag {
			sh.DeltaPicOrderCnt[1] = int32(r.ReadSignedGolomb())
		}
	}
	if pps.RedundantPicCntPresentFlag {
		sh.RedundantPicCnt = uint32(r.ReadExpGolomb())
	}

	sliceType := SliceType(sh.SliceType % 5)
	if sliceType == SLICE_B {
		sh.DirectSpatialMvPredFlag = r.ReadFlag()
	}

	switch sliceType {
	case SLICE_P, SLICE_SP, SLICE_B:
		sh.NumRefIdxActiveOverrideFlag = r.ReadFlag()

		if sh.NumRefIdxActiveOverrideFlag {
			sh.NumRefIdxL0ActiveMinus1 = uint32(r.ReadExpGolomb())
			if sliceType == SLICE_B {
				sh.NumRefIdxL1ActiveMinus1 = uint32(r.ReadExpGolomb())
			}
		} else {
			sh.NumRefIdxL0ActiveMinus1 = uint32(pps.NumRefIdxI0DefaultActiveMinus1)
			sh.NumRefIdxL1ActiveMinus1 = uint32(pps.NumRefIdxI1DefaultActiveMinus1)
		}
	}

	// ref_pic_list_modification (nal unit type != 20)
	if sliceType != SLICE_I && sliceType != SLICE_SI {
		sh.RefPicListModificationL0Flag = r.ReadFlag()
		if sh.RefPicListModificationL0Flag {
			for {
				sh.ModificationOfPicNumsIDC = uint32(r.ReadExpGolomb())
				switch sh.ModificationOfPicNumsIDC {
				case 0, 1:
					sh.AbsDiffPicNumMinus1 = uint32(r.ReadExpGolomb())
				case 2:
					sh.LongTermPicNum = uint32(r.ReadExpGolomb())
				case 3:
					break
				}
				if r.AccError() != nil {
					break
				}
			}
		}
	}
	if sliceType == SLICE_B {
		sh.RefPicListModificationL1Flag = r.ReadFlag()
		if sh.RefPicListModificationL1Flag {
			for {
				sh.ModificationOfPicNumsIDC = uint32(r.ReadExpGolomb())
				switch sh.ModificationOfPicNumsIDC {
				case 0, 1:
					sh.AbsDiffPicNumMinus1 = uint32(r.ReadExpGolomb())
				case 2:
					sh.LongTermPicNum = uint32(r.ReadExpGolomb())
				case 3:
					break
				}
			}
		}
	}
	// end ref_pic_list_modification

	if pps.WeightedPredFlag && (sliceType == SLICE_P || sliceType == SLICE_SP) ||
		(pps.WeightedBipredIDC == 1 && sliceType == SLICE_B) {
		// pred_weight_table
		sh.LumaLog2WeightDenom = uint32(r.ReadExpGolomb())
		if sps.ChromaArrayType() != 0 {
			sh.ChromaLog2WeightDenom = uint32(r.ReadExpGolomb())
		}

		for i := uint32(0); i <= sh.NumRefIdxL0ActiveMinus1; i++ {
			lumaWeightL0Flag := r.ReadFlag()
			if lumaWeightL0Flag {
				// Just parse, don't store this
				_ = r.ReadExpGolomb() // luma_weight_l0[i] = SignedGolomb()
				_ = r.ReadExpGolomb() // luma_offset_l0[i] = SignedGolomb()
			}
			if sps.ChromaArrayType() != 0 {
				chromaWeightL0Flag := r.ReadFlag()
				if chromaWeightL0Flag {
					for j := 0; j < 2; j++ {
						// Just parse, don't store this
						_ = r.ReadExpGolomb() // chroma_weight_l0[i][j] = SignedGolomb()
						_ = r.ReadExpGolomb() // chroma_offset_l0[i][j] = SignedGolomb()
					}
				}
			}
		}
		if sliceType == SLICE_B {
			for i := uint32(0); i <= sh.NumRefIdxL1ActiveMinus1; i++ {
				lumaWeightL1Flag := r.ReadFlag()
				if lumaWeightL1Flag {
					// Just parse, don't store this
					_ = r.ReadExpGolomb() // luma_weight_l1[i] = SignedGolomb()
					_ = r.ReadExpGolomb() // luma_offset_l1[i] = SignedGolomb()
				}
				if sps.ChromaFormatIDC != 0 {
					chromaWeightL0Flag := r.ReadFlag()
					if chromaWeightL0Flag {
						// Just parse, don't store this
						for j := 0; j < 2; j++ {
							_ = r.ReadSignedGolomb() // chroma_weight_l1[i][j] = SignedGolomb()
							_ = r.ReadSignedGolomb() // chroma_offset_l1[i][j] = SignedGolomb()
						}
					}
				}
			}
		}
		// end pred_weight_table
	}

	if nalRefIDC != 0 {
		// dec_ref_pic_marking
		if naluType == NALU_IDR {
			sh.NoOutputOfPriorPicsFlag = r.ReadFlag()
			sh.LongTermReferenceFlag = r.ReadFlag()
		} else {
			sh.AdaptiveRefPicMarkingModeFlag = r.ReadFlag()
			if sh.AdaptiveRefPicMarkingModeFlag {
				for {
					memoryManagementControlOperation := r.ReadExpGolomb()
					switch memoryManagementControlOperation {
					case 1, 3:
						sh.DifferenceOfPicNumsMinus1 = uint32(r.ReadExpGolomb())
					case 2:
						sh.LongTermPicNum = uint32(r.ReadExpGolomb())
					}
					switch memoryManagementControlOperation {
					case 3, 6:
						sh.LongTermFramIdx = uint32(r.ReadExpGolomb())
					case 4:
						sh.MaxLongTermFrameIdxPlus1 = uint32(r.ReadExpGolomb())
					case 0:
						break
					}
				}
			}
		}
		// end dec_ref_pic_marking
	}
	if pps.EntropyCodingModeFlag && sliceType != SLICE_I && sliceType != SLICE_SI {
		sh.CabacInitIDC = uint32(r.ReadExpGolomb())
	}
	sh.SliceQPDelta = int32(r.ReadSignedGolomb())
	if sliceType == SLICE_SP || sliceType == SLICE_SI {
		if sliceType == SLICE_SP {
			sh.SPForSwitchFlag = r.ReadFlag()
		}
		sh.SliceQSDelta = int32(r.ReadSignedGolomb())
	}
	if pps.DeblockingFilterControlPresentFlag {
		sh.DisableDeblockingFilterIDC = uint32(r.ReadExpGolomb())
		if sh.DisableDeblockingFilterIDC != 1 {
			sh.SliceAlphaC0OffsetDiv2 = int32(r.ReadSignedGolomb())
			sh.SliceBetaOffsetDiv2 = int32(r.ReadSignedGolomb())
		}
	}
	if pps.NumSliceGroupsMinus1 > 0 &&
		pps.SliceGroupMapType >= 3 &&
		pps.SliceGroupMapType <= 5 {
		picSizeInMapUnits := pps.PicSizeInMapUnitsMinus1 + 1
		sliceGroupChangeRage := pps.SliceGroupChangeRateMinus1 + 1
		nrBits := int(math.Ceil(math.Log2(float64(picSizeInMapUnits/sliceGroupChangeRage + 1))))
		sh.SliceGroupChangeCycle = uint32(r.Read(nrBits))
	}

	// compute the size in bytes. The last byte may not be fully read
	sh.Size = uint32(r.NrBytesRead())
	return &sh, nil
}
