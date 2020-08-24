package mp4

import (
	"fmt"
	"io"
)

// MediaSegment - MP4 Media Segment
type MediaSegment struct {
	Styp      *StypBox
	Fragments []*Fragment
}

// NewMediaSegment - New empty MediaSegment
func NewMediaSegment() *MediaSegment {
	return &MediaSegment{
		Fragments: []*Fragment{},
	}
}

// AddFragment - Add a fragment to a MediaSegment
func (s *MediaSegment) AddFragment(f *Fragment) {
	s.Fragments = append(s.Fragments, f)
}

// LastFragment - Currently last fragment
func (s *MediaSegment) LastFragment() *Fragment {
	return s.Fragments[len(s.Fragments)-1]
}

// Encode - Write MediaSegment via writer
func (s *MediaSegment) Encode(w io.Writer) error {
	if s.Styp != nil {
		err := s.Encode(w)
		if err != nil {
			return err
		}
	}
	for _, f := range s.Fragments {
		err := f.Encode(w)
		if err != nil {
			return err
		}
	}
	return nil
}

// Fragmentify - Split into multiple fragments. Assume single mdat and trun for now
func (s *MediaSegment) Fragmentify(timescale uint64, trex *TrexBox, duration uint32) ([]*Fragment, error) {
	inFragments := s.Fragments
	outFragments := make([]*Fragment, 0)
	var of *Fragment

	var cumDur uint32 = 0

	for _, inFrag := range inFragments {

		samples := inFrag.GetCompleteSamples(trex)
		for _, s := range samples {
			if cumDur == 0 {
				var err error
				of, err = CreateFragment(inFrag.Moof.Mfhd.SequenceNumber, inFrag.Moof.Traf.Tfhd.TrackID)
				if err != nil {
					return nil, err
				}
				outFragments = append(outFragments, of)
			}
			of.AddSample(s)
			cumDur += s.Dur
			if cumDur >= duration {
				fmt.Printf("Wrote fragment with duration %d\n", cumDur)
				cumDur = 0
			}
		}
	}
	return outFragments, nil
}
