package hevc

import (
	"bytes"
	"fmt"

	"github.com/Eyevinn/mp4ff/bits"
)

const maxLayers = 8

// VPS - HEVC Video Parameter Set
// ISO/IEC 23008-2 Sec. 7.3.2.1
type VPS struct {
	VpsID                 byte
	BaseLayerInternalFlag bool
	BaseLayerAvailableFlag bool
	MaxLayersMinus1       byte
	MaxSubLayersMinus1    byte
	TemporalIDNestingFlag bool
	ProfileTierLevel      ProfileTierLevel

	// Sub-layer ordering info
	SubLayerOrderingInfos []SubLayerOrderingInfo

	MaxLayerID    byte
	NumLayerSets  uint16
	// LayerIdIncludedFlag[layerSet][layerId] (not stored, only used during parsing)

	// VPS extension fields (multi-layer/multi-view)
	ExtensionFound bool
	ScalabilityMask       [16]bool
	NumScalabilityTypes   int
	LayerIdInNuh          [maxLayers]byte
	LayerIdInVps          [64]byte // reverse mapping: nuh_layer_id -> vps layer index
	DimensionId           [maxLayers][16]byte
	DirectDependencyFlag  [maxLayers][maxLayers]bool
	NumDirectRefLayers    [64]byte
	IdDirectRefLayers     [64][maxLayers]byte

	NumProfileTierLevel   int
	ExtProfileTierLevels  []ProfileTierLevel // index 0 = ext_ptl[0], etc.
	NumOutputLayerSets    int
	OutputLayerFlag       [maxLayers][maxLayers]bool
	ProfileTierLevelIdx   [maxLayers][maxLayers]byte
	NumNecessaryLayers    [maxLayers]int
	NecessaryLayersFlag   [maxLayers][maxLayers]bool

	NumLayersInIdList     [maxLayers]int
	LayerSetLayerIdList   [maxLayers][maxLayers]byte

	// Rep formats
	NumRepFormats int
	RepFormats    []RepFormat
	RepFormatIdx  [maxLayers]byte

	SubLayersVpsMaxMinus1 [maxLayers]byte
}

// RepFormat holds resolution and format info for a layer.
type RepFormat struct {
	PicWidthLumaSamples  uint16
	PicHeightLumaSamples uint16
	ChromaFormatIDC      byte
	BitDepthLuma         byte
	BitDepthChroma       byte
}

// GetNumLayers returns the number of layers in this VPS.
func (v *VPS) GetNumLayers() int {
	return int(v.MaxLayersMinus1) + 1
}

// IsMultiLayer returns true if this VPS describes a multi-layer bitstream.
func (v *VPS) IsMultiLayer() bool {
	return v.MaxLayersMinus1 > 0 && v.ExtensionFound
}

// GetNumViews returns the number of unique views in the multi-view configuration.
func (v *VPS) GetNumViews() int {
	if !v.ExtensionFound {
		return 1
	}
	numViews := 1
	for i := 1; i <= int(v.MaxLayersMinus1); i++ {
		newView := true
		viewIdx := v.getViewIndex(i)
		for j := 0; j < i; j++ {
			if v.getViewIndex(j) == viewIdx {
				newView = false
				break
			}
		}
		if newView {
			numViews++
		}
	}
	return numViews
}

// getViewIndex returns the view order index for a layer (VPS index).
// The view order index is scalability dimension type 1 (LHVC_VIEW_ORDER_INDEX).
func (v *VPS) getViewIndex(layerIdx int) byte {
	dimIdx := 0
	for j := 0; j < 16; j++ {
		if v.ScalabilityMask[j] {
			if j == 1 { // view_order_index is scalability type 1
				return v.DimensionId[layerIdx][dimIdx]
			}
			dimIdx++
		}
	}
	return 0
}

// ScalabilityMaskBits returns the scalability mask as a 16-bit value.
func (v *VPS) ScalabilityMaskBits() uint16 {
	var mask uint16
	for i := 0; i < 16; i++ {
		if v.ScalabilityMask[i] {
			mask |= 1 << i
		}
	}
	return mask
}

