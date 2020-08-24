package mp4

import (
	"fmt"
	"io"

	log "github.com/sirupsen/logrus"
)

// Fragment - MP4 Fragment ([prft] + moof + mdat)
type Fragment struct {
	Prft  *PrftBox
	Moof  *MoofBox
	Mdat  *MdatBox
	boxes []Box
}

// NewFragment - New emtpy one-track MP4 Fragment
func NewFragment() *Fragment {
	return &Fragment{}
}

// CreateFragment - create emtpy fragment with one single track for output
func CreateFragment(seqNumber uint32, trackID uint32) (*Fragment, error) {
	f := NewFragment()
	moof := &MoofBox{}
	f.AddChild(moof)
	mfhd := CreateMfhd(seqNumber)
	_ = moof.AddChild(mfhd)
	traf := &TrafBox{}
	_ = moof.AddChild(traf) // Can only have error when adding second track
	tfhd := CreateTfhd(trackID)
	_ = traf.AddChild(tfhd)
	tfdt := &TfdtBox{} // Data will be provided by first sample
	_ = traf.AddChild(tfdt)
	trun := CreateTrun()
	_ = traf.AddChild(trun)
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

// GetFullSamples - Get full samples including media and accumulated time
func (f *Fragment) GetFullSamples(trex *TrexBox) []*FullSample {
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
	samples := trun.GetFullSamples(uint32(offsetInMdat), baseTime, f.Mdat)
	return samples
}

// AddFullSample - add a full sample to a fragment
func (f *Fragment) AddFullSample(s *FullSample) {
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
	samples := f.GetFullSamples(trex)
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
	traf := f.Moof.Traf
	err := traf.OptimizeTfhdTrun()
	if err != nil {
		return err
	}
	trun := f.Moof.Traf.Trun
	if trun.HasDataOffset() {
		// media data start with respect to start of moof
		// Valid when writing single track with single trun
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
