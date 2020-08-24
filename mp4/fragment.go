package mp4

import (
	"fmt"
	"io"

	log "github.com/sirupsen/logrus"
)

// Fragment - MP4 Fragment (moof + mdat)
type Fragment struct {
	Prft        *PrftBox
	Moof        *MoofBox
	Mdat        *MdatBox
	Independent bool
	boxes       []Box
}

// NewFragment - New emtpy MP4 Fragment
func NewFragment() *Fragment {
	return &Fragment{}
}

// CreateFragment - create emtpy fragment for output
func CreateFragment(seqNumber uint32, trackID uint32) (*Fragment, error) {
	f := NewFragment()
	moof := &MoofBox{}
	f.AddChild(moof)
	mfhd := CreateMfhd(seqNumber)
	moof.AddChild(mfhd)
	traf := &TrafBox{}
	err := moof.AddChild(traf)
	if err != nil {
		return nil, err
	}
	tfhd := CreateTfhd(trackID)
	traf.AddChild(tfhd)
	tfdt := &TfdtBox{} // Data will be provided by first sample
	traf.AddChild(tfdt)
	trun := CreateTrun()
	traf.AddChild(trun)
	mdat := &MdatBox{}
	f.AddChild(mdat)

	return f, nil
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

// GetCompleteSamples - Get complete samples including data and accumulated time
func (f *Fragment) GetCompleteSamples(trex *TrexBox) []*SampleComplete {
	moof := f.Moof
	mdat := f.Mdat
	seqNr := moof.Mfhd.SequenceNumber
	log.Debugf("Got samples for Segment %d\n", seqNr)
	tfhd := moof.Traf.Tfhd
	baseTime := moof.Traf.Tfdt.BaseMediaDecodeTime
	trun := moof.Traf.Trun
	moofStartPos := moof.StartPos
	mdatStartPos := mdat.StartPos
	trun.AddSampleDefaultValues(tfhd, trex)
	var baseOffset uint64
	if tfhd.HasBaseDataOffset() {
		baseOffset = tfhd.BaseDataOffset
	} else if tfhd.DefaultBaseIfMoof() {
		baseOffset = moofStartPos
	}
	if trun.HasDataOffset() {
		baseOffset = uint64(int64(trun.DataOffset) + int64(baseOffset))
	}
	mdatDataLength := uint64(len(mdat.Data)) // TODO Make len take 64-bit number
	offsetInMdat := baseOffset - mdatStartPos - headerLength(mdatDataLength)
	if offsetInMdat > mdatDataLength {
		log.Fatalf("Offset in mdata beyond size")
	}
	samples := trun.GetCompleteSamples(uint32(offsetInMdat), baseTime, f.Mdat)
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
func (f *Fragment) DumpSampleData(w io.Writer, trex *TrexBox) error {
	samples := f.GetCompleteSamples(trex)
	for i, s := range samples {
		if i < 9 {
			fmt.Printf("%4d %8d %8d %6x %d %d\n", i, s.DecodeTime, s.PresentationTime(),
				s.Flags, s.Size, len(s.Data))
		}
		toAnnexB(s.Data)
		if w != nil {
			_, err := w.Write(s.Data)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// Encode - write fragment via writer
func (f *Fragment) Encode(w io.Writer) error {
	trun := f.Moof.Traf.Trun
	if trun.HasDataOffset() {
		// Make documentation or other stuff clear about that
		trun.DataOffset = int32(f.Moof.Size() + 8)
	}
	for _, b := range f.boxes {
		err := b.Encode(w)
		if err != nil {
			return err
		}
	}
	return nil
}

// Boxes - return children boxes
func (f *Fragment) Boxes() []Box {
	return f.boxes
}
