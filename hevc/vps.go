package hevc

import (
	"bytes"
	"fmt"

	"github.com/Eyevinn/mp4ff/bits"
)

// maxLayers is the maximum number of layers supported when parsing the
// multilayer VPS extension. MV-HEVC stereo uses 2 layers; the fixed-size
// extension arrays are dimensioned for this bound.
const maxLayers = 8

// VPS is HEVC VPS parameters
// ISO/IEC 23008-2 (Ed. 5) Sec. 7.3.2.1 page 47 and 7.4.3.1 page 92
type VPS struct {
	VpsID                           byte
	BaseLayerInternalFlag           bool
	BaseLayerAvailableFlag          bool
	MaxLayersMinus1                 byte
	MaxSubLayersMinus1              byte
	TemporalIdNestingFlag           bool
	ProfileTierLevel                ProfileTierLevel
	SubLayerOrderingInfoPresentFlag bool
	MaxDecPicBufferingMinus1        []uint
	MaxNumReorderPics               []uint
	MaxLatencyIncreasePlus1         []uint
	MaxLayerID                      byte
	NumLayerSetsMinus1              uint
	TimingInfoPresentFlag           bool
	TimingInfo                      *VPSTimingInfo
	ExtensionFlag                   bool
	// Extension holds the multilayer/multiview (MV-HEVC, SHVC) parameters from
	// vps_extension(). It is non-nil only when ExtensionFlag is set and the
	// extension was parsed. See ISO/IEC 23008-2 (Ed. 5) Annex F.7.3.2.1.1.
	Extension *VPSExtension
}

// VPSTimingInfo contains VPS timing info parameters.
type VPSTimingInfo struct {
	NumUnitsInTick              uint32
	TimeScale                   uint32
	PocProportionalToTimingFlag bool
	NumTicksPocDiffOneMinus1    uint
	NumHrdParameters            uint
	HrdParameters               []*HrdParameters
}

// VPSExtension contains the multilayer/multiview parameters parsed from
// vps_extension(). See ISO/IEC 23008-2 (Ed. 5) Annex F.7.3.2.1.1.
type VPSExtension struct {
	ScalabilityMask      [16]bool
	NumScalabilityTypes  int
	LayerIdInNuh         [maxLayers]byte
	LayerIdInVps         [64]byte // reverse mapping: nuh_layer_id -> vps layer index
	DimensionId          [maxLayers][16]byte
	DirectDependencyFlag [maxLayers][maxLayers]bool
	NumDirectRefLayers   [64]byte
	IdDirectRefLayers    [64][maxLayers]byte

	NumProfileTierLevel  int
	ExtProfileTierLevels []ProfileTierLevel // ext_ptl[0], ext_ptl[1], ...
	NumOutputLayerSets   int
	OutputLayerFlag      [maxLayers][maxLayers]bool
	ProfileTierLevelIdx  [maxLayers][maxLayers]byte
	NumNecessaryLayers   [maxLayers]int
	NecessaryLayersFlag  [maxLayers][maxLayers]bool

	NumLayersInIdList   [maxLayers]int
	LayerSetLayerIdList [maxLayers][maxLayers]byte

	NumRepFormats int
	RepFormats    []RepFormat
	RepFormatIdx  [maxLayers]byte

	SubLayersVpsMaxMinus1 [maxLayers]byte
}

// RepFormat holds resolution and format info for a layer (rep_format() in
// ISO/IEC 23008-2 Annex F.7.3.2.1.2).
type RepFormat struct {
	PicWidthLumaSamples  uint16
	PicHeightLumaSamples uint16
	ChromaFormatIDC      byte
	BitDepthLuma         byte
	BitDepthChroma       byte
}

// GetNumLayers returns the number of layers signalled by this VPS.
func (v *VPS) GetNumLayers() int {
	return int(v.MaxLayersMinus1) + 1
}

// IsMultiLayer returns true if this VPS describes a multi-layer bitstream.
func (v *VPS) IsMultiLayer() bool {
	return v.MaxLayersMinus1 > 0 && v.Extension != nil
}

