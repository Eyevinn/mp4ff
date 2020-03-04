package mp4

import (
	"fmt"
	"io"
)

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
	s.boxes = append(s.boxes, b)
}

// Encode - encode an initsegment to a Writer
func (s *InitSegment) Encode(w io.Writer) error {
	for _, b := range s.boxes {
		err := b.Encode(w)
		if err != nil {
			return err
		}
	}
	return nil
}

// CreateEmptyMP4Init - Create a one-track MP4 init segment with empty stsd box
func CreateEmptyMP4Init(timeScale uint32, mediaType, language string) *InitSegment {
	/* Build tree like
	   moov
	   - mvhd  (Nothing interesting)
	   - trak
	     + tkhd (trakID, flags, width, height)
	     + mdia
	       - mdhd (track Timescale, language (3letters))
		   - hdlr (hdlr showing mediaType)
		   - elng (only if language is not 3 letters)
	       - minf
	         + vmhd/smhd etc (media header box)
			 + dinf
			   - dref
			     + url
	         + stbl
	           - stsd
	             + empty on purpose
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
	mvhd := CreateMvhd()
	moov.AddChild(mvhd)
	trak := &TrakBox{}
	moov.AddChild(trak)
	tkhd := CreateTkhd()
	trak.AddChild(tkhd)
	mdia := &MdiaBox{}
	trak.AddChild(mdia)
	mdhd := &MdhdBox{}
	mdhd.Timescale = timeScale
	mdia.AddChild(mdhd)
	hdlr := &HdlrBox{}
	switch mediaType {
	case "video":
		hdlr.HandlerType = "vide"
		hdlr.Name = "Edgeware Video Handler"
	case "audio":
		hdlr.HandlerType = "soun"
		hdlr.Name = "Edgeware Audio Handler"
	default:
		panic(fmt.Sprintf("mediaType %s not supported", mediaType))
	}
	mdia.AddChild(hdlr)
	if len(language) == 3 {
		mdhd.SetLanguage(language)
	} else {
		mdhd.SetLanguage("und")
		elng := CreateElng(language)
		mdia.AddChild(elng)
	}
	minf := NewMinfBox()
	mdia.AddChild(minf)
	switch mediaType {
	case "video":
		minf.AddChild(CreateVmhd())
	case "audio":
		minf.AddChild(CreateSmhd())
	default:
		panic(fmt.Sprintf("mediaType %s not supported", mediaType))
	}
	dinf := &DinfBox{}
	dinf.AddChild(CreateDref())
	minf.AddChild(dinf)
	stbl := NewStblBox()
	minf.AddChild(stbl)
	stsd := NewStsdBox()
	stbl.AddChild(stsd)
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

// SetAVCDescriptor - Modify a TrakBox by adding AVC SampleDescriptor from one SPS and multiple PPS
func (t *TrakBox) SetAVCDescriptor(sampledDescriptorType string, spsNALU []byte, ppsNALUs [][]byte) {
	avcSPS, err := ParseSPSNALUnit(spsNALU)
	if err != nil {
		panic("Cannot handle SPS parsing errors")
	}
	t.Tkhd.Width = Fixed32(avcSPS.Width << 16)   // This is display width
	t.Tkhd.Height = Fixed32(avcSPS.Height << 16) // This is display height
	stsd := t.Mdia.Minf.Stbl.Stsd
	if sampledDescriptorType != "avc1" && sampledDescriptorType != "avc3" {
		panic(fmt.Sprintf("sampleDescriptorType %s not allowed", sampledDescriptorType))
	}
	avcC := CreateAvcC(spsNALU, ppsNALUs)
	width, height := uint16(avcSPS.Width), uint16(avcSPS.Height)
	avcx := CreateVisualSampleEntryBox(sampledDescriptorType, width, height, avcC)
	stsd.AddChild(avcx)
}
