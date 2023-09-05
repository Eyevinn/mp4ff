package mp4

import "testing"

func TestTlou(t *testing.T) {
	tlou := &TlouBox{
		loudnessBaseBox: loudnessBaseBox{
			Version: 1,
			Flags:   0,
			LoudnessBases: []*LoudnessBase{
				{
					EQSetID:                0,
					DownmixID:              0,
					DRCSetID:               0,
					BsSamplePeakLevel:      1087,
					BsTruePeakLevel:        1086,
					MeasurementSystemForTP: 2,
					ReliabilityForTP:       3,
					Measurements: []Measurement{
						{
							MethodDefinition:  1,
							MethodValue:       121,
							MeasurementSystem: 2,
							Reliability:       3,
						},
						{
							MethodDefinition:  3,
							MethodValue:       122,
							MeasurementSystem: 1,
							Reliability:       3,
						},
					},
				},
				{
					EQSetID:                0,
					DownmixID:              0,
					DRCSetID:               0,
					BsSamplePeakLevel:      1087,
					BsTruePeakLevel:        1086,
					MeasurementSystemForTP: 2,
					ReliabilityForTP:       3,
					Measurements: []Measurement{
						{
							MethodDefinition:  4,
							MethodValue:       124,
							MeasurementSystem: 1,
							Reliability:       3,
						},
						{
							MethodDefinition:  5,
							MethodValue:       122,
							MeasurementSystem: 1,
							Reliability:       3,
						},
					},
				},
			},
		},
	}
	boxDiffAfterEncodeAndDecode(t, tlou)
}

func TestAlou(t *testing.T) {
	alou := &AlouBox{
		loudnessBaseBox: loudnessBaseBox{
			Version: 1,
			Flags:   0,
			LoudnessBases: []*LoudnessBase{
				{
					EQSetID:                0,
					DownmixID:              0,
					DRCSetID:               0,
					BsSamplePeakLevel:      1087,
					BsTruePeakLevel:        1086,
					MeasurementSystemForTP: 2,
					ReliabilityForTP:       3,
					Measurements: []Measurement{
						{
							MethodDefinition:  1,
							MethodValue:       121,
							MeasurementSystem: 2,
							Reliability:       3,
						},
						{
							MethodDefinition:  3,
							MethodValue:       122,
							MeasurementSystem: 1,
							Reliability:       3,
						},
					},
				},
				{
					EQSetID:                0,
					DownmixID:              0,
					DRCSetID:               0,
					BsSamplePeakLevel:      1087,
					BsTruePeakLevel:        1086,
					MeasurementSystemForTP: 2,
					ReliabilityForTP:       3,
					Measurements: []Measurement{
						{
							MethodDefinition:  4,
							MethodValue:       124,
							MeasurementSystem: 1,
							Reliability:       3,
						},
						{
							MethodDefinition:  5,
							MethodValue:       122,
							MeasurementSystem: 1,
							Reliability:       3,
						},
					},
				},
			},
		},
	}
	boxDiffAfterEncodeAndDecode(t, alou)
}
