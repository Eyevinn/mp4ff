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

// AddFragment - Add a fragment to a MediaSegment
func (s *MediaSegment) AddFragment(f *Fragment) {
	s.Fragments = append(s.Fragments, f)
}

func (s *MediaSegment) lastFragment() *Fragment {
	return s.Fragments[len(s.Fragments)-1]
}

// Fragment - MP4 Fragment (moof + mdat)
type Fragment struct {
	Prft  *PrftBox
	Moof  *MoofBox
	Mdat  *MdatBox
	boxes []Box
}

// NewFragment - New emtpy MP4 Fragment
func NewFragment() *Fragment {
	return &Fragment{}
}

// CreateFragment - create emtpy fragment for output
func CreateFragment(seqNumber uint32, trackID uint32) *Fragment {
	f := NewFragment()
	moof := &MoofBox{}
	f.AddChild(moof)
	mfhd := CreateMfhd(seqNumber)
	moof.AddChild(mfhd)
	traf := &TrafBox{}
	moof.AddChild(traf)
	tfhd := CreateTfhd(trackID)
	traf.AddChild(tfhd)
	tfdt := &TfdtBox{} // We will get time with samples
	traf.AddChild(tfdt)
	trun := CreateTrun()
	traf.AddChild(trun)
	mdat := &MdatBox{}
	f.AddChild(mdat)

	return f
}

// AddChild - Add a child box to Fragment
func (s *Fragment) AddChild(b Box) {
	switch b.Type() {
	case "prft":
		s.Prft = b.(*PrftBox)
	case "moof":
		s.Moof = b.(*MoofBox)
	case "mdat":
		s.Mdat = b.(*MdatBox)
	}
	s.boxes = append(s.boxes, b)
}
