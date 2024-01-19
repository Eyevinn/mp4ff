package hevc

import (
	"bytes"
	"fmt"

	"github.com/Eyevinn/mp4ff/bits"
)

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
	MaxLayerID                      byte
	NumLayerSetsMinus1              uint32
	TimingInfoPresentFlag           bool
	NumUnitsInTick                  uint32
	TimeScale                       uint32
	PocProportionalToTimingFlag     bool
	NumTicksPocDiffOneMinus1        uint32
	NumHrdParameters                uint32
	HrdParameters                   *HrdParameters
	ExtensionFlag                   bool
	Extension                       *VPSExtension
}

// ParseVPSNALUnit parses HEVC VPS NAL unit starting with NAL unit header.
// The parsing is not complete. Sublayers are silently ignored.
func ParseVPSNALUnit(data []byte) (*VPS, error) {

	vps := &VPS{}

	rd := bytes.NewReader(data)
	r := bits.NewAccErrEBSPReader(rd)
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
	vps.SubLayerOrderingInfoPresentFlag = r.ReadFlag()
	start := vps.MaxSubLayersMinus1
	if vps.SubLayerOrderingInfoPresentFlag {
		start = 0
	}
	for i := start; i <= vps.MaxSubLayersMinus1; i++ {
		_ = r.ReadExpGolomb() // vps_max_dec_pic_buffering_minus1[i]
		_ = r.ReadExpGolomb() // vps_max_num_reorder_pics[i]
		_ = r.ReadExpGolomb() // vps_max_latency_increase_plus1[i]
	}
	vps.MaxLayerID = byte(r.Read(6))
	vps.NumLayerSetsMinus1 = uint32(r.ReadExpGolomb())
	for i := uint32(1); i <= vps.NumLayerSetsMinus1; i++ {
		for j := uint32(0); j <= uint32(vps.MaxLayerID); j++ {
			_ = r.ReadFlag() // layer_id_included_flag[i][j]
		}
	}
	vps.TimingInfoPresentFlag = r.ReadFlag()
	if vps.TimingInfoPresentFlag {
		vps.NumUnitsInTick = uint32(r.Read(32))
		vps.TimeScale = uint32(r.Read(32))
		vps.PocProportionalToTimingFlag = r.ReadFlag()
		if vps.PocProportionalToTimingFlag {
			vps.NumTicksPocDiffOneMinus1 = uint32(r.ReadExpGolomb())
		}
		vps.NumHrdParameters = uint32(r.ReadExpGolomb())
		for i := uint32(0); i < vps.NumHrdParameters; i++ {
			_ = r.ReadExpGolomb() // hrd_layer_set_idx[i]
			cprmsPresentFlag := true
			if i > 0 {
				cprmsPresentFlag = r.ReadFlag()
			}
			if cprmsPresentFlag {
				vps.HrdParameters = parseHrdParameters(r, cprmsPresentFlag, vps.MaxSubLayersMinus1)
			}
		}
	}
	vps.ExtensionFlag = r.ReadFlag()
	if vps.ExtensionFlag {
		for {
			more, err := r.MoreRbspData()
			if err != nil {
				return nil, err
			}
			if !more {
				break
			}

			ve, err := vps.parseVPSExtension(r)
			if err != nil {
				return nil, err
			}
			vps.Extension = ve
		}
	}
	err := r.ReadRbspTrailingBits()
	if err != nil {
		return nil, err
	}

	return vps, r.AccError()
}

type VPSExtension struct {
	ProfileTierLevel              ProfileTierLevel
	SplittingFlag                 bool
	ScalabilityMaskFlags          [16]bool
	DimensionIdLenMinus1s         []byte
	NuhLayerIdPresentFlag         bool
	LayerIdInNuhs                 []byte
	ViewIdLen                     byte
	SubLayersMaxMinus1PresentFlag bool
	MaxTidRefPresentFlag          bool
	DefaultRefLayersActiveFlag    bool
	NumProfileTierLevelMinus1     uint32
	NumRepFormatsMinus1           uint32
	MaxOneActiveRefLayerFlag      bool
	PocLsbAlignedFlag             bool
	DirectDepTypeLenMinus2        uint32
	DirectDependencyAllLayersFlag bool
	NonVuiExtensionLength         uint32
	VUIPresentFlag                bool
}

