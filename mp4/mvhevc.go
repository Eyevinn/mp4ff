package mp4

import (
	"fmt"

	"github.com/Eyevinn/mp4ff/hevc"
)

// BuildOinfFromVPS builds an Operating Points Information sample group entry
// ('oinf', ISO/IEC 14496-15 Sec. 9.6.2) from a parsed layered VPS. It requires
// the VPS to carry a multilayer extension (vps.Extension != nil), as produced for
// an MV-HEVC/SHVC bitstream. The derivation follows GPAC's naludmx_set_hevc_oinf.
func BuildOinfFromVPS(vps *hevc.VPS) (*OinfSampleGroupEntry, error) {
	if vps == nil || vps.Extension == nil {
		return nil, fmt.Errorf("oinf requires a multilayer VPS with a parsed extension")
	}
	ext := vps.Extension
	e := &OinfSampleGroupEntry{ScalabilityMask: vps.ScalabilityMaskBits()}

	// Profile tier levels: index 0 is the base layer PTL, the rest are the
	// extension PTLs parsed from vps_extension().
	for i := 0; i < ext.NumProfileTierLevel; i++ {
		ptl := vps.ProfileTierLevel
		if i > 0 {
			if i-1 >= len(ext.ExtProfileTierLevels) {
				break
			}
			ptl = ext.ExtProfileTierLevels[i-1]
		}
		e.ProfileTierLevels = append(e.ProfileTierLevels, OinfPTL{
			GeneralProfileSpace:              ptl.GeneralProfileSpace,
			GeneralTierFlag:                  ptl.GeneralTierFlag,
			GeneralProfileIDC:                ptl.GeneralProfileIDC,
			GeneralProfileCompatibilityFlags: ptl.GeneralProfileCompatibilityFlags,
			GeneralConstraintIndicatorFlags:  ptl.GeneralConstraintIndicatorFlags,
			GeneralLevelIDC:                  ptl.GeneralLevelIDC,
		})
	}

	numDims := popcount16(e.ScalabilityMask)

	// One operating point per output layer set. For MV-HEVC there are no
	// additional output layer sets, so the OLS index equals its layer-set index;
	// LayerSetLayerIdList / NecessaryLayersFlag / ProfileTierLevelIdx are all
	// indexed by that value. Output layer sets that map to a different layer set
	// (num_add_olss > 0) are not supported here, as that mapping is not retained.
	numOLS := ext.NumOutputLayerSets
	if numOLS > len(ext.NumNecessaryLayers) {
		numOLS = len(ext.NumNecessaryLayers)
	}
	for i := 0; i < numOLS; i++ {
		op := OinfOperatingPoint{OutputLayerSetIdx: uint16(i)}
		numInSet := ext.NumLayersInIdList[i]
		if numInSet > len(ext.LayerSetLayerIdList[i]) {
			numInSet = len(ext.LayerSetLayerIdList[i])
		}
		var minW, minH, maxW, maxH uint16
		var maxChroma, maxBitDepth, maxTid byte
		// List the necessary layers of the OLS, keyed by their nuh_layer_id.
		for k := 0; k < numInSet; k++ {
			if !ext.NecessaryLayersFlag[i][k] {
				continue
			}
			nuhLayerID := ext.LayerSetLayerIdList[i][k]
			vpsIdx := ext.LayerIdInVps[nuhLayerID]
			op.Layers = append(op.Layers, OinfOPLayer{
				PtlIdx:        ext.ProfileTierLevelIdx[i][k],
				LayerID:       nuhLayerID,
				IsOutputLayer: ext.OutputLayerFlag[i][k],
			})
			if ext.SubLayersVpsMaxMinus1[vpsIdx] > maxTid {
				maxTid = ext.SubLayersVpsMaxMinus1[vpsIdx]
			}
			fmtIdx := ext.RepFormatIdx[vpsIdx]
			if int(fmtIdx) >= len(ext.RepFormats) {
				continue
			}
			rf := ext.RepFormats[fmtIdx]
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
		op.MaxTemporalID = maxTid
		op.MinPicWidth = minW
		op.MinPicHeight = minH
		op.MaxPicWidth = maxW
		op.MaxPicHeight = maxH
		op.MaxChromaFormat = maxChroma
		if maxBitDepth >= 8 {
			op.MaxBitDepthMinus8 = maxBitDepth - 8
		}
		e.OperatingPoints = append(e.OperatingPoints, op)
	}

	// Dependency layers: one entry per layer with its direct reference layers and
	// the dimension IDs corresponding to the set scalability_mask bits.
	for i := 0; i <= int(vps.MaxLayersMinus1) && i < len(ext.LayerIdInNuh); i++ {
		nuhLID := ext.LayerIdInNuh[i]
		dep := OinfDependencyLayer{LayerID: nuhLID}
		for j := byte(0); j < ext.NumDirectRefLayers[nuhLID] && int(j) < len(ext.IdDirectRefLayers[nuhLID]); j++ {
			dep.DependsOnLayers = append(dep.DependsOnLayers, ext.IdDirectRefLayers[nuhLID][j])
		}
		dimIdx := 0
		for k := 0; k < 16; k++ {
			if e.ScalabilityMask&(1<<k) != 0 {
				if dimIdx < len(ext.DimensionId[i]) {
					dep.DimensionIds = append(dep.DimensionIds, ext.DimensionId[i][dimIdx])
				}
				dimIdx++
			}
		}
		for len(dep.DimensionIds) < numDims {
			dep.DimensionIds = append(dep.DimensionIds, 0)
		}
		e.DependencyLayers = append(e.DependencyLayers, dep)
	}

	return e, nil
}

// BuildLinfFromVPS builds a Layer Information sample group entry ('linf',
// ISO/IEC 14496-15 Sec. 4.15) covering all layers of a layered VPS. It requires
// the VPS to carry a multilayer extension. maxTemporalIDs optionally gives the
// observed max temporal id per VPS layer index; a missing entry falls back to the
// VPS-signalled sub_layers_vps_max_minus1 for that layer.
func BuildLinfFromVPS(vps *hevc.VPS, maxTemporalIDs []byte) (*LinfSampleGroupEntry, error) {
	if vps == nil || vps.Extension == nil {
		return nil, fmt.Errorf("linf requires a multilayer VPS with a parsed extension")
	}
	ext := vps.Extension
	e := &LinfSampleGroupEntry{}
	numLayers := int(vps.MaxLayersMinus1) + 1
	for i := 0; i < numLayers && i < len(ext.LayerIdInNuh); i++ {
		maxTid := ext.SubLayersVpsMaxMinus1[i]
		if i < len(maxTemporalIDs) {
			maxTid = maxTemporalIDs[i]
		}
		e.Layers = append(e.Layers, LinfLayerEntry{
			LayerID:               ext.LayerIdInNuh[i],
			MinTemporalID:         0,
			MaxTemporalID:         maxTid,
			SubLayerPresenceFlags: subLayerPresenceMask(maxTid),
		})
	}
	return e, nil
}

// subLayerPresenceMask returns the 7-bit sub_layer_presence_flags value with the
// bits for temporal ids 0..maxTid set.
func subLayerPresenceMask(maxTid byte) byte {
	if maxTid >= 6 {
		return 0x7f
	}
	return byte((1 << (maxTid + 1)) - 1)
}