// GetNumViews returns the number of unique views in the multi-view configuration.
func (v *VPS) GetNumViews() int {
	if v.Extension == nil {
		return 1
	}
	numViews := 1
	for i := 1; i <= int(v.MaxLayersMinus1); i++ {
		viewIdx := v.Extension.getViewIndex(i)
		newView := true
		for j := 0; j < i; j++ {
			if v.Extension.getViewIndex(j) == viewIdx {
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

// ScalabilityMaskBits returns the scalability mask as a 16-bit value, or 0 if
// there is no VPS extension.
func (v *VPS) ScalabilityMaskBits() uint16 {
	if v.Extension == nil {
		return 0
	}
	var mask uint16
	for i := 0; i < 16; i++ {
		if v.Extension.ScalabilityMask[i] {
			mask |= 1 << i
		}
	}
	return mask
}

// getViewIndex returns the view order index for a layer (VPS index).
// The view order index is scalability dimension type 1 (view order index).
func (e *VPSExtension) getViewIndex(layerIdx int) byte {
	dimIdx := 0
	for j := 0; j < 16; j++ {
		if e.ScalabilityMask[j] {
			if j == 1 { // view_order_index is scalability type 1
				return e.DimensionId[layerIdx][dimIdx]
			}
			dimIdx++
		}
	}
	return 0
}

// ParseVPSNALUnit parses HEVC VPS NAL unit starting with NAL unit header.
func ParseVPSNALUnit(data []byte) (*VPS, error) {
	vps := &VPS{}

	rd := bytes.NewReader(data)
	r := bits.NewEBSPReader(rd)
	// Note! First two bytes are NALU Header

	naluHdrBits := r.Read(16)
	naluType := GetNaluType(byte(naluHdrBits >> 8))
	if naluType != NALU_VPS {
		return nil, fmt.Errorf("NALU type is %s, not VPS", naluType)
	}
	vps.VpsID = byte(r.Read(4))
	vps.BaseLayerInternalFlag = r.ReadFlag()
	vps.BaseLayerAvailableFlag = r.ReadFlag()
	vps.MaxLayersMinus1 = byte(r.Read(6))
	vps.MaxSubLayersMinus1 = byte(r.Read(3))
	vps.TemporalIdNestingFlag = r.ReadFlag()
	_ = r.Read(16) // vps_reserved_0xffff_16bits

	vps.ProfileTierLevel = parseProfileTierLevel(r, true, vps.MaxSubLayersMinus1)

	if r.AccError() != nil {
		return nil, fmt.Errorf("error parsing VPS profile_tier_level: %w", r.AccError())
	}

	vps.SubLayerOrderingInfoPresentFlag = r.ReadFlag()
	start := uint(vps.MaxSubLayersMinus1)
	if vps.SubLayerOrderingInfoPresentFlag {
		start = 0
	}
	nrEntries := uint(vps.MaxSubLayersMinus1) + 1
	vps.MaxDecPicBufferingMinus1 = make([]uint, nrEntries)
	vps.MaxNumReorderPics = make([]uint, nrEntries)
	vps.MaxLatencyIncreasePlus1 = make([]uint, nrEntries)
	for i := start; i <= uint(vps.MaxSubLayersMinus1); i++ {
		vps.MaxDecPicBufferingMinus1[i] = r.ReadExpGolomb()
		vps.MaxNumReorderPics[i] = r.ReadExpGolomb()
		vps.MaxLatencyIncreasePlus1[i] = r.ReadExpGolomb()
	}

	vps.MaxLayerID = byte(r.Read(6))
	vps.NumLayerSetsMinus1 = r.ReadExpGolomb()
	// Capture layer set membership for use by the VPS extension (if present).
	var numLayersInIdList [maxLayers]int
	var layerSetLayerIdList [maxLayers][maxLayers]byte
	numLayersInIdList[0] = 1
	for i := uint(1); i <= vps.NumLayerSetsMinus1; i++ {
		n := 0
		for j := byte(0); j <= vps.MaxLayerID; j++ {
			if r.ReadFlag() { // layer_id_included_flag[i][j]
				if i < maxLayers && n < maxLayers {
					layerSetLayerIdList[i][n] = j
				}
				n++
			}
		}
		if i < maxLayers {
			numLayersInIdList[i] = n
		}
	}

	vps.TimingInfoPresentFlag = r.ReadFlag()
	if vps.TimingInfoPresentFlag {
		ti := &VPSTimingInfo{}
		ti.NumUnitsInTick = uint32(r.Read(32))
		ti.TimeScale = uint32(r.Read(32))
		ti.PocProportionalToTimingFlag = r.ReadFlag()
		if ti.PocProportionalToTimingFlag {
			ti.NumTicksPocDiffOneMinus1 = r.ReadExpGolomb()
		}
		ti.NumHrdParameters = r.ReadExpGolomb()
		if ti.NumHrdParameters > 0 {
			ti.HrdParameters = make([]*HrdParameters, ti.NumHrdParameters)
			for i := uint(0); i < ti.NumHrdParameters; i++ {
				_ = r.ReadExpGolomb() // hrd_layer_set_idx[i]
				cprmsPresentFlag := true
				if i > 0 {
					cprmsPresentFlag = r.ReadFlag()
				}
				ti.HrdParameters[i] = parseHrdParameters(r, cprmsPresentFlag, vps.MaxSubLayersMinus1)
			}
		}
		vps.TimingInfo = ti
	}

	if r.AccError() != nil {
		return nil, fmt.Errorf("error parsing VPS: %w", r.AccError())
	}

	vps.ExtensionFlag = r.ReadFlag()
	if !vps.ExtensionFlag || r.AccError() != nil {
		return vps, nil
	}

	// Byte-align before the extension (vps_extension_alignment_bit_equal_to_one).
	if nBits := r.NrBitsReadInCurrentByte(); nBits > 0 && nBits < 8 {
		_ = r.Read(8 - nBits)
	}

	ext := &VPSExtension{
		NumLayersInIdList:   numLayersInIdList,
		LayerSetLayerIdList: layerSetLayerIdList,
	}
	vps.Extension = ext
	if err := parseVPSExtension(vps, ext, r); err != nil {
		return vps, fmt.Errorf("error parsing VPS extension: %w", err)
	}

	return vps, nil
}

// parseVPSExtension parses vps_extension() into ext.
// ISO/IEC 23008-2 (Ed. 5) Annex F.7.3.2.1.1.
func parseVPSExtension(vps *VPS, ext *VPSExtension, r *bits.EBSPReader) error {
	if int(vps.MaxLayersMinus1) >= maxLayers {
		return fmt.Errorf("VPS extension with %d layers exceeds supported maximum %d",
			vps.MaxLayersMinus1+1, maxLayers)
	}

	if vps.MaxLayersMinus1 > 0 && vps.BaseLayerInternalFlag {
		ext.ExtProfileTierLevels = append(ext.ExtProfileTierLevels,
			parseProfileTierLevel(r, false, vps.MaxSubLayersMinus1))
	}

	// Scalability mask
	splittingFlag := r.ReadFlag()
	for i := 0; i < 16; i++ {
		ext.ScalabilityMask[i] = r.ReadFlag()
		if ext.ScalabilityMask[i] {
			ext.NumScalabilityTypes++
		}
	}

	// Dimension ID bit lengths
	var dimensionIDLen [16]byte
	var dimBitOffset [17]byte
	if ext.NumScalabilityTypes > 0 {
		nst := ext.NumScalabilityTypes
		if splittingFlag {
			nst--
		}
		for i := 0; i < nst; i++ {
			dimensionIDLen[i] = byte(r.Read(3)) + 1
		}
		if splittingFlag {
			numBits := byte(0)
			for i := 0; i < ext.NumScalabilityTypes-1; i++ {
				numBits += dimensionIDLen[i]
				dimBitOffset[i+1] = numBits
			}
			dimensionIDLen[ext.NumScalabilityTypes-1] = 6 - numBits
			dimBitOffset[ext.NumScalabilityTypes] = 6
		}
	}

	// Layer ID mapping and dimension IDs
	nuhLayerIDPresentFlag := r.ReadFlag()
	for i := byte(1); i <= vps.MaxLayersMinus1; i++ {
		if nuhLayerIDPresentFlag {
			ext.LayerIdInNuh[i] = byte(r.Read(6))
		} else {
			ext.LayerIdInNuh[i] = i
		}
		ext.LayerIdInVps[ext.LayerIdInNuh[i]] = i

		if !splittingFlag {
			for j := 0; j < ext.NumScalabilityTypes; j++ {
				ext.DimensionId[i][j] = byte(r.Read(int(dimensionIDLen[j])))
			}
		}
	}
	if splittingFlag {
		for i := byte(0); i <= vps.MaxLayersMinus1; i++ {
			for j := 0; j < ext.NumScalabilityTypes; j++ {
				if dimBitOffset[j+1] <= 31 {
					ext.DimensionId[i][j] = (ext.LayerIdInNuh[i] & ((1 << dimBitOffset[j+1]) - 1)) >> dimBitOffset[j]
				}
			}
		}
	}

	// View ID
	viewIDLen := r.Read(4)
	if viewIDLen > 0 {
		numViews := vps.GetNumViews()
		for i := 0; i < numViews; i++ {
			_ = r.Read(int(viewIDLen)) // view_id_val[i]
		}
	}

	// Direct dependency flags
	for i := byte(1); i <= vps.MaxLayersMinus1; i++ {
		for j := byte(0); j < i; j++ {
			ext.DirectDependencyFlag[i][j] = r.ReadFlag()
		}
	}

	// Build direct reference layer lists
	for i := byte(0); i <= vps.MaxLayersMinus1; i++ {
		iNuhLID := ext.LayerIdInNuh[i]
		d := byte(0)
		for j := byte(0); j <= vps.MaxLayersMinus1; j++ {
			jNuhLID := ext.LayerIdInNuh[j]
			if ext.DirectDependencyFlag[i][j] {
				ext.IdDirectRefLayers[iNuhLID][d] = jNuhLID
				d++
			}
		}
		ext.NumDirectRefLayers[iNuhLID] = d
	}

	// Count independent layers, then additional layer sets
	numIndependentLayers := 0
	for i := byte(0); i <= vps.MaxLayersMinus1; i++ {
		if ext.NumDirectRefLayers[ext.LayerIdInNuh[i]] == 0 {
			numIndependentLayers++
		}
	}
	numAddLayerSets := 0
	if numIndependentLayers > 1 {
		numAddLayerSets = int(r.ReadExpGolomb())
	}
	// Additional layer sets require the tree-partition (F-6) and layer-set (F-9)
	// derivations to read highest_layer_idx_plus1[i][j] with the correct per-tree
	// width and to populate the added layer sets. They never occur for MV-HEVC
	// (which has a single independent layer); refuse rather than misparse.
	if numAddLayerSets > 0 {
		return fmt.Errorf("VPS extension with %d additional layer sets is not supported", numAddLayerSets)
	}

	// Sub-layers max
	if r.ReadFlag() { // vps_sub_layers_max_minus1_present_flag
		for i := byte(0); i <= vps.MaxLayersMinus1; i++ {
			ext.SubLayersVpsMaxMinus1[i] = byte(r.Read(3))
		}
	} else {
		for i := byte(0); i <= vps.MaxLayersMinus1; i++ {
			ext.SubLayersVpsMaxMinus1[i] = vps.MaxSubLayersMinus1
		}
	}

	// Max TID reference present
	if r.ReadFlag() { // max_tid_ref_present_flag
		for i := byte(0); i < vps.MaxLayersMinus1; i++ {
			for j := i + 1; j <= vps.MaxLayersMinus1; j++ {
				if ext.DirectDependencyFlag[j][i] {
					_ = r.Read(3) // max_tid_il_ref_pics_plus1[i][j]
				}
			}
		}
	}

	_ = r.ReadFlag() // default_ref_layers_active_flag

	// Profile tier levels for extension layers
	ext.NumProfileTierLevel = int(r.ReadExpGolomb()) + 1
	startIdx := 1
	if vps.BaseLayerInternalFlag {
		startIdx = 2
	}
	for i := startIdx; i < ext.NumProfileTierLevel; i++ {
		profilePresentFlag := r.ReadFlag()
		ext.ExtProfileTierLevels = append(ext.ExtProfileTierLevels,
			parseProfileTierLevel(r, profilePresentFlag, vps.MaxSubLayersMinus1))
	}

	// Output layer sets
	numLayerSets := int(vps.NumLayerSetsMinus1) + 1 + numAddLayerSets
	numAddOlss := 0
	defaultOutputLayerIDC := 0
	if numLayerSets > 1 {
		numAddOlss = int(r.ReadExpGolomb())
		defaultOutputLayerIDC = int(r.Read(2))
		if defaultOutputLayerIDC > 2 {
			defaultOutputLayerIDC = 2
		}
	}
	ext.NumOutputLayerSets = numAddOlss + numLayerSets

	// Transitive dependency closure
	var dependencyFlag [maxLayers][maxLayers]bool
	for i := byte(0); i <= vps.MaxLayersMinus1; i++ {
		for j := byte(0); j <= vps.MaxLayersMinus1; j++ {
			dependencyFlag[i][j] = ext.DirectDependencyFlag[i][j]
			for k := byte(0); k < i; k++ {
				if ext.DirectDependencyFlag[i][k] && dependencyFlag[k][j] {
					dependencyFlag[i][j] = true
				}
			}
		}
	}

	// The OLS loop only reads syntax elements for i >= 1; the i == 0 case (the
	// base layer set) reads nothing. See the for(i=1;...) loop in F.7.3.2.1.1.
	// NumOutputLayerSets may be far larger than maxLayers (num_add_olss can be up
	// to 1023), so per-OLS state is computed in locals indexed by the in-set layer
	// index j and only persisted into ext for the first maxLayers OLSs.
	for i := 0; i < ext.NumOutputLayerSets; i++ {
		olsIdxToLsIdx := i
		if i >= numLayerSets {
			// layer_set_idx_for_ols_minus1[i] is only signalled when NumLayerSets > 2,
			// otherwise it is inferred to be 0 (F-11).
			if numLayerSets > 2 {
				nbBits := ceilLog2(numLayerSets - 1)
				olsIdxToLsIdx = int(r.Read(nbBits)) + 1
			} else {
				olsIdxToLsIdx = 1
			}
		}
		if olsIdxToLsIdx >= maxLayers {
			return fmt.Errorf("VPS extension layer set index %d exceeds supported maximum %d",
				olsIdxToLsIdx, maxLayers)
		}
		numLayersInOls := ext.NumLayersInIdList[olsIdxToLsIdx]

		var outputLayerFlag [maxLayers]bool

		// output_layer_flag[i][j] is signalled only for i >= 1 when
		// i > vps_num_layer_sets_minus1 or defaultOutputLayerIdc == 2.
		signalled := i >= 1 && (i > int(vps.NumLayerSetsMinus1) || defaultOutputLayerIDC == 2)
		switch {
		case signalled:
			for j := 0; j < numLayersInOls; j++ {
				flag := r.ReadFlag()
				if j < maxLayers {
					outputLayerFlag[j] = flag
				}
			}
		case i <= int(vps.NumLayerSetsMinus1) && (defaultOutputLayerIDC == 0 || defaultOutputLayerIDC == 1):
			// Derived for default modes 0/1: a layer is output if idc == 0, or it is
			// the highest layer in the layer set (which is the last entry, since
			// LayerSetLayerIdList is filled in increasing nuh_layer_id order).
			for j := 0; j < numLayersInOls && j < maxLayers; j++ {
				if defaultOutputLayerIDC == 0 || j == numLayersInOls-1 {
					outputLayerFlag[j] = true
				}
			}
		}

		if i == 0 {
			// OLS 0 is layer set 0, which contains only the base layer. No bits are
			// read for it, but its derived state is still defined (F-12/F-13): the
			// single base layer is the inferred output layer and is necessary.
			ext.OutputLayerFlag[0][0] = true
			ext.NecessaryLayersFlag[0][0] = true
			ext.NumNecessaryLayers[0] = 1
			if vps.BaseLayerInternalFlag && vps.MaxLayersMinus1 > 0 {
				ext.ProfileTierLevelIdx[0][0] = 1
			}
			continue
		}

		// Derive NumOutputLayersInOutputLayerSet and OlsHighestOutputLayerId (F-12).
		numOutputLayers := 0
		var highestOutputLayerID byte
		for j := 0; j < numLayersInOls && j < maxLayers; j++ {
			if outputLayerFlag[j] {
				numOutputLayers++
				highestOutputLayerID = ext.LayerSetLayerIdList[olsIdxToLsIdx][j]
			}
		}

		// Derive NecessaryLayerFlag / NumNecessaryLayers (F-13).
		var necessaryLayerFlag [maxLayers]bool
		for j := 0; j < numLayersInOls && j < maxLayers; j++ {
			if outputLayerFlag[j] {
				necessaryLayerFlag[j] = true
				curLayerID := ext.LayerSetLayerIdList[olsIdxToLsIdx][j]
				for k := 0; k < j; k++ {
					refLayerID := ext.LayerSetLayerIdList[olsIdxToLsIdx][k]
					if dependencyFlag[ext.LayerIdInVps[curLayerID]][ext.LayerIdInVps[refLayerID]] {
						necessaryLayerFlag[k] = true
					}
				}
			}
		}

		// profile_tier_level_idx[i][j] is read only when vps_num_profile_tier_level_minus1 > 0.
		if ext.NumProfileTierLevel > 1 {
			nbBits := ceilLog2(ext.NumProfileTierLevel)
			for j := 0; j < numLayersInOls && j < maxLayers; j++ {
				if necessaryLayerFlag[j] {
					val := byte(r.Read(nbBits))
					if i < maxLayers {
						ext.ProfileTierLevelIdx[i][j] = val
					}
				}
			}
		}

		// alt_output_layer_flag[i]
		if numOutputLayers == 1 && ext.NumDirectRefLayers[highestOutputLayerID] > 0 {
			_ = r.ReadFlag()
		}

		// Persist derived per-OLS state for the OLSs that fit in the fixed arrays.
		if i < maxLayers {
			numNecessary := 0
			for j := 0; j < numLayersInOls && j < maxLayers; j++ {
				ext.OutputLayerFlag[i][j] = outputLayerFlag[j]
				ext.NecessaryLayersFlag[i][j] = necessaryLayerFlag[j]
				if necessaryLayerFlag[j] {
					numNecessary++
				}
			}
			ext.NumNecessaryLayers[i] = numNecessary
		}
	}

	// Rep formats
	ext.NumRepFormats = int(r.ReadExpGolomb()) + 1
	for i := 0; i < ext.NumRepFormats; i++ {
		ext.RepFormats = append(ext.RepFormats, parseRepFormat(r))
	}

	repFormatIDXPresentFlag := false
	if ext.NumRepFormats > 1 {
		repFormatIDXPresentFlag = r.ReadFlag()
	}
	nbBits := ceilLog2(ext.NumRepFormats)
	startLayer := byte(1)
	if !vps.BaseLayerInternalFlag {
		startLayer = 0
	}
	for i := startLayer; i <= vps.MaxLayersMinus1; i++ {
		if repFormatIDXPresentFlag {
			ext.RepFormatIdx[i] = byte(r.Read(nbBits))
		} else if int(i) < ext.NumRepFormats {
			ext.RepFormatIdx[i] = i
		} else {
			ext.RepFormatIdx[i] = byte(ext.NumRepFormats - 1)
		}
	}

	return r.AccError()
}

// parseRepFormat parses rep_format() (ISO/IEC 23008-2 Annex F.7.3.2.1.2).
func parseRepFormat(r *bits.EBSPReader) RepFormat {
	rf := RepFormat{}
	rf.PicWidthLumaSamples = uint16(r.Read(16))
	rf.PicHeightLumaSamples = uint16(r.Read(16))
	if r.ReadFlag() { // chroma_and_bit_depth_vps_present_flag
		rf.ChromaFormatIDC = byte(r.Read(2))
		if rf.ChromaFormatIDC == 3 {
			_ = r.ReadFlag() // separate_colour_plane_vps_flag
		}
		rf.BitDepthLuma = byte(r.Read(4)) + 8
		rf.BitDepthChroma = byte(r.Read(4)) + 8
	}
	if r.ReadFlag() { // conformance_window_vps_flag
		_ = r.ReadExpGolomb() // conf_win_vps_left_offset
		_ = r.ReadExpGolomb() // conf_win_vps_right_offset
		_ = r.ReadExpGolomb() // conf_win_vps_top_offset
		_ = r.ReadExpGolomb() // conf_win_vps_bottom_offset
	}
	return rf
}

// ceilLog2 returns Ceil(Log2(x)), i.e. the smallest n >= 0 with (1 << n) >= x.
// It is used for the variable-length u(v) fields in vps_extension().
func ceilLog2(x int) int {
	n := 0
	for (1 << n) < x {
		n++
	}
	return n
}
