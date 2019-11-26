package mp4

import (
	"fmt"
	"io"
	"log"
)

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
func (f *Fragment) AddChild(b Box) {
	switch b.Type() {
	case "prft":
		f.Prft = b.(*PrftBox)
	case "moof":
		f.Moof = b.(*MoofBox)
	case "mdat":
		f.Mdat = b.(*MdatBox)
	}
	f.boxes = append(f.boxes, b)
}

// GetSampleData - Get all samples including data
func (f *Fragment) GetSampleData() []*SampleComplete {
	moof := f.Moof
	mdat := f.Mdat
	seqNr := moof.Mfhd.SequenceNumber
	fmt.Printf(" Got samples for Segment %d\n", seqNr)
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
	return samples
}

// AddSample - add a complete sample to a fragment
func (f *Fragment) AddSample(s *SampleComplete) {
	trun := f.Moof.Traf.Trun
	if trun.SampleCount() == 0 {
		tfdt := f.Moof.Traf.Tfdt
		tfdt.SetBaseMediaDecodeTime(s.DecodeTime)
	}
	trun.AddSample(&s.Sample)
	mdat := f.Mdat
	mdat.AddSampleData(s.Data)
}

// DumpSampleData - Get Sample data and print out
func (f *Fragment) DumpSampleData(w io.Writer) {
	samples := f.GetSampleData()
	for i, s := range samples {
		if i < 9 {
			fmt.Printf("%4d %8d %8d %6x %d %d\n", i, s.DecodeTime, s.PresentationTime, s.Flags, s.Size, len(s.Data))
		}
		toAnnexB(s.Data)
		if w != nil {
			w.Write(s.Data)
		}
	}
}
