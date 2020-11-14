package avc

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"github.com/edgeware/mp4ff/bits"
)

type PPS struct {
	PicParameterSetID                     uint
	SeqParameterSetID                     uint
	EntropyCodingModeFlag                 bool
	BottomFieldPicOrderInFramePresentFlag bool
	NumSliceGroupsMinus1                  uint
	SliceGroupMapType                     uint
	RunLengthMinus1                       []uint
	TopLeft                               []uint
	BottomRight                           []uint
	SliceGroupChangeDirectionFlag         bool
	SliceGroupChangeRateMinus1            uint
	PicSizeInMapUnitsMinus1               uint
	SliceGroupId                          []uint
	NumRefIdxI0DefaultActiveMinus1        uint
	NumRefIdxI1DefaultActiveMinus1        uint
	WeightedPredFlag                      bool
	WeightedBipredIDC                     uint
	PicInitQpMinus26                      int
	PicInitQsMinus26                      int
	ChromaQpIndexOffset                   int
	DeblockingFilterControlPresentFlag    bool
	ConstrainedIntraPredFlag              bool
	RedundantPicCntPresentFlag            bool
	Transform8x8ModeFlag                  bool
	PicScalingMatrixPresentFlag           bool
	PicScalingLists                       []ScalingList
	SecondChromaQpIndexOffset             int
}

var ErrNotPPS = errors.New("Not an PPS NAL unit")

