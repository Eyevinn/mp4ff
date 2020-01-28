package mp4

import (
	"fmt"
	"io"
	"os"

	log "github.com/sirupsen/logrus"
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

// AddMediaSegment - add a mediasegment to file f
func (f *File) AddMediaSegment(m *MediaSegment) {
	f.Segments = append(f.Segments, m)
}

// DecodeFile - top-level of a file from a Reader
func DecodeFile(r io.Reader) (*File, error) {
	f := NewFile()
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
		log.Debugf("Box %v, size %v at pos %v", bType, bSize, boxStartPos)
		if err != nil {
			return nil, err
		}
		if bType == "mdat" {
			if !f.isFragmented {
				if lastBoxType != "moov" {
					log.Fatalf("Does not support %v between moov and mdat", lastBoxType)
				}
			} else {
				if lastBoxType != "moof" {
					log.Fatalf("Does not support %v between moof and mdat", lastBoxType)
				}
			}
		}
		f.AddChildBox(box, boxStartPos)
		lastBoxType = bType
		boxStartPos += bSize
	}
	return f, nil
}

// AddChildBox - add child with start position
func (f *File) AddChildBox(box Box, boxStartPos uint64) {
	bType := box.Type()
	switch bType {
	case "ftyp":
		f.Ftyp = box.(*FtypBox)
	case "moov":
		f.Moov = box.(*MoovBox)
		if len(f.Moov.Trak[0].Mdia.Minf.Stbl.Stts.SampleCount) == 0 {
			f.isFragmented = true
			f.Init = NewMP4Init()
			f.Init.Ftyp = f.Ftyp
			f.Init.Moov = f.Moov
		}
	case "styp":
		f.isFragmented = true
		newSeg := NewMediaSegment()
		newSeg.Styp = box.(*StypBox)
		f.AddMediaSegment(newSeg)
	case "moof":
		f.isFragmented = true
		moof := box.(*MoofBox)
		moof.StartPos = boxStartPos

		var currentSegment *MediaSegment

		if len(f.Segments) == 0 || f.Segments[0].Styp == nil {
			// No styp present, so one fragment per segment
			currentSegment = NewMediaSegment()
			f.AddMediaSegment(currentSegment)
		} else {
			currentSegment = f.lastSegment()
		}
		newFragment := NewFragment()
		currentSegment.AddFragment(newFragment)
		newFragment.AddChild(moof)
	case "mdat":
		mdat := box.(*MdatBox)
		if !f.isFragmented {
			f.Mdat = mdat
		} else {
			currentFragment := f.lastSegment().lastFragment()
			currentFragment.AddChild(mdat)
		}
	}
	f.boxes = append(f.boxes, box)
}

// Dump - print information about
func (f *File) Dump(r io.Reader) {
	if f.isFragmented {
		fmt.Printf("Init segment\n")
		f.Init.Moov.Dump()
		for i, seg := range f.Segments {
			fmt.Printf("  mediaSegment %d\n", i)
			for j, f := range seg.Fragments {
				fmt.Printf("  fragment %d\n ", j)
				w, _ := os.Create("tmp.264")
				defer w.Close()
				f.DumpSampleData(w)
			}
		}

	} else {
		f.Ftyp.Dump()
		f.Moov.Dump()
	}
}

// Boxes - return the top-level boxes from a media
func (f *File) Boxes() []Box {
	return f.boxes
}

// Encode - encode a file to a Writer
func (f *File) Encode(w io.Writer) error {
	for _, b := range f.Boxes() {
		err := b.Encode(w)
		if err != nil {
			return err
		}
	}
	return nil
}

func (f *File) lastSegment() *MediaSegment {
	return f.Segments[len(f.Segments)-1]
}

// IsFragmented - is file made of multiple segments (Mp4 fragments)
func (f *File) IsFragmented() bool {
	return f.isFragmented
}
