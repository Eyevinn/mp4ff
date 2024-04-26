package mp4

import "testing"

func TestSsix(t *testing.T) {
	ssix := SsixBox{}
	ss := SubSegment{
		Ranges: []SubSegmentRange{
			NewSubSegmentRange(1, 2),
			NewSubSegmentRange(3, 4),
		}}
	ssix.SubSegments = append(ssix.SubSegments, ss)
	ss = SubSegment{
		Ranges: []SubSegmentRange{
			NewSubSegmentRange(2, 2),
			NewSubSegmentRange(5, 4),
		}}
	ssix.SubSegments = append(ssix.SubSegments, ss)
	boxDiffAfterEncodeAndDecode(t, &ssix)
}
