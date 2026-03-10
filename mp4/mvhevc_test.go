package mp4_test

import (
	"testing"

	"github.com/Eyevinn/mp4ff/mp4"
)

func TestOinfSgpd(t *testing.T) {
	oinf := &mp4.OinfSampleGroupEntry{
		ScalabilityMask: 0x0002,
		ProfileTierLevels: []mp4.OinfPTL{
			{GeneralProfileSpace: 0, GeneralTierFlag: false, GeneralProfileIDC: 1, GeneralLevelIDC: 120,
				GeneralProfileCompatibilityFlags: 0x60000000, GeneralConstraintIndicatorFlags: 0x900000000000},
			{GeneralProfileSpace: 0, GeneralTierFlag: false, GeneralProfileIDC: 0, GeneralLevelIDC: 120},
			{GeneralProfileSpace: 0, GeneralTierFlag: false, GeneralProfileIDC: 6, GeneralLevelIDC: 120},
		},
		OperatingPoints: []mp4.OinfOperatingPoint{
			{
				OutputLayerSetIdx: 0, MaxTemporalID: 0,
				Layers: []mp4.OinfOPLayer{
					{PtlIdx: 1, LayerID: 0, IsOutputLayer: true},
				},
				MinPicWidth: 1920, MinPicHeight: 1080, MaxPicWidth: 1920, MaxPicHeight: 1080,
				MaxChromaFormat: 1, MaxBitDepthMinus8: 0,
			},
			{
				OutputLayerSetIdx: 1, MaxTemporalID: 0,
				Layers: []mp4.OinfOPLayer{
					{PtlIdx: 1, LayerID: 0, IsOutputLayer: false},
					{PtlIdx: 2, LayerID: 1, IsOutputLayer: false},
				},
				MinPicWidth: 1920, MinPicHeight: 1080, MaxPicWidth: 1920, MaxPicHeight: 1080,
				MaxChromaFormat: 1, MaxBitDepthMinus8: 0,
			},
		},
		DependencyLayers: []mp4.OinfDependencyLayer{
			{LayerID: 0, DependsOnLayers: nil, DimensionIds: []byte{0}},
			{LayerID: 1, DependsOnLayers: []byte{0}, DimensionIds: []byte{1}},
		},
	}

	sgpd := &mp4.SgpdBox{
		Version:            2,
		GroupingType:       "oinf",
		DefaultLength:      uint32(oinf.Size()),
		SampleGroupEntries: []mp4.SampleGroupEntry{oinf},
	}
	boxDiffAfterEncodeAndDecode(t, sgpd)
}

func TestLinfSgpd(t *testing.T) {
	linf := &mp4.LinfSampleGroupEntry{
		Layers: []mp4.LinfLayerEntry{
			{LayerID: 0, MinTemporalID: 0, MaxTemporalID: 0, SubLayerPresenceFlags: 0x7f},
			{LayerID: 1, MinTemporalID: 0, MaxTemporalID: 0, SubLayerPresenceFlags: 0x7f},
		},
	}

	sgpd := &mp4.SgpdBox{
		Version:            2,
		GroupingType:       "linf",
		DefaultLength:      uint32(linf.Size()),
		SampleGroupEntries: []mp4.SampleGroupEntry{linf},
	}
	boxDiffAfterEncodeAndDecode(t, sgpd)
}
