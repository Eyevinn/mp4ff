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
	initSeg := NewMP4Init()
	initSeg.AddChild(CreateFtyp())

	// moov
	// - mvhd  (Nothing interesting)
	// - trak
	//   + tkhd (trakID, flags, width, height)
	//   + mdia
	//     - mdhd (track Timescale, language (3letters))
	//     - hdlr (hdlr string)
	//     - minf
	//       + vmhd (video media header box)
	//       + dinf (can drop)
	//       + stbl
	//         - stsd
	//           + avc1
	//             - avcC
	//         - stts
	//         - stsc
	//         - stsz
	//         - stco
	// - mvex
	//   + trex

	return initSeg
}