// ParseVPSNALUnit parses a VPS NAL unit including the 2-byte NAL header.
func ParseVPSNALUnit(data []byte) (*VPS, error) {
	vps := &VPS{}
	rd := bytes.NewReader(data)
	r := bits.NewEBSPReader(rd)

	// Read 16 bits NALU header
	naluHdrBits := r.Read(16)
	naluType := GetNaluType(byte(naluHdrBits >> 8))
	if naluType != NALU_VPS {
		return nil, fmt.Errorf("NALU type is %s not VPS", naluType)
	}

	vps.VpsID = byte(r.Read(4))
	vps.BaseLayerInternalFlag = r.ReadFlag()
	vps.BaseLayerAvailableFlag = r.ReadFlag()
	vps.MaxLayersMinus1 = byte(r.Read(6))
	vps.MaxSubLayersMinus1 = byte(r.Read(3))
	vps.TemporalIDNestingFlag = r.ReadFlag()
	_ = r.Read(16) // vps_reserved_0xffff_16bits

	// Profile tier level for base layer
	vps.ProfileTierLevel = parseProfileTierLevel(r, true, vps.MaxSubLayersMinus1)

	// Sub-layer ordering info
	subLayerOrderingInfoPresentFlag := r.ReadFlag()
	startIdx := byte(0)
	if !subLayerOrderingInfoPresentFlag {
		startIdx = vps.MaxSubLayersMinus1
	}
	for i := startIdx; i <= vps.MaxSubLayersMinus1; i++ {
		info := SubLayerOrderingInfo{}
		info.MaxDecPicBufferingMinus1 = byte(r.ReadExpGolomb())
		info.MaxNumReorderPics = byte(r.ReadExpGolomb())
		info.MaxLatencyIncreasePlus1 = byte(r.ReadExpGolomb())
		vps.SubLayerOrderingInfos = append(vps.SubLayerOrderingInfos, info)
	}

	vps.MaxLayerID = byte(r.Read(6))

	numLayerSetsMinus1 := r.ReadExpGolomb()
	vps.NumLayerSets = uint16(numLayerSetsMinus1) + 1

	// Layer sets
	for i := uint32(1); i < uint32(vps.NumLayerSets); i++ {
		n := 0
		for j := byte(0); j <= vps.MaxLayerID; j++ {
			included := r.ReadFlag()
			if included {
				if i < maxLayers {
					vps.LayerSetLayerIdList[i][n] = j
				}
				n++
			}
		}
		if i < maxLayers {
			vps.NumLayersInIdList[i] = n
		}
	}
	vps.NumLayersInIdList[0] = 1

	// Timing info
	timingInfoPresentFlag := r.ReadFlag()
	if timingInfoPresentFlag {
		_ = r.Read(32) // vps_num_units_in_tick
		_ = r.Read(32) // vps_time_scale
		pocProportionalToTimingFlag := r.ReadFlag()
		if pocProportionalToTimingFlag {
			_ = r.ReadExpGolomb() // vps_num_ticks_poc_diff_one_minus1
		}
		vpsNumHrdParameters := r.ReadExpGolomb()
		for i := uint(0); i < vpsNumHrdParameters; i++ {
			_ = r.ReadExpGolomb() // hrd_layer_set_idx
			if i > 0 {
				cprmsPresentFlag := r.ReadFlag()
				_ = cprmsPresentFlag
			}
			// Skip HRD parameters (complex, not needed for our use case)
			skipHrdParameters(r, true, vps.MaxSubLayersMinus1)
		}
	}

	if r.AccError() != nil {
		return nil, fmt.Errorf("error parsing VPS base: %w", r.AccError())
	}

	extensionFlag := r.ReadFlag()
	if !extensionFlag {
		return vps, nil
	}

	// Align to byte boundary before extension (vps_extension_alignment_bit_equal_to_one)
	nBits := r.NrBitsReadInCurrentByte()
	if nBits > 0 && nBits < 8 {
		_ = r.Read(8 - nBits)
	}

	err := parseVPSExtension(vps, r)
	if err != nil {
		return vps, fmt.Errorf("error parsing VPS extension: %w", err)
	}

	return vps, nil
}

