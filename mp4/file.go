package mp4

import (
	"fmt"
	"io"
	"log"
	"os"
)

// File - an MPEG-4 file asset
//
// A MPEG-4 media contains three main boxes if progressive :
//
//   ftyp : the file type box
//   moov : the movie box (meta-data)
//   mdat : the media data (chunks and samples)
//
// If segmented, it instead contain a list of segments
// Other boxes can also be present (pdin, moof, mfra, free, ...), but are not decoded.
type File struct {
	Ftyp         *FtypBox
	Moov         *MoovBox
	Mdat         *MdatBox // Only used for non-fragmented boxes
	boxes        []Box    // All boxes in order
	isFragmented bool
	Init         *InitSegment
	Segments     []*MediaSegment
}

// NewFile - create MP4 file
func NewFile() *File {
	return &File{
		boxes:    []Box{},
		Segments: []*MediaSegment{},
	}
}

// DecodeFile - top-level of a file from a Reader
func DecodeFile(r io.Reader) (*File, error) {

	var currentSegment *MediaSegment
	var currentFragment *Fragment
	stypPresent := false
	m := NewFile()
	var boxStartPos uint64 = 0
	lastBoxType := ""
LoopBoxes:
	for {
		//f := r.(*os.File)
		//p, err := f.Seek(0, os.SEEK_CUR)
		//log.Printf("Byte position is %v", p)
		box, err := DecodeBox(boxStartPos, r)
		if err == io.EOF {
			break LoopBoxes
		}
		if err != nil {
			return nil, err
		}
		bType, bSize := box.Type(), box.Size()
		log.Printf("Box %v, size %v at pos %v", bType, bSize, boxStartPos)
		if err != nil {
			return nil, err
		}
		m.boxes = append(m.boxes, box)
		switch bType {
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
			currentSegment.Styp = box.(*StypBox)
			m.Segments = append(m.Segments, currentSegment)
		case "moof":
			moof := box.(*MoofBox)
			moof.StartPos = boxStartPos

			if !stypPresent {
				currentSegment = NewMediaSegment()
				m.Segments = append(m.Segments, currentSegment)
			}
			currentFragment = NewFragment()
			currentSegment.AddFragment(currentFragment)
			currentFragment.AddChild(moof)
		case "mdat":
			mdat := box.(*MdatBox)
			if !m.isFragmented {
				if lastBoxType != "moov" {
					log.Fatalf("Does not support %v between moov and mdat", lastBoxType)
				}
				m.Mdat = mdat
			} else {
				if lastBoxType != "moof" {
					log.Fatalf("Does not support %v between moof and mdat", lastBoxType)
				}
				currentFragment.AddChild(mdat)
			}
		}
		lastBoxType = bType
		boxStartPos += bSize
	}
	return m, nil
}

// Dump displays some information about a media
func (m *File) Dump(r io.Reader) {
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
func (m *File) Boxes() []Box {
	return m.boxes
}

// Encode encodes a media to a Writer
func (m *File) Encode(w io.Writer) error {
	for _, b := range m.Boxes() {
		err := b.Encode(w)
		if err != nil {
			return err
		}
	}
	return nil
}

// DumpSampleData - Get Sample data and print out
func (f *Fragment) DumpSampleData(r io.Reader, w io.Writer) {
	moof := f.Moof
	mdat := f.Mdat
	seqNr := moof.Mfhd.SequenceNumber
	fmt.Printf("Samples for Segment %d\n", seqNr)
	tfhd := moof.Traf.Tfhd
	baseTime := moof.Traf.Tfdt.BaseMediaDecodeTime
	trun := moof.Traf.Trun
	moofStartPos := moof.StartPos
	mdatStartPos := mdat.StartPos
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
	mdatDataLength := uint64(len(mdat.Data)) // Todo. Make len take 64-bit number
	offsetInMdat := baseOffset - mdatStartPos - headerLength(mdatDataLength)
	if offsetInMdat > mdatDataLength {
		log.Fatalf("Offset in mdata beyond size")
	}
	samples := trun.GetSampleData(uint32(offsetInMdat), baseTime, f.Mdat)
	for i, s := range samples {
		if i < 9 {
			fmt.Printf("%4d %8d %8d %6x %d %d\n", i, s.DecodeTime, s.PresentationTime, s.Flags, s.Size, len(s.Data))
		}
		if w != nil {
			w.Write(s.Data)
		}
	}
}
