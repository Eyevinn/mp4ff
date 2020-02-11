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

// CreateMP4Init - Create a full one-track MP4 init segment
func CreateMP4Init(timeScale uint32) *InitSegment {
	/* Build tree like
	   moov
	   - mvhd  (Nothing interesting)
	   - trak
	     + tkhd (trakID, flags, width, height)
	     + mdia
	       - mdhd (track Timescale, language (3letters))
	       - hdlr (hdlr string)
	       - minf
	         + vmhd (video media header box etc)
	         + dinf (can drop)
	         + stbl
	           - stsd
	             + avc1
	               - avcC
	           - stts
	           - stsc
	           - stsz
	           - stco
	   - mvex
	     + trex
	*/
	initSeg := NewMP4Init()
	initSeg.AddChild(CreateFtyp())
	moov := NewMoovBox()
	initSeg.AddChild(moov)
	mvhd := &MvhdBox{}
	mvhd.Timescale = 90000
	moov.AddChild(mvhd)
	trak := &TrakBox{}
	moov.AddChild(trak)
	tkhd := &TkhdBox{}
	trak.AddChild(tkhd)
	mdia := &MdiaBox{}
	trak.AddChild(mdia)
	mdhd := &MdhdBox{}
	mdia.AddChild(mdhd)
	hdlr := &HdlrBox{}
	mdia.AddChild(hdlr)
	minf := NewMinfBox()
	mdia.AddChild(minf)
	vmhd := &VmhdBox{}
	minf.AddChild(vmhd)
	stbl := NewStblBox()
	minf.AddChild(stbl)
	stsd := NewStsdBox()
	stbl.AddChild(stsd)
	// TODO. Add avc1 etc sample description
	stbl.AddChild(&SttsBox{})
	stbl.AddChild(&StscBox{})
	stbl.AddChild(&StszBox{})
	stbl.AddChild(&StcoBox{})
	mvex := NewMvexBox()
	moov.AddChild(mvex)
	trex := &TrexBox{}
	mvex.AddChild(trex)

	return initSeg
}