func parseVPSExtension(vps *VPS, r *bits.EBSPReader) error {
	vps.ExtensionFound = true

	if vps.MaxLayersMinus1 > 0 && vps.BaseLayerInternalFlag {
		vps.ExtProfileTierLevels = append(vps.ExtProfileTierLevels,
			parseProfileTierLevel(r, false, vps.MaxSubLayersMinus1))
	}

	// Scalability mask
	splittingFlag := r.ReadFlag()
	vps.NumScalabilityTypes = 0
	for i := 0; i < 16; i++ {
		vps.ScalabilityMask[i] = r.ReadFlag()
		if vps.ScalabilityMask[i] {
			vps.NumScalabilityTypes++
		}
	}
	if vps.NumScalabilityTypes > 16 {
		vps.NumScalabilityTypes = 16
	}

	// Dimension ID lengths
	dimensionIdLen := [16]byte{}
	dimBitOffset := [17]byte{}
	if vps.NumScalabilityTypes > 0 {
		nst := vps.NumScalabilityTypes
		if splittingFlag {
			nst--
		}
		for i := 0; i < nst; i++ {
			dimensionIdLen[i] = byte(r.Read(3)) + 1
		}
		if splittingFlag {
			numBits := byte(0)
			for i := 0; i < vps.NumScalabilityTypes-1; i++ {
				numBits += dimensionIdLen[i]
				dimBitOffset[i+1] = numBits
			}
			dimensionIdLen[vps.NumScalabilityTypes-1] = 6 - numBits
			dimBitOffset[vps.NumScalabilityTypes] = 6
		}
	}

	// Layer ID mapping
	nuhLayerIdPresentFlag := r.ReadFlag()
	vps.LayerIdInNuh[0] = 0
	vps.LayerIdInVps[0] = 0
	for i := byte(1); i <= vps.MaxLayersMinus1; i++ {
		if nuhLayerIdPresentFlag {
			vps.LayerIdInNuh[i] = byte(r.Read(6))
		} else {
			vps.LayerIdInNuh[i] = i
		}
		vps.LayerIdInVps[vps.LayerIdInNuh[i]] = i

		if !splittingFlag {
			for j := 0; j < vps.NumScalabilityTypes; j++ {
				vps.DimensionId[i][j] = byte(r.Read(int(dimensionIdLen[j])))
			}
		}
	}

	// Handle splitting_flag case
	if splittingFlag {
		for i := byte(0); i <= vps.MaxLayersMinus1; i++ {
			for j := 0; j < vps.NumScalabilityTypes; j++ {
				if dimBitOffset[j+1] <= 31 {
					vps.DimensionId[i][j] = (vps.LayerIdInNuh[i] & ((1 << dimBitOffset[j+1]) - 1)) >> dimBitOffset[j]
				}
			}
		}
	}

	// View ID
	viewIdLen := r.Read(4)
	if viewIdLen > 0 {
		numViews := vps.GetNumViews()
		for i := 0; i < numViews; i++ {
			_ = r.Read(int(viewIdLen)) // view_id_val
		}
	}

	// Direct dependency flags
	for i := byte(1); i <= vps.MaxLayersMinus1; i++ {
		for j := byte(0); j < i; j++ {
			vps.DirectDependencyFlag[i][j] = r.ReadFlag()
		}
	}

	// Build reference layer lists
	for i := byte(0); i <= vps.MaxLayersMinus1; i++ {
		iNuhLId := vps.LayerIdInNuh[i]
		d := byte(0)
		for j := byte(0); j <= vps.MaxLayersMinus1; j++ {
			jNuhLId := vps.LayerIdInNuh[j]
			if vps.DirectDependencyFlag[i][j] {
				vps.IdDirectRefLayers[iNuhLId][d] = jNuhLId
				d++
			}
		}
		vps.NumDirectRefLayers[iNuhLId] = d
	}

	// Count independent layers and additional layer sets
	numIndependentLayers := 0
	for i := byte(0); i <= vps.MaxLayersMinus1; i++ {
		iNuhLId := vps.LayerIdInNuh[i]
		if vps.NumDirectRefLayers[iNuhLId] == 0 {
			numIndependentLayers++
		}
	}

	numAddLayerSets := 0
	if numIndependentLayers > 1 {
		numAddLayerSets = int(r.ReadExpGolomb())
	}
	for i := 0; i < numAddLayerSets; i++ {
		for j := 1; j < numIndependentLayers; j++ {
			nbBits := 1
			for (1 << nbBits) < (numIndependentLayers + 1) {
				nbBits++
			}
			_ = r.Read(nbBits) // highest_layer_idx_plus1
		}
	}

	// Sub-layers max
	if r.ReadFlag() { // vps_sub_layers_max_minus1_present_flag
		for i := byte(0); i <= vps.MaxLayersMinus1; i++ {
			vps.SubLayersVpsMaxMinus1[i] = byte(r.Read(3))
		}
	} else {
		for i := byte(0); i <= vps.MaxLayersMinus1; i++ {
			vps.SubLayersVpsMaxMinus1[i] = vps.MaxSubLayersMinus1
		}
	}

	// Max TID ref
	if r.ReadFlag() { // max_tid_ref_present_flag
		for i := byte(0); i < vps.MaxLayersMinus1; i++ {
			for j := i + 1; j <= vps.MaxLayersMinus1; j++ {
				if vps.DirectDependencyFlag[j][i] {
					_ = r.Read(3) // max_tid_il_ref_pics_plus1
				}
			}
		}
	}

	_ = r.ReadFlag() // default_ref_layers_active_flag

	// Profile tier level for extension layers
	vps.NumProfileTierLevel = int(r.ReadExpGolomb()) + 1
	startIdx := 1
	if vps.BaseLayerInternalFlag {
		startIdx = 2
	}
	for i := startIdx; i < vps.NumProfileTierLevel; i++ {
		profilePresentFlag := r.ReadFlag()
		ptl := parseProfileTierLevel(r, profilePresentFlag, vps.MaxSubLayersMinus1)
		vps.ExtProfileTierLevels = append(vps.ExtProfileTierLevels, ptl)
	}

	// Output layer sets
	numLayerSets := int(vps.NumLayerSets) + numAddLayerSets
	numAddOlss := 0
	defaultOutputLayerIDC := 0
	if numLayerSets > 1 {
		numAddOlss = int(r.ReadExpGolomb())
		defaultOutputLayerIDC = int(r.Read(2))
		if defaultOutputLayerIDC > 2 {
			defaultOutputLayerIDC = 2
		}
	}
	vps.NumOutputLayerSets = numAddOlss + numLayerSets

	// Compute dependency_flag (transitive closure)
	var dependencyFlag [maxLayers][maxLayers]bool
	for i := byte(0); i <= vps.MaxLayersMinus1; i++ {
		for j := byte(0); j <= vps.MaxLayersMinus1; j++ {
			dependencyFlag[i][j] = vps.DirectDependencyFlag[i][j]
			for k := byte(0); k < i; k++ {
				if vps.DirectDependencyFlag[i][k] && dependencyFlag[k][j] {
					dependencyFlag[i][j] = true
				}
			}
		}
	}

	for i := 0; i < vps.NumOutputLayerSets; i++ {
		olsIdxToLsIdx := i
		if i >= numLayerSets {
			nbBits := 1
			for (1 << nbBits) < (numLayerSets - 1) {
				nbBits++
			}
			lsIdx := int(r.Read(nbBits))
			olsIdxToLsIdx = lsIdx + 1
		}

		if i > int(vps.NumLayerSets)-1 || defaultOutputLayerIDC == 2 {
			numLayers := vps.NumLayersInIdList[olsIdxToLsIdx]
			for j := 0; j < numLayers; j++ {
				if i < maxLayers && j < maxLayers {
					vps.OutputLayerFlag[i][j] = r.ReadFlag()
				} else {
					_ = r.ReadFlag()
				}
			}
		}

		// Compute output layer flags for default modes
		if defaultOutputLayerIDC == 0 || defaultOutputLayerIDC == 1 {
			numLayers := vps.NumLayersInIdList[olsIdxToLsIdx]
			for j := 0; j < numLayers; j++ {
				if i < maxLayers && j < maxLayers {
					if defaultOutputLayerIDC == 0 {
						vps.OutputLayerFlag[i][j] = true
					} else {
						// Only highest layer is output
						vps.OutputLayerFlag[i][j] = (j == numLayers-1)
					}
				}
			}
		}

		// Compute necessary layers
		numLayersInOls := vps.NumLayersInIdList[olsIdxToLsIdx]
		for j := 0; j < numLayersInOls; j++ {
			if i < maxLayers && j < maxLayers && vps.OutputLayerFlag[i][j] {
				vps.NecessaryLayersFlag[i][j] = true
				curLayerID := vps.LayerSetLayerIdList[olsIdxToLsIdx][j]
				for k := 0; k < j; k++ {
					refLayerID := vps.LayerSetLayerIdList[olsIdxToLsIdx][k]
					if dependencyFlag[vps.LayerIdInVps[curLayerID]][vps.LayerIdInVps[refLayerID]] {
						vps.NecessaryLayersFlag[i][k] = true
					}
				}
			}
		}
		vps.NumNecessaryLayers[i] = 0
		for j := 0; j < numLayersInOls; j++ {
			if i < maxLayers && j < maxLayers && vps.NecessaryLayersFlag[i][j] {
				vps.NumNecessaryLayers[i]++
			}
		}

		if i == 0 {
			if vps.BaseLayerInternalFlag {
				if vps.MaxLayersMinus1 > 0 {
					vps.ProfileTierLevelIdx[0][0] = 1
				}
			}
			continue
		}

		// Profile tier level indices
		nbBits := 1
		for (1 << nbBits) < vps.NumProfileTierLevel {
			nbBits++
		}
		for j := 0; j < numLayersInOls; j++ {
			if i < maxLayers && j < maxLayers {
				if vps.NecessaryLayersFlag[i][j] && vps.NumProfileTierLevel > 0 {
					vps.ProfileTierLevelIdx[i][j] = byte(r.Read(nbBits))
				}
			}
		}
	}

	// Rep formats
	vps.NumRepFormats = int(r.ReadExpGolomb()) + 1
	for i := 0; i < vps.NumRepFormats; i++ {
		rf := parseRepFormat(r)
		vps.RepFormats = append(vps.RepFormats, rf)
	}

	repFormatIdxPresentFlag := false
	if vps.NumRepFormats > 1 {
		repFormatIdxPresentFlag = r.ReadFlag()
	}

	vps.RepFormatIdx[0] = 0
	nbBits := 1
	for (1 << nbBits) < vps.NumRepFormats {
		nbBits++
	}
	startLayer := byte(1)
	if !vps.BaseLayerInternalFlag {
		startLayer = 0
	}
	for i := startLayer; i <= vps.MaxLayersMinus1; i++ {
		if repFormatIdxPresentFlag {
			vps.RepFormatIdx[i] = byte(r.Read(nbBits))
		} else {
			if int(i) < vps.NumRepFormats {
				vps.RepFormatIdx[i] = i
			} else {
				vps.RepFormatIdx[i] = byte(vps.NumRepFormats - 1)
			}
		}
	}

	return r.AccError()
}

