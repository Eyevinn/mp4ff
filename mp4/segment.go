package mp4

import "io"

// InitSegment - MP4/CMAF init segment
type InitSegment struct {
	Ftyp  *FtypBox
	Moov  *MoovBox
	boxes []Box
}

// NewMP4Init - Create MP4Init
func NewMP4Init() *InitSegment {
	return &InitSegment{
		boxes: []Box{},
	}
}

// AddChild - Add a child box to InitSegment
func (s *InitSegment) AddChild(b Box) {
	switch b.Type() {
	case "ftyp":
		s.Ftyp = b.(*FtypBox)
	case "moov":
		s.Moov = b.(*MoovBox)
	}
	s.boxes = append(s.boxes)
}

// Encode - write InitSegment via writer
func (s *InitSegment) Encode(w io.Writer) error {
	for _, b := range s.boxes {
		err := b.Encode(w)
		if err != nil {
			return err
		}
	}
	return nil
}

// MediaSegment - MP4 Media Segment
type MediaSegment struct {
	Styp      *StypBox
	Fragments []*Fragment
}

// NewMediaSegment - Create MP4Segment
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
