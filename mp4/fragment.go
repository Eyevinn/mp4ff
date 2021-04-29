package mp4

import (
	"errors"
	"fmt"
	"io"
	"sort"
)

// Fragment - MP4 Fragment ([prft] + moof + mdat)
type Fragment struct {
	Prft        *PrftBox
	Moof        *MoofBox
	Mdat        *MdatBox
	Children    []Box       // All top-level boxes in order
	nextTrunNr  uint32      // To handle multi-trun cases
	EncOptimize EncOptimize // Bit field with optimizations being done at encoding
}

// NewFragment - New emtpy one-track MP4 Fragment
func NewFragment() *Fragment {
	return &Fragment{}
}

// CreateFragment - create single track empty fragment
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
	trun := CreateTrun(f.nextTrunNr)
	f.nextTrunNr++
	_ = traf.AddChild(trun)
	mdat := &MdatBox{}
	f.AddChild(mdat)

	return f, nil
}

// CreateFragment - create multi-track empty fragment without trun
func CreateMultiTrackFragment(seqNumber uint32, trackIDs []uint32) (*Fragment, error) {
	f := NewFragment()
	moof := &MoofBox{}
	f.AddChild(moof)
	mfhd := CreateMfhd(seqNumber)
	_ = moof.AddChild(mfhd)
	for _, trackID := range trackIDs {
		traf := &TrafBox{}
		_ = moof.AddChild(traf) // Can only have error when adding second track
		tfhd := CreateTfhd(trackID)
		_ = traf.AddChild(tfhd)
		tfdt := &TfdtBox{} // Data will be provided by first sample
		_ = traf.AddChild(tfdt)
		// Don't add trun, but let that happen in write order
	}
	mdat := &MdatBox{}
	f.AddChild(mdat)

	return f, nil
}

// AddChild - Add a top-level box to Fragment
func (f *Fragment) AddChild(b Box) {
	switch b.Type() {
	case "prft":
		f.Prft = b.(*PrftBox)
	case "moof":
		f.Moof = b.(*MoofBox)
	case "mdat":
		f.Mdat = b.(*MdatBox)
	}
	f.Children = append(f.Children, b)
}

// GetFullSamples - Get full samples including media and accumulated time
func (f *Fragment) GetFullSamples(trex *TrexBox) ([]*FullSample, error) {
	moof := f.Moof
	mdat := f.Mdat
	//seqNr := moof.Mfhd.SequenceNumber
	var traf *TrafBox
	foundTrak := false
	if trex != nil {
		for _, traf = range moof.Trafs {
			if traf.Tfhd.TrackID == trex.TrackID {
				foundTrak = true
				break
			}
		}
		if !foundTrak {
			return nil, nil // This trackID may not exist for this fragment
		}
	} else {
		traf = moof.Traf // The first one
	}
	tfhd := traf.Tfhd
	baseTime := traf.Tfdt.BaseMediaDecodeTime
	moofStartPos := moof.StartPos
	var samples []*FullSample
	for _, trun := range traf.Truns {
		totalDur := trun.AddSampleDefaultValues(tfhd, trex)
		var baseOffset uint64
		if tfhd.HasBaseDataOffset() {
			baseOffset = tfhd.BaseDataOffset
		} else if tfhd.DefaultBaseIfMoof() {
			baseOffset = moofStartPos
		}
		if trun.HasDataOffset() {
			baseOffset = uint64(int64(trun.DataOffset) + int64(baseOffset))
		}
		mdatDataLength := uint64(len(mdat.Data)) // len should be fine for 64-bit
		offsetInMdat := baseOffset - mdat.PayloadAbsoluteOffset()
		if offsetInMdat > mdatDataLength {
			return nil, errors.New("Offset in mdata beyond size")
		}
		samples = append(samples, trun.GetFullSamples(uint32(offsetInMdat), baseTime, mdat)...)
		baseTime += totalDur // Next trun start after this
	}

	return samples, nil
}

// AddFullSample - add a full sample to first (and only) trun of a track
// AddFullSampleToTrack is the more general function
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

// AddFullSampleToTrack - allows for adding samples to any track
// New trun boxes will be created if latest trun of fragment is not in this track
func (f *Fragment) AddFullSampleToTrack(s *FullSample, trackID uint32) error {
	var traf *TrafBox
	for _, traf = range f.Moof.Trafs {
		if traf.Tfhd.TrackID == trackID {
			break
		}
	}
	if traf == nil {
		return fmt.Errorf("No track with trackID=%d", trackID)
	}
	if len(traf.Truns) == 0 { // Create first trun if needed
		trun := CreateTrun(f.nextTrunNr)
		f.nextTrunNr++
		err := traf.AddChild(trun)
		if err != nil {
			return err
		}
	}
	if len(traf.Truns) == 1 && traf.Trun.SampleCount() == 0 {
		tfdt := traf.Tfdt
		tfdt.SetBaseMediaDecodeTime(s.DecodeTime)
	}
	trun := traf.Truns[len(traf.Truns)-1] // latest of this track
	if trun.writeOrderNr != f.nextTrunNr-1 {
		// We are not in the latest trun. Must make a new one
		trun = CreateTrun(f.nextTrunNr)
		f.nextTrunNr++
		err := traf.AddChild(trun)
		if err != nil {
			return err
		}
	}
	trun.AddSample(&s.Sample)
	mdat := f.Mdat
	mdat.AddSampleData(s.Data)
	return nil
}

// DumpSampleData - Get Sample data and print out
func (f *Fragment) DumpSampleData(w io.Writer, trex *TrexBox) error {
	samples, err := f.GetFullSamples(trex)
	if err != nil {
		return err
	}
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
	if f.Moof == nil {
		return fmt.Errorf("moof not set in fragment")
	}
	traf := f.Moof.Traf
	if f.EncOptimize&OptimizeTrun != 0 {
		err := traf.OptimizeTfhdTrun()
		if err != nil {
			return err
		}
	}
	if f.Mdat == nil {
		return fmt.Errorf("mdat not set in fragment")
	}
	f.SetTrunDataOffsets()
	for _, b := range f.Children {
		err := b.Encode(w)
		if err != nil {
			return err
		}
	}
	return nil
}

func (f *Fragment) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	for _, box := range f.Children {
		err := box.Info(w, specificBoxLevels, indent, indentStep)
		if err != nil {
			return err
		}
	}
	return nil
}

// GetChildren - return children boxes
func (f *Fragment) GetChildren() []Box {
	return f.Children
}

// SetTrunDataOffsets - set DataOffset in trun depending on size and writeOrder
func (f *Fragment) SetTrunDataOffsets() {
	var truns []*TrunBox
	for _, traf := range f.Moof.Trafs {
		truns = append(truns, traf.Truns...)
	}
	sort.Slice(truns, func(i, j int) bool {
		return truns[i].writeOrderNr < truns[j].writeOrderNr
	})
	dataOffset := f.Moof.Size() + f.Mdat.HeaderSize()
	for _, trun := range truns {
		trun.DataOffset = int32(dataOffset)
		dataOffset += trun.SizeOfData()
	}
}
