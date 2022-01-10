package mp4

import (
	"io"

	"github.com/edgeware/mp4ff/bits"
)

// MediaSegment - MP4 Media Segment
type MediaSegment struct {
	Styp        *StypBox
	Sidx        *SidxBox // Sidx for a segment
	Fragments   []*Fragment
	EncOptimize EncOptimize
}

// NewMediaSegment - create empty MediaSegment with CMAF styp box
func NewMediaSegment() *MediaSegment {
	return &MediaSegment{
		Styp:        CreateStyp(),
		Fragments:   nil,
		EncOptimize: OptimizeNone,
	}
}

// NewMediaSegmentWithoutStyp - create empty media segment with no styp box
func NewMediaSegmentWithoutStyp() *MediaSegment {
	return &MediaSegment{
		Styp:        nil,
		Fragments:   nil,
		EncOptimize: OptimizeNone,
	}
}

// AddFragment - Add a fragment to a MediaSegment
func (s *MediaSegment) AddFragment(f *Fragment) {
	s.Fragments = append(s.Fragments, f)
}

// LastFragment - Currently last fragment
func (s *MediaSegment) LastFragment() *Fragment {
	return s.Fragments[len(s.Fragments)-1]
}

// Size - return size of media segment
func (s *MediaSegment) Size() uint64 {
	var size uint64 = 0
	if s.Styp != nil {
		size += s.Styp.Size()
	}
	if s.Sidx != nil {
		size += s.Sidx.Size()
	}
	for _, f := range s.Fragments {
		size += f.Size()
	}
	return size
}

// Encode - Write MediaSegment via writer
func (s *MediaSegment) Encode(w io.Writer) error {
	if s.Styp != nil {
		err := s.Styp.Encode(w)
		if err != nil {
			return err
		}
	}
	if s.Sidx != nil {
		err := s.Sidx.Encode(w)
		if err != nil {
			return err
		}
	}
	for _, f := range s.Fragments {
		f.EncOptimize = s.EncOptimize
		err := f.Encode(w)
		if err != nil {
			return err
		}
	}
	return nil
}

// EncodeSW - Write MediaSegment via SliceWriter
func (s *MediaSegment) EncodeSW(sw bits.SliceWriter) error {
	if s.Styp != nil {
		err := s.Styp.EncodeSW(sw)
		if err != nil {
			return err
		}
	}
	if s.Sidx != nil {
		err := s.Sidx.EncodeSW(sw)
		if err != nil {
			return err
		}
	}
	for _, f := range s.Fragments {
		f.EncOptimize = s.EncOptimize
		err := f.EncodeSW(sw)
		if err != nil {
			return err
		}
	}
	return nil
}

// Info - write box tree with indent for each level
func (s *MediaSegment) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	if s.Styp != nil {
		err := s.Styp.Info(w, specificBoxLevels, indent, indentStep)
		if err != nil {
			return err
		}
	}
	if s.Sidx != nil {
		err := s.Sidx.Info(w, specificBoxLevels, indent, indentStep)
		if err != nil {
			return err
		}
	}
	for _, f := range s.Fragments {
		err := f.Info(w, specificBoxLevels, indent, indentStep)
		if err != nil {
			return err
		}
	}
	return nil
}

// Fragmentify - Split into multiple fragments. Assume single mdat and trun for now
func (s *MediaSegment) Fragmentify(timescale uint64, trex *TrexBox, duration uint32) ([]*Fragment, error) {
	inFragments := s.Fragments
	outFragments := make([]*Fragment, 0)
	var of *Fragment

	var cumDur uint32 = 0

	for _, inFrag := range inFragments {
		trackID := inFrag.Moof.Traf.Tfhd.TrackID

		samples, err := inFrag.GetFullSamples(trex)
		if err != nil {
			return nil, err
		}
		for _, s := range samples {
			if cumDur == 0 {
				var err error
				of, err = CreateFragment(inFrag.Moof.Mfhd.SequenceNumber, trackID)
				if err != nil {
					return nil, err
				}
				outFragments = append(outFragments, of)
			}
			//of.AddFullSample(s)
			err = of.AddFullSampleToTrack(s, trackID)
			if err != nil {
				return nil, err
			}
			cumDur += s.Dur
			if cumDur >= duration {
				// fmt.Printf("Wrote fragment with duration %d\n", cumDur)
				cumDur = 0
			}
		}
	}
	return outFragments, nil
}
