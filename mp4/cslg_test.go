package mp4

import "testing"

func TestCslgEncodeDecode(t *testing.T) {
	cslg := CslgBox{
		Version:                      0,
		CompositionToDTSShift:        -100,
		LeastDecodeToDisplayDelta:    200,
		GreatestDecodeToDisplayDelta: -30,
		CompositionStartTime:         1600,
		CompositionEndTime:           1000,
	}

	boxDiffAfterEncodeAndDecode(t, &cslg)

	cslg = CslgBox{
		Version:                      1,
		CompositionToDTSShift:        -100,
		LeastDecodeToDisplayDelta:    200,
		GreatestDecodeToDisplayDelta: -30,
		CompositionStartTime:         1600,
		CompositionEndTime:           1000,
	}

	boxDiffAfterEncodeAndDecode(t, &cslg)
}