// parseVPSExtension follows F.7.3.2.1.1 (page 565) and F.7.4.3.1.1 (page 795)
func (vps *VPS) parseVPSExtension(r *bits.AccErrEBSPReader) (*VPSExtension, error) {
	ve := VPSExtension{}
	if vps.MaxLayersMinus1 > 0 && vps.BaseLayerInternalFlag {
		ve.ProfileTierLevel = parseProfileTierLevel(r, false, vps.MaxSubLayersMinus1)
	}
	splittingFlag := r.ReadFlag()
	numScalabilityTypes := 0
	for i := 0; i < 16; i++ {
		ve.ScalabilityMaskFlags[i] = r.ReadFlag()
		if ve.ScalabilityMaskFlags[i] {
			numScalabilityTypes++
		}
	}
	nst := numScalabilityTypes
	if splittingFlag {
		nst--
	}
	ve.DimensionIdLenMinus1s = make([]byte, nst)
	for j := 0; j < nst; j++ {
		ve.DimensionIdLenMinus1s[j] = byte(r.Read(3))
	}
	ve.NuhLayerIdPresentFlag = r.ReadFlag()
	if vps.MaxLayersMinus1 > 0 {
		// Cannot parse any longer
		return &ve, nil
	}
	// Note! The following is only for single layer
	ve.ViewIdLen = byte(r.Read(4))
	if ve.ViewIdLen != 0 {
		return nil, fmt.Errorf("view_id_len_minus1 is %d, not 0", ve.ViewIdLen)
	}
	ve.SubLayersMaxMinus1PresentFlag = r.ReadFlag()
	if ve.SubLayersMaxMinus1PresentFlag {
		return nil, fmt.Errorf("sub_layers_max_minus1_present_flag is true")
	}
	ve.MaxTidRefPresentFlag = r.ReadFlag()
	ve.DefaultRefLayersActiveFlag = r.ReadFlag()
	ve.NumProfileTierLevelMinus1 = uint32(r.ReadExpGolomb())
	start := uint32(1)
	if vps.BaseLayerInternalFlag {
		start = 2
	}
	for i := start; i <= ve.NumProfileTierLevelMinus1; i++ {
		profilePresentFlag := r.ReadFlag() // profile_present_flag[i]
		if profilePresentFlag {
			_ = parseProfileTierLevel(r, profilePresentFlag, vps.MaxSubLayersMinus1)
		}
		_ = r.ReadFlag() // level_present_flag[i]
	}
	ve.NumRepFormatsMinus1 = uint32(r.ReadExpGolomb())
	if ve.NumRepFormatsMinus1 > 0 {
		return nil, fmt.Errorf("num_rep_formats_minus1 is %d, not 0", ve.NumRepFormatsMinus1)
	}
	ve.MaxOneActiveRefLayerFlag = r.ReadFlag()
	ve.PocLsbAlignedFlag = r.ReadFlag()
	// dpb_size()
	ve.DirectDepTypeLenMinus2 = uint32(r.ReadExpGolomb())
	ve.DirectDependencyAllLayersFlag = r.ReadFlag()
	// A bit unclear if we need to do anything when we only have one layer
	ve.NonVuiExtensionLength = uint32(r.ReadExpGolomb())
	for i := uint32(0); i < ve.NonVuiExtensionLength; i++ {
		_ = r.Read(8) // non_vui_extension_byte[i]
	}
	ve.VUIPresentFlag = r.ReadFlag()
	if ve.VUIPresentFlag {
		for {
			b := r.NrBitsReadInCurrentByte()
			if b != 0 && b != 8 {
				continue
			}
			break
		}
		// TODO
		//ve.VUI = parseVUIParameters(r)
	}
	return &ve, r.AccError()
}