func parseRepFormat(r *bits.EBSPReader) RepFormat {
	rf := RepFormat{}
	rf.PicWidthLumaSamples = uint16(r.Read(16))
	rf.PicHeightLumaSamples = uint16(r.Read(16))
	chromaAndBitDepthPresentFlag := r.ReadFlag()
	if chromaAndBitDepthPresentFlag {
		rf.ChromaFormatIDC = byte(r.Read(2))
		if rf.ChromaFormatIDC == 3 {
			_ = r.ReadFlag() // separate_colour_plane_flag
		}
		rf.BitDepthLuma = byte(r.Read(4)) + 8
		rf.BitDepthChroma = byte(r.Read(4)) + 8
	}
	conformanceWindowPresentFlag := r.ReadFlag()
	if conformanceWindowPresentFlag {
		_ = r.ReadExpGolomb() // left
		_ = r.ReadExpGolomb() // right
		_ = r.ReadExpGolomb() // top
		_ = r.ReadExpGolomb() // bottom
	}
	return rf
}

func skipHrdParameters(r *bits.EBSPReader, commonInfPresentFlag bool, maxNumSubLayersMinus1 byte) {
	nalHrdParametersPresentFlag := false
	vclHrdParametersPresentFlag := false
	subPicHrdParamsPresentFlag := false

	if commonInfPresentFlag {
		nalHrdParametersPresentFlag = r.ReadFlag()
		vclHrdParametersPresentFlag = r.ReadFlag()
		if nalHrdParametersPresentFlag || vclHrdParametersPresentFlag {
			subPicHrdParamsPresentFlag = r.ReadFlag()
			if subPicHrdParamsPresentFlag {
				_ = r.Read(8)  // tick_divisor_minus2
				_ = r.Read(5)  // du_cpb_removal_delay_increment_length_minus1
				_ = r.ReadFlag() // sub_pic_cpb_params_in_pic_timing_sei_flag
				_ = r.Read(5)  // dpb_output_delay_du_length_minus1
			}
			_ = r.Read(4) // bit_rate_scale
			_ = r.Read(4) // cpb_size_scale
			if subPicHrdParamsPresentFlag {
				_ = r.Read(4) // cpb_size_du_scale
			}
			_ = r.Read(5) // initial_cpb_removal_delay_length_minus1
			_ = r.Read(5) // au_cpb_removal_delay_length_minus1
			_ = r.Read(5) // dpb_output_delay_length_minus1
		}
	}

	for i := byte(0); i <= maxNumSubLayersMinus1; i++ {
		fixedPicRateGeneralFlag := r.ReadFlag()
		fixedPicRateWithinCvsFlag := false
		if !fixedPicRateGeneralFlag {
			fixedPicRateWithinCvsFlag = r.ReadFlag()
		}
		lowDelayHrdFlag := false
		if fixedPicRateGeneralFlag || fixedPicRateWithinCvsFlag {
			_ = r.ReadExpGolomb() // elemental_duration_in_tc_minus1
		} else {
			lowDelayHrdFlag = r.ReadFlag()
		}
		cpbCntMinus1 := uint(0)
		if !lowDelayHrdFlag {
			cpbCntMinus1 = r.ReadExpGolomb()
		}
		if nalHrdParametersPresentFlag {
			skipSubLayerHrdParameters(r, cpbCntMinus1, subPicHrdParamsPresentFlag)
		}
		if vclHrdParametersPresentFlag {
			skipSubLayerHrdParameters(r, cpbCntMinus1, subPicHrdParamsPresentFlag)
		}
	}
}

func skipSubLayerHrdParameters(r *bits.EBSPReader, cpbCntMinus1 uint, subPicHrdParams bool) {
	for i := uint(0); i <= cpbCntMinus1; i++ {
		_ = r.ReadExpGolomb() // bit_rate_value_minus1
		_ = r.ReadExpGolomb() // cpb_size_value_minus1
		if subPicHrdParams {
			_ = r.ReadExpGolomb() // cpb_size_du_value_minus1
			_ = r.ReadExpGolomb() // bit_rate_du_value_minus1
		}
		_ = r.ReadFlag() // cbr_flag
	}
}
