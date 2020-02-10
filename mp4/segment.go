package mp4

import "io"

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

func (s *MediaSegment) lastFragment() *Fragment {
	return s.Fragments[len(s.Fragments)-1]
}

// Encode - write MediaSegment via writer
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
