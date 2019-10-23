package mp4

import (
	"fmt"
	"io"
	"os"
)

// MP4 - an MPEG-4 file asset
//
// A MPEG-4 media contains three main boxes if progressive :
//
//   ftyp : the file type box
//   moov : the movie box (meta-data)
//   mdat : the media data (chunks and samples)
//
// If segmented, it instead contain a list of segments
// Other boxes can also be present (pdin, moof, mfra, free, ...), but are not decoded.
type MP4 struct {
	Ftyp         *FtypBox
	Moov         *MoovBox
	Mdat         *MdatBox
	boxes        []Box
	isFragmented bool
	Init         *InitSegment
	Segments     []*MediaSegment
}

// NewMP4 - create MP4
func NewMP4() *MP4 {
	return &MP4{
		boxes:    []Box{},
		Segments: []*MediaSegment{},
	}
}

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

// MediaSegment - MP4 Media Segment
type MediaSegment struct {
	Styp      *UnknownBox
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

// Decode - decode a media from a Reader
func Decode(r io.ReadSeeker) (*MP4, error) {

	var currentSegment *MediaSegment
	var currentFragment *Fragment
	stypPresent := false
	m := NewMP4()
	var boxStartPos uint64 = 0
LoopBoxes:
	for {
		//f := r.(*os.File)
		//p, err := f.Seek(0, os.SEEK_CUR)
		//log.Printf("Byte position is %v", p)
		h, err := DecodeHeader(r)
		if err == io.EOF || h.Size == 0 {
			break LoopBoxes
		}
		if err != nil {
			return nil, err
		}
		box, err := DecodeBox(h, r)
		if err != nil {
			return nil, err
		}
		m.boxes = append(m.boxes, box)
		switch h.Type {
		case "ftyp":
			m.Ftyp = box.(*FtypBox)
		case "moov":
			m.Moov = box.(*MoovBox)
			if len(m.Moov.Trak[0].Mdia.Minf.Stbl.Stts.SampleCount) == 0 {
				m.isFragmented = true
				m.Init = NewMP4Init()
				m.Init.Ftyp = m.Ftyp
				m.Init.Moov = m.Moov
			}
		case "styp":
			stypPresent = true
			currentSegment = NewMediaSegment()
			currentSegment.Styp = box.(*UnknownBox)
			m.Segments = append(m.Segments, currentSegment)
		case "moof":
			moof := box.(*MoofBox)
			moof.StartPos = boxStartPos

			if !stypPresent {
				currentSegment = NewMediaSegment()
				m.Segments = append(m.Segments, currentSegment)
			}
			currentFragment = NewFragment()
			currentSegment.Fragments = append(currentSegment.Fragments, currentFragment)
			currentFragment.Moof = moof
		case "mdat":
			mdat := box.(*MdatBox)
			if !m.isFragmented {
				m.Mdat = mdat
			} else {
				currentFragment.Mdat = mdat
			}
		}
		boxStartPos += uint64(box.Size())
	}
	return m, nil
}

// Dump displays some information about a media
func (m *MP4) Dump(r io.ReadSeeker) {
	if m.isFragmented {
		fmt.Printf("Init segment\n")
		m.Init.Moov.Dump()
		for i, seg := range m.Segments {
			fmt.Printf("  mediaSegment %d\n", i)
			for j, f := range seg.Fragments {
				fmt.Printf("  fragment %d\n ", j)
				w, _ := os.Create("tmp.264")
				defer w.Close()
				f.DumpSampleData(r, w)
			}
		}

	} else {
		m.Ftyp.Dump()
		m.Moov.Dump()
	}
}

// Boxes lists the top-level boxes from a media
func (m *MP4) Boxes() []Box {
	return m.boxes
}

// Encode encodes a media to a Writer
func (m *MP4) Encode(w io.Writer) error {
	for _, b := range m.Boxes() {
		err := b.Encode(w)
		if err != nil {
			return err
		}
	}
	return nil
}

// DumpSampleData - Get Sample data and print out
func (f *Fragment) DumpSampleData(r io.ReadSeeker, w io.Writer) {

	seqNr := f.Moof.Mfhd.SequenceNumber
	fmt.Printf("Samples for Segment %d\n", seqNr)
	tfhd := f.Moof.Traf.Tfhd
	baseTime := f.Moof.Traf.Tfdt.BaseMediaDecodeTime
	trun := f.Moof.Traf.Trun
	moofStartPos := f.Moof.StartPos
	trun.AddSampleDefaultValues(tfhd)
	var baseOffset uint64
	if tfhd.HasBaseDataOffset() {
		baseOffset = tfhd.BaseDataOffset
	} else if tfhd.DefaultBaseIfMoof() {
		baseOffset = moofStartPos
	}
	if trun.HasDataOffset() {
		baseOffset = uint64(int64(trun.DataOffset) + int64(baseOffset))
	}
	samples := trun.GetSampleData(r, baseOffset, baseTime)
	for i, s := range samples {
		if i < 9 {
			fmt.Printf("%4d %8d %8d %6x %d %d\n", i, s.DecodeTime, s.PresentationTime, s.Flags, s.Size, len(s.Data))
		}
		if w != nil {
			w.Write(s.Data)
		}
	}
}
