package mp4

import (
	"bytes"
	"fmt"
	"io"
)

// InitSegment - MP4/CMAF init segment
type InitSegment struct {
	MediaType string
	Ftyp      *FtypBox
	Moov      *MoovBox
	Children  []Box // All top-level boxes in order
}

// NewMP4Init - Create MP4Init
func NewMP4Init() *InitSegment {
	return &InitSegment{
		Children: []Box{},
	}
}

// AddChild - Add a top-level box to InitSegment
func (s *InitSegment) AddChild(b Box) {
	switch b.Type() {
	case "ftyp":
		s.Ftyp = b.(*FtypBox)
	case "moov":
		s.Moov = b.(*MoovBox)
	}
	s.Children = append(s.Children, b)
}

// Encode - encode an initsegment to a Writer
func (s *InitSegment) Encode(w io.Writer) error {
	for _, b := range s.Children {
		err := b.Encode(w)
		if err != nil {
			return err
		}
	}
	return nil
}

// CreateEmptyMP4Init - Create a one-track MP4 init segment with empty stsd box
// The trak has trackID = 1. The irrelevant mdhd timescale is set to 90000 and duration = 0
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
	if mediaType == "audio" {
		tkhd.Volume = 0x0100 // Fixed 16 value 1.0
	}
	trak.AddChild(tkhd)

	mdia := &MdiaBox{}
	trak.AddChild(mdia)
	mdhd := &MdhdBox{}
	mdhd.Timescale = timeScale
	mdia.AddChild(mdhd)
	hdlr, err := CreateHdlr(mediaType)
	if err != nil {
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
	trex := CreateTrex()
	mvex.AddChild(trex)

	return initSeg
}

// SetAVCDescriptor - Modify a TrakBox by adding AVC SampleDescriptor from one SPS and multiple PPS
// Get width and height from SPS and fill into tkhd box.
func (t *TrakBox) SetAVCDescriptor(sampleDescriptorType string, spsNALU []byte, ppsNALUs [][]byte) {
	avcSPS, err := ParseSPSNALUnit(spsNALU)
	if err != nil {
		panic("Cannot handle SPS parsing errors")
	}
	t.Tkhd.Width = Fixed32(avcSPS.Width << 16)   // This is display width
	t.Tkhd.Height = Fixed32(avcSPS.Height << 16) // This is display height
	stsd := t.Mdia.Minf.Stbl.Stsd
	if sampleDescriptorType != "avc1" && sampleDescriptorType != "avc3" {
		panic(fmt.Sprintf("sampleDescriptorType %s not allowed", sampleDescriptorType))
	}
	avcC := CreateAvcC(spsNALU, ppsNALUs)
	width, height := uint16(avcSPS.Width), uint16(avcSPS.Height)
	avcx := CreateVisualSampleEntryBox(sampleDescriptorType, width, height, avcC)
	stsd.AddChild(avcx)
}

// GetMediaType - should return video or audio (at present)
func (s *InitSegment) GetMediaType() string {
	switch s.Moov.Trak[0].Mdia.Hdlr.HandlerType {
	case "soun":
		return "audio"
	case "vide":
		return "video"
	default:
		return "unknown"
	}
}

// SetAACDescriptor - Modify a TrakBox by adding AAC SampleDescriptor
// objType is one of AAClc, HEAACv1, HEAACv2
// For HEAAC, the samplingFrequency is the base frequency (normally 24000)
func (t *TrakBox) SetAACDescriptor(objType byte, samplingFrequency int) error {
	stsd := t.Mdia.Minf.Stbl.Stsd
	asc := &AudioSpecificConfig{
		ObjectType:           objType,
		ChannelConfiguration: 2,
		SamplingFrequency:    samplingFrequency,
		ExtensionFrequency:   0,
		SBRPresentFlag:       false,
		PSPresentFlag:        false,
	}
	switch objType {
	case HEAACv1:
		asc.ExtensionFrequency = 2 * samplingFrequency
		asc.SBRPresentFlag = true
	case HEAACv2:
		asc.ExtensionFrequency = 2 * samplingFrequency
		asc.SBRPresentFlag = true
		asc.ChannelConfiguration = 1
		asc.PSPresentFlag = true
	}

	buf := &bytes.Buffer{}
	err := asc.Encode(buf)
	if err != nil {
		return err
	}
	ascBytes := buf.Bytes()
	esds := CreateEsdsBox(ascBytes)
	mp4a := CreateAudioSampleEntryBox("mp4a",
		uint16(asc.ChannelConfiguration),
		16, uint16(samplingFrequency), esds)
	stsd.AddChild(mp4a)
	return nil
}
