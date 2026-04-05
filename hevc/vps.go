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
	MaxDecPicBufferingMinus1        []uint
	MaxNumReorderPics               []uint
	MaxLatencyIncreasePlus1         []uint
	MaxLayerID                      byte
	NumLayerSetsMinus1              uint
	TimingInfoPresentFlag           bool
	TimingInfo                      *VPSTimingInfo
	ExtensionFlag                   bool
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
	for i := uint(1); i <= vps.NumLayerSetsMinus1; i++ {
		for j := byte(0); j <= vps.MaxLayerID; j++ {
			_ = r.ReadFlag() // layer_id_included_flag[i][j]
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
	if r.AccError() != nil {
		return vps, nil
	}

	return vps, nil
}
