package hevc

import (
	"encoding/hex"
	"testing"

	"github.com/go-test/deep"
)

func TestVPSParser(t *testing.T) {

	testCases := []struct {
		name      string
		hexData   string
		wantedVPS VPS
	}{
		{
			name:    "VPS 1",
			hexData: "40010c01ffff016000000300900000030000030078959809",
			wantedVPS: VPS{
				VpsID:                  0,
				BaseLayerInternalFlag:  true,
				BaseLayerAvailableFlag: true,
				MaxLayersMinus1:        0,
				MaxSubLayersMinus1:     0,
				TemporalIdNestingFlag:  true,
				ProfileTierLevel: ProfileTierLevel{
					GeneralProfileSpace:              0,
					GeneralTierFlag:                  false,
					GeneralProfileIDC:                1,
					GeneralProfileCompatibilityFlags: 0x60000000,
					GeneralConstraintIndicatorFlags:  0x900000000000,
					GeneralLevelIDC:                  120,
				},
				SubLayerOrderingInfoPresentFlag: true,
			},
		},
	}
	for _, c := range testCases {
		t.Run(c.name, func(t *testing.T) {
			data, err := hex.DecodeString(c.hexData)
			if err != nil {
				t.Errorf("Error decoding hex string: %v", err)
			}
			gotVPS, err := ParseVPSNALUnit(data)
			if err != nil {
				t.Errorf("Error parsing VPS: %v", err)
			}
			if diff := deep.Equal(*gotVPS, c.wantedVPS); diff != nil {
				t.Error(diff)
			}
		})
	}
}
