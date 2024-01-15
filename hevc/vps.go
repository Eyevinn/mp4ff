package hevc

import (
	"bytes"
	"fmt"

	"github.com/Eyevinn/mp4ff/bits"
)

// VPS is HEVC VPS parameters
// ISO/IEC 23008-2 Sec. 7.3.2.1
type VPS struct {
	VpsID                           byte
	BaseLayerInternalFlag           bool
	BaseLayerAvailableFlag          bool
	MaxLayersMinus1                 byte
	MaxSubLayersMinus1              byte
	TemporalIdNestingFlag           bool
	ProfileTierLevel                ProfileTierLevel
	SubLayerOrderingInfoPresentFlag bool
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
	vps.ProfileTierLevel.GeneralProfileSpace = byte(r.Read(2))
	vps.ProfileTierLevel.GeneralTierFlag = r.ReadFlag()
	vps.ProfileTierLevel.GeneralProfileIDC = byte(r.Read(5))
	vps.ProfileTierLevel.GeneralProfileCompatibilityFlags = uint32(r.Read(32))
	vps.ProfileTierLevel.GeneralConstraintIndicatorFlags = uint64(r.Read(48))
	vps.ProfileTierLevel.GeneralLevelIDC = byte(r.Read(8))
	if vps.MaxSubLayersMinus1 != 0 {
		return vps, nil // Cannot parse any further
	}
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

	return vps, nil
}
