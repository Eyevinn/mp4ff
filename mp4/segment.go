package mp4

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

// Fragment - MP4 Fragment (moof + mdat)
type Fragment struct {
	Moof  *MoofBox
	Mdat  *MdatBox
	boxes []Box
}

// NewFragment - Create MP4 Fragment
func NewFragment() *Fragment {
	return &Fragment{}
}
