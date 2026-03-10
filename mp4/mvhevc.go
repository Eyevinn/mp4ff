package mp4

import (
	"github.com/Eyevinn/mp4ff/hevc"
)

// BuildOinfFromVPS creates an oinf sample group entry from a parsed VPS.
// This follows the logic in gpac's naludmx_set_hevc_oinf (reframe_nalu.c).
func BuildOinfFromVPS(vps *hevc.VPS) *OinfSampleGroupEntry {
	e := &OinfSampleGroupEntry{}
	e.ScalabilityMask = vps.ScalabilityMaskBits()

	// Profile tier levels
	for i := 0; i < vps.NumProfileTierLevel; i++ {
		var ptl hevc.ProfileTierLevel
		if i == 0 {
			ptl = vps.ProfileTierLevel
		} else {
			ptl = vps.ExtProfileTierLevels[i-1]
		}
		oinfPTL := OinfPTL{
			GeneralProfileSpace:              ptl.GeneralProfileSpace,
			GeneralTierFlag:                  ptl.GeneralTierFlag,
			GeneralProfileIDC:                ptl.GeneralProfileIDC,
			GeneralProfileCompatibilityFlags: ptl.GeneralProfileCompatibilityFlags,
			GeneralConstraintIndicatorFlags:  ptl.GeneralConstraintIndicatorFlags,
			GeneralLevelIDC:                  ptl.GeneralLevelIDC,
		}
		e.ProfileTierLevels = append(e.ProfileTierLevels, oinfPTL)
	}

	// Operating points
	numDims := popcount16(e.ScalabilityMask)
	for i := 0; i < vps.NumOutputLayerSets; i++ {
		op := OinfOperatingPoint{
			OutputLayerSetIdx: uint16(i),
		}
		op.MaxTemporalID = vps.SubLayersVpsMaxMinus1[0] // default
		layerCount := vps.NumNecessaryLayers[i]
		for j := 0; j < layerCount; j++ {
			l := OinfOPLayer{
				PtlIdx:              vps.ProfileTierLevelIdx[i][j],
				LayerID:             byte(j),
				IsOutputLayer:       vps.OutputLayerFlag[i][j],
				IsAlternateOutLayer: false,
			}
			op.Layers = append(op.Layers, l)
		}

		// Compute min/max dimensions from rep formats
		var minW, minH, maxW, maxH uint16
		var maxChroma, maxBitDepth byte
		for j := 0; j < layerCount; j++ {
			fmtIdx := vps.RepFormatIdx[j]
			if int(fmtIdx) >= len(vps.RepFormats) {
				continue
			}
			rf := vps.RepFormats[fmtIdx]
			if minW == 0 || rf.PicWidthLumaSamples < minW {
				minW = rf.PicWidthLumaSamples
			}
			if minH == 0 || rf.PicHeightLumaSamples < minH {
				minH = rf.PicHeightLumaSamples
			}
			if rf.PicWidthLumaSamples > maxW {
				maxW = rf.PicWidthLumaSamples
			}
			if rf.PicHeightLumaSamples > maxH {
				maxH = rf.PicHeightLumaSamples
			}
			if rf.ChromaFormatIDC > maxChroma {
				maxChroma = rf.ChromaFormatIDC
			}
			bd := rf.BitDepthLuma
			if rf.BitDepthChroma > bd {
				bd = rf.BitDepthChroma
			}
			if bd > maxBitDepth {
				maxBitDepth = bd
			}
		}
		op.MinPicWidth = minW
		op.MinPicHeight = minH
		op.MaxPicWidth = maxW
		op.MaxPicHeight = maxH
		op.MaxChromaFormat = maxChroma
		if maxBitDepth >= 8 {
			op.MaxBitDepthMinus8 = maxBitDepth - 8
		}
		op.FrameRateInfoFlag = false
		op.BitRateInfoFlag = false

		e.OperatingPoints = append(e.OperatingPoints, op)
	}

	// Dependency layers
	for i := byte(0); i <= vps.MaxLayersMinus1; i++ {
		nuhLId := vps.LayerIdInNuh[i]
		dep := OinfDependencyLayer{
			LayerID: nuhLId,
		}
		for j := byte(0); j < vps.NumDirectRefLayers[nuhLId]; j++ {
			dep.DependsOnLayers = append(dep.DependsOnLayers, vps.IdDirectRefLayers[nuhLId][j])
		}
		// Dimension IDs: one per set bit in scalability mask
		dimIdx := 0
		for j := 0; j < 16; j++ {
			if e.ScalabilityMask&(1<<j) != 0 {
				dep.DimensionIds = append(dep.DimensionIds, vps.DimensionId[i][dimIdx])
				dimIdx++
			}
		}
		// Pad to numDims if needed
		for len(dep.DimensionIds) < numDims {
			dep.DimensionIds = append(dep.DimensionIds, 0)
		}
		e.DependencyLayers = append(e.DependencyLayers, dep)
	}

	return e
}

// BuildLinfFromVPS creates a linf sample group entry from observed layers.
// layerInfos should contain one entry per layer with layer ID and temporal ID range.
func BuildLinfFromVPS(vps *hevc.VPS, maxTemporalIDs []byte) *LinfSampleGroupEntry {
	e := &LinfSampleGroupEntry{}
	numLayers := int(vps.MaxLayersMinus1) + 1
	for i := 0; i < numLayers; i++ {
		nuhLId := vps.LayerIdInNuh[i]
		maxTid := byte(0)
		if i < len(maxTemporalIDs) {
			maxTid = maxTemporalIDs[i]
		}
		l := LinfLayerEntry{
			LayerID:               nuhLId,
			MinTemporalID:         0,
			MaxTemporalID:         maxTid,
			SubLayerPresenceFlags: 0x7f, // all sub-layers present
		}
		e.Layers = append(e.Layers, l)
	}
	return e
}

// CreateLhvCFromNalus creates an LhvCBox from enhancement layer parameter sets.
func CreateLhvCFromNalus(spsNalus, ppsNalus [][]byte) *LhvCBox {
	naluArrays := []hevc.NaluArray{
		hevc.NewNaluArray(true, hevc.NALU_SPS, spsNalus),
		hevc.NewNaluArray(true, hevc.NALU_PPS, ppsNalus),
	}
	dcr := hevc.DecConfRec{
		ConfigurationVersion: 1,
		LengthSizeMinusOne:   3,
		NaluArrays:           naluArrays,
	}
	return &LhvCBox{dcr}
}
