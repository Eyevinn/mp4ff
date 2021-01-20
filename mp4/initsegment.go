package mp4

import (
	"bytes"
	"fmt"
	"io"

	"github.com/edgeware/mp4ff/aac"
	"github.com/edgeware/mp4ff/avc"
	"github.com/edgeware/mp4ff/hevc"
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

// Info - write box tree with indent for each level
func (i *InitSegment) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	for _, box := range i.Children {
		err := box.Info(w, specificBoxLevels, indent, indentStep)
		if err != nil {
			return err
		}
	}
	return nil
}

// CreateEmptyInit - create an init segment for fragmented files
func CreateEmptyInit() *InitSegment {
	initSeg := NewMP4Init()
	initSeg.AddChild(CreateFtyp())
	moov := NewMoovBox()
	initSeg.AddChild(moov)
	mvhd := CreateMvhd()
	moov.AddChild(mvhd)
	mvex := NewMvexBox()
	moov.AddChild(mvex)
	return initSeg
}

// AddEmptyTrack - add trak + trex box with appropriate trackID value
func (i *InitSegment) AddEmptyTrack(timeScale uint32, mediaType, language string) {
	moov := i.Moov
	trackID := uint32(len(moov.Traks) + 1)
	moov.Mvhd.NextTrackID = trackID + 1
	newTrak := CreateEmptyTrak(trackID, timeScale, mediaType, language)
	moov.AddChild(newTrak)
	moov.Mvex.AddChild(CreateTrex(trackID))
}

// Create a full Trak tree for an empty (fragmented) track with no samples or stsd content
func CreateEmptyTrak(trackID, timeScale uint32, mediaType, language string) *TrakBox {
	/*  Built tree like
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
	*/
	trak := &TrakBox{}
	tkhd := CreateTkhd()
	tkhd.TrackID = trackID
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
	case "subtitle":
		minf.AddChild(&SthdBox{})
	default:
		minf.AddChild(&NmhdBox{})
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
	return trak
}

// SetAVCDescriptor - Set AVC SampleDescriptor based on SPS and PPS
func (t *TrakBox) SetAVCDescriptor(sampleDescriptorType string, spsNALUs, ppsNALUs [][]byte) error {
	if sampleDescriptorType != "avc1" && sampleDescriptorType != "avc3" {
		return fmt.Errorf("sampleDescriptorType %s not allowed", sampleDescriptorType)
	}
	avcSPS, err := avc.ParseSPSNALUnit(spsNALUs[0], false)
	if err != nil {
		return fmt.Errorf("Could not parse SPS NALU: %w", err)
	}
	t.Tkhd.Width = Fixed32(avcSPS.Width << 16)   // This is display width
	t.Tkhd.Height = Fixed32(avcSPS.Height << 16) // This is display height
	stsd := t.Mdia.Minf.Stbl.Stsd

	avcC, err := CreateAvcC(spsNALUs, ppsNALUs)
	if err != nil {
		return err
	}
	width, height := uint16(avcSPS.Width), uint16(avcSPS.Height)
	avcx := CreateVisualSampleEntryBox(sampleDescriptorType, width, height, avcC)
	stsd.AddChild(avcx)
	return nil
}

// SetHEVCDescriptor - Set HEVC SampleDescriptor based on VPS, SPS, and PPS
func (t *TrakBox) SetHEVCDescriptor(sampleDescriptorType string, vpsNALUs, spsNALUs, ppsNALUs [][]byte) error {
	if sampleDescriptorType != "hvc1" && sampleDescriptorType != "hev1" {
		return fmt.Errorf("sampleDescriptorType %s not allowed", sampleDescriptorType)
	}
	hevcSPS, err := hevc.ParseSPSNALUnit(spsNALUs[0])
	if err != nil {
		return fmt.Errorf("Could not parse SPS NALU: %w", err)
	}
	width, height := hevcSPS.ImageSize()
	t.Tkhd.Width = Fixed32(width << 16)   // This is display width
	t.Tkhd.Height = Fixed32(height << 16) // This is display height
	stsd := t.Mdia.Minf.Stbl.Stsd

	hvcC, err := CreateHvcC(vpsNALUs, spsNALUs, ppsNALUs, true, true, true)
	if err != nil {
		return err
	}
	avcx := CreateVisualSampleEntryBox(sampleDescriptorType, uint16(width), uint16(height), hvcC)
	stsd.AddChild(avcx)
	return nil
}

// GetMediaType - should return video or audio (at present)
func (s *InitSegment) GetMediaType() string {
	switch s.Moov.Trak.Mdia.Hdlr.HandlerType {
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
	asc := &aac.AudioSpecificConfig{
		ObjectType:           objType,
		ChannelConfiguration: 2,
		SamplingFrequency:    samplingFrequency,
		ExtensionFrequency:   0,
		SBRPresentFlag:       false,
		PSPresentFlag:        false,
	}
	switch objType {
	case aac.HEAACv1:
		asc.ExtensionFrequency = 2 * samplingFrequency
		asc.SBRPresentFlag = true
	case aac.HEAACv2:
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
