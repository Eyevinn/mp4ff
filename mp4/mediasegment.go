package mp4

import (
	"io"
)

// MediaSegment - MP4 Media Segment
type MediaSegment struct {
	Styp        *StypBox
	Sidx        *SidxBox // Sidx for a segment
	Fragments   []*Fragment
	EncOptimize EncOptimize
}

// NewMediaSegment - New empty MediaSegment
func NewMediaSegment() *MediaSegment {
	return &MediaSegment{
		Styp:        CreateStyp(),
		Fragments:   []*Fragment{},
		EncOptimize: OptimizeNone,
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
		err := s.Styp.Encode(w)
		if err != nil {
			return err
		}
	}
	if s.Sidx != nil {
		err := s.Sidx.Encode(w)
		if err != nil {
			return err
		}
	}
	for _, f := range s.Fragments {
		f.EncOptimize = s.EncOptimize
		err := f.Encode(w)
		if err != nil {
			return err
		}
	}
	return nil
}

// Info - write box tree with indent for each level
func (m *MediaSegment) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	if m.Styp != nil {
		err := m.Styp.Info(w, specificBoxLevels, indent, indentStep)
		if err != nil {
			return err
		}
	}
	if m.Sidx != nil {
		err := m.Sidx.Info(w, specificBoxLevels, indent, indentStep)
		if err != nil {
			return err
		}
	}
	for _, f := range m.Fragments {
		err := f.Info(w, specificBoxLevels, indent, indentStep)
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
		trackID := inFrag.Moof.Traf.Tfhd.TrackID

		samples, err := inFrag.GetFullSamples(trex)
		if err != nil {
			return nil, err
		}
		for _, s := range samples {
			if cumDur == 0 {
				var err error
				of, err = CreateFragment(inFrag.Moof.Mfhd.SequenceNumber, trackID)
				if err != nil {
					return nil, err
				}
				outFragments = append(outFragments, of)
			}
			//of.AddFullSample(s)
			err = of.AddFullSampleToTrack(s, trackID)
			if err != nil {
				return nil, err
			}
			cumDur += s.Dur
			if cumDur >= duration {
				// fmt.Printf("Wrote fragment with duration %d\n", cumDur)
				cumDur = 0
			}
		}
	}
	return outFragments, nil
}
