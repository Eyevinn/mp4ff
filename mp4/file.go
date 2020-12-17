package mp4

import (
	"fmt"
	"io"
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
// If fragmented, the data is instead in Init and/or Segments.
//
// segments.
type File struct {
	Ftyp         *FtypBox        // Only used for non-fragmented files
	Moov         *MoovBox        // Only used for non-fragmented files
	Mdat         *MdatBox        // Only used for non-fragmented files
	Init         *InitSegment    // Init data (ftyp + moov for fragmented file)
	Sidx         *SidxBox        // SidxBox for a DASH OnDemand file
	Segments     []*MediaSegment // Media segment
	Children     []Box           // All top-level boxes in order
	isFragmented bool
}

// NewFile - create MP4 file
func NewFile() *File {
	return &File{
		Children: []Box{},
		Segments: []*MediaSegment{},
	}
}

// ReadMP4File - read an mp4 file from path
func ReadMP4File(path string) (*File, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	mp4Root, err := DecodeFile(f)
	if err != nil {
		return nil, err
	}
	return mp4Root, nil
}

// BoxStructure represent a box or similar entity such as a Segment
type BoxStructure interface {
	Encode(w io.Writer) error
}

// WriteToFile - write a box structure to a file at filePath
func WriteToFile(boxStructure BoxStructure, filePath string) error {
	ofd, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer ofd.Close()
	err = boxStructure.Encode(ofd)
	return err
}

// AddMediaSegment - add a mediasegment to file f
func (f *File) AddMediaSegment(m *MediaSegment) {
	f.Segments = append(f.Segments, m)
}

// DecodeFile - parse and decode a file from reader r
func DecodeFile(r io.Reader) (*File, error) {
	f := NewFile()
	var boxStartPos uint64 = 0
	lastBoxType := ""

LoopBoxes:
	for {
		box, err := DecodeBox(boxStartPos, r)
		if err == io.EOF {
			break LoopBoxes
		}
		if err != nil {
			return nil, err
		}
		boxType, boxSize := box.Type(), box.Size()
		if err != nil {
			return nil, err
		}
		if boxType == "mdat" {
			if f.isFragmented {
				if lastBoxType != "moof" {
					return nil, fmt.Errorf("Does not support %v between moof and mdat", lastBoxType)
				}
			}
		}
		f.AddChild(box, boxStartPos)
		lastBoxType = boxType
		boxStartPos += boxSize
	}
	return f, nil
}

// AddChild - add child with start position
func (f *File) AddChild(box Box, boxStartPos uint64) {
	switch box.Type() {
	case "ftyp":
		f.Ftyp = box.(*FtypBox)
	case "moov":
		f.Moov = box.(*MoovBox)
		if len(f.Moov.Trak.Mdia.Minf.Stbl.Stts.SampleCount) == 0 {
			f.isFragmented = true
			f.Init = NewMP4Init()
			f.Init.Ftyp = f.Ftyp
			f.Init.Moov = f.Moov
		}
	case "sidx":
		if len(f.Segments) == 0 { // sidx before first styp
			f.Sidx = box.(*SidxBox)
		} else {
			currSeg := f.Segments[len(f.Segments)-1]
			currSeg.Sidx = box.(*SidxBox)
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
			currentSegment = f.LastSegment()
		}
		newFragment := NewFragment()
		currentSegment.AddFragment(newFragment)
		newFragment.AddChild(moof)
	case "mdat":
		mdat := box.(*MdatBox)
		if !f.isFragmented {
			f.Mdat = mdat
		} else {
			currentFragment := f.LastSegment().LastFragment()
			currentFragment.AddChild(mdat)
		}
	}
	f.Children = append(f.Children, box)
}

// DumpWithSampleData - print information about file and its children boxes
func (f *File) DumpWithSampleData(w io.Writer) error {
	if f.isFragmented {
		fmt.Printf("Init segment\n")
		err := f.Init.Dump(w, "  ")
		if err != nil {
			return err
		}
		for i, seg := range f.Segments {
			fmt.Printf("  mediaSegment %d\n", i)
			for j, frag := range seg.Fragments {
				fmt.Printf("  fragment %d\n ", j)
				w, err := os.OpenFile("tmp.264", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
				if err != nil {
					return err
				}
				err = frag.DumpSampleData(w, f.Init.Moov.Mvex.Trex)
				if err != nil {
					w.Close()
					return err
				}
				w.Close()
			}
		}

	} else {
		err := f.Ftyp.Dump(w, "", "  ")
		if err != nil {
			return err
		}
		err = f.Moov.Dump(w, "", "  ")
		if err != nil {
			return err
		}
	}

	return nil
}

// Encode - encode a file to a Writer
func (f *File) Encode(w io.Writer) error {
	for _, b := range f.Children {
		err := b.Encode(w)
		if err != nil {
			return err
		}
	}
	return nil
}

// Dump - write box tree with indent for each level
func (f *File) Dump(w io.Writer, indent string) error {
	for _, box := range f.Children {
		err := box.Dump(w, "", indent)
		if err != nil {
			return err
		}
	}
	return nil
}

// LastSegment - Currently last segment
func (f *File) LastSegment() *MediaSegment {
	return f.Segments[len(f.Segments)-1]
}

// IsFragmented - is file made of multiple segments (Mp4 fragments)
func (f *File) IsFragmented() bool {
	return f.isFragmented
}