// ParsePPSNALUnit - Parse AVC PPS NAL unit starting with NAL header
func ParsePPSNALUnit(data []byte, sps *SPS) (*PPS, error) {

	var err error

	pps := &PPS{}

	rd := bytes.NewReader(data)
	reader := bits.NewEBSPReader(rd)
	// Note! First byte is NAL Header

	nalHdr, err := reader.Read(8)
	if err != nil {
		return nil, err
	}
	nalType := GetNalType(byte(nalHdr))
	if nalType != NALU_PPS {
		return nil, ErrNotPPS
	}

	pps.PicParameterSetID, err = reader.ReadExpGolomb()
	if err != nil {
		return nil, err
	}

	pps.SeqParameterSetID, err = reader.ReadExpGolomb()
	if err != nil {
		return nil, err
	}

	pps.EntropyCodingModeFlag, err = reader.ReadFlag()
	if err != nil {
		return nil, err
	}

	pps.BottomFieldPicOrderInFramePresentFlag, err = reader.ReadFlag()
	if err != nil {
		return nil, err
	}

	pps.NumSliceGroupsMinus1, err = reader.ReadExpGolomb()
	if err != nil {
		return nil, err
	}

	if pps.NumSliceGroupsMinus1 > 0 {
		pps.SliceGroupMapType, err = reader.ReadExpGolomb()
		if err != nil {
			return nil, err
		}
		switch pps.SliceGroupMapType {
		case 0:
			for iGroup := uint(0); iGroup <= pps.NumSliceGroupsMinus1; iGroup++ {
				rl, err := reader.ReadExpGolomb()
				if err != nil {
					return nil, err
				}
				pps.RunLengthMinus1 = append(pps.RunLengthMinus1, rl)
			}
		case 2:
			for iGroup := uint(0); iGroup <= pps.NumSliceGroupsMinus1; iGroup++ {
				tl, err := reader.ReadExpGolomb()
				if err != nil {
					return nil, err
				}
				pps.TopLeft = append(pps.TopLeft, tl)
				br, err := reader.ReadExpGolomb()
				if err != nil {
					return nil, err
				}
				pps.BottomRight = append(pps.BottomRight, br)
			}
		case 3, 4, 5:
			pps.SliceGroupChangeDirectionFlag, err = reader.ReadFlag()
			if err != nil {
				return nil, err
			}
			pps.SliceGroupChangeRateMinus1, err = reader.ReadExpGolomb()
			if err != nil {
				return nil, err
			}
		case 6:
			// slice_group_id[i] has Ceil(Log2(num_slice_groups_minus1 +1) bits)
			nrBits := ceilLog2(pps.NumSliceGroupsMinus1 + 1)

			for iGroup := uint(0); iGroup <= pps.NumSliceGroupsMinus1; iGroup++ {
				sgi, err := reader.Read(nrBits)
				if err != nil {
					return nil, err
				}
				pps.SliceGroupId = append(pps.SliceGroupId, sgi)
			}
		}
	}
	pps.NumRefIdxI0DefaultActiveMinus1, err = reader.ReadExpGolomb()
	if err != nil {
		return nil, err
	}
	pps.NumRefIdxI1DefaultActiveMinus1, err = reader.ReadExpGolomb()
	if err != nil {
		return nil, err
	}
	pps.WeightedPredFlag, err = reader.ReadFlag()
	if err != nil {
		return nil, err
	}
	pps.WeightedBipredIDC, err = reader.Read(2)
	if err != nil {
		return nil, err
	}
	pps.PicInitQpMinus26, err = reader.ReadSignedGolomb()
	if err != nil {
		return nil, err
	}
	pps.PicInitQsMinus26, err = reader.ReadSignedGolomb()
	if err != nil {
		return nil, err
	}
	pps.ChromaQpIndexOffset, err = reader.ReadSignedGolomb()
	if err != nil {
		return nil, err
	}
	pps.DeblockingFilterControlPresentFlag, err = reader.ReadFlag()
	if err != nil {
		return nil, err
	}
	pps.ConstrainedIntraPredFlag, err = reader.ReadFlag()
	if err != nil {
		return nil, err
	}
	pps.RedundantPicCntPresentFlag, err = reader.ReadFlag()
	if err != nil {
		return nil, err
	}
	if !reader.IsSeeker() {
		// Cannot call MoreRbspData, so cannot parse further
		return pps, nil
	}
	moreRbsp, err := reader.MoreRbspData()
	if err != nil {
		return nil, err
	}
	if moreRbsp {
		pps.Transform8x8ModeFlag, err = reader.ReadFlag()
		if err != nil {
			return nil, err
		}
		pps.PicScalingMatrixPresentFlag, err = reader.ReadFlag()
		if err != nil {
			return nil, err
		}
		if pps.PicScalingMatrixPresentFlag {
			if sps == nil {
				return pps, fmt.Errorf("Need SPS to decode PPS PicScalings")
			}
			nrScalingLists := 6
			if pps.Transform8x8ModeFlag {
				if sps.ChromaFormatIDC != 3 {
					nrScalingLists += 2
				} else {
					nrScalingLists += 6
				}
				pps.PicScalingLists = make([]ScalingList, nrScalingLists)

				for i := 0; i < nrScalingLists; i++ {
					picScalingPresent, err := reader.ReadFlag()
					if err != nil {
						return nil, err
					}
					if !picScalingPresent {
						pps.PicScalingLists[i] = nil
						continue
					}
					sizeOfScalingList := 16 // 4x4 for i < 6
					if i >= 6 {
						sizeOfScalingList = 64 // 8x8 for i >= 6
					}
					pps.PicScalingLists[i], err = readScalingList(reader, sizeOfScalingList)
					if err != nil {
						return nil, err
					}
				}
			}
		}
		pps.SecondChromaQpIndexOffset, err = reader.ReadSignedGolomb()
		if err != nil {
			return nil, err
		}
	}

	err = reader.ReadRbspTrailingBits()
	if err != nil {
		return nil, err
	}
	_, err = reader.Read(1)
	if err != io.EOF {
		return nil, fmt.Errorf("Not at end after reading rbsp_trailing_bits")
	}
	return pps, nil
}

// ceilLog2 - nr bits needed to represent numbers 0 - n-1 values
func ceilLog2(n uint) int {
	for i := 0; i < 32; i++ {
		maxNr := uint(1 << i)
		if maxNr >= n {
			return i
		}
	}
	return 32
}
