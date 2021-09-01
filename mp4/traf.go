package mp4

import (
	"errors"
	"io"
)

// TrafBox - Track Fragment Box (traf)
//
// Contained in : Movie Fragment Box (moof)
//
type TrafBox struct {
	Tfhd     *TfhdBox
	Tfdt     *TfdtBox
	Trun     *TrunBox // The first TrunBox
	Truns    []*TrunBox
	Children []Box
}

// DecodeTraf - box-specific decode
func DecodeTraf(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	children, err := DecodeContainerChildren(hdr, startPos, startPos+hdr.size, r)
	if err != nil {
		return nil, err
	}
	t := &TrafBox{}
	for _, b := range children {
		err := t.AddChild(b)
		if err != nil {
			return nil, err
		}
	}
	return t, nil
}

// AddChild - add child box
func (t *TrafBox) AddChild(b Box) error {
	switch b.Type() {
	case "tfhd":
		t.Tfhd = b.(*TfhdBox)
	case "tfdt":
		t.Tfdt = b.(*TfdtBox)
	case "trun":
		if t.Trun == nil {
			t.Trun = b.(*TrunBox)
		}
		t.Truns = append(t.Truns, b.(*TrunBox))
	default:
	}
	t.Children = append(t.Children, b)
	return nil
}

// Type - return box type
func (t *TrafBox) Type() string {
	return "traf"
}

// Size - return calculated size
func (t *TrafBox) Size() uint64 {
	return containerSize(t.Children)
}

// GetChildren - list of child boxes
func (t *TrafBox) GetChildren() []Box {
	return t.Children
}

// Encode - write box to w
func (t *TrafBox) Encode(w io.Writer) error {
	return EncodeContainer(t, w)
}

func (t *TrafBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	return ContainerInfo(t, w, specificBoxLevels, indent, indentStep)
}

// OptimizeTfhdTrun - optimize trun by default values in tfhd box
// Only look at first trun, even if there is more than one
// Don't optimize again, if already done so that no data is present
func (t *TrafBox) OptimizeTfhdTrun() error {
	tfhd := t.Tfhd
	trun := t.Trun
	if len(trun.Samples) == 0 {
		return errors.New("No samples in trun")
	}
	if len(trun.Samples) == 1 {
		return nil // No need to optimize
	}

	if trun.HasSampleDuration() {
		hasCommonDur := true
		commonDur := trun.Samples[0].Dur
		for _, s := range trun.Samples {
			if s.Dur != commonDur {
				hasCommonDur = false
				break
			}
		}
		if hasCommonDur {
			// Set defaultSampleDuration in tfhd and remove from trun
			tfhd.Flags = tfhd.Flags | defaultSampleDurationPresent
			tfhd.DefaultSampleDuration = commonDur
			trun.flags = trun.flags & ^sampleDurationPresentFlag
		}
	}

	if trun.HasSampleSize() {
		hasCommonSize := true
		commonSize := trun.Samples[0].Size
		for _, s := range trun.Samples {
			if s.Size != commonSize {
				hasCommonSize = false
				break
			}
		}
		if hasCommonSize {
			// Set defaultSampleSize in tfhd and remove from trun
			tfhd.Flags = tfhd.Flags | defaultSampleSizePresent
			tfhd.DefaultSampleSize = commonSize
			trun.flags = trun.flags & ^sampleSizePresentFlag
		}
	}

	if trun.HasSampleFlags() {
		firstSampleFlags := trun.Samples[0].Flags
		hasCommonFlags := true
		commonSampleFlags := trun.Samples[1].Flags
		for i, s := range trun.Samples {
			if i >= 1 {
				if s.Flags != commonSampleFlags {
					hasCommonFlags = false
					break
				}
			}
		}
		if hasCommonFlags {
			if firstSampleFlags != commonSampleFlags {
				trun.firstSampleFlags = firstSampleFlags
				trun.flags |= firstSampleFlagsPresentFlag
			}
			tfhd.Flags = tfhd.Flags | defaultSampleFlagsPresent
			tfhd.DefaultSampleFlags = commonSampleFlags
			trun.flags = trun.flags & ^sampleFlagsPresentFlag
		}
	}

	if trun.HasSampleCompositionTimeOffset() {
		allZeroCTO := true
		for _, s := range trun.Samples {
			if s.CompositionTimeOffset != 0 {
				allZeroCTO = false
				break
			}
		}
		if allZeroCTO {
			trun.flags = trun.flags & ^sampleCompositionTimeOffsetPresentFlag
		}
	}
	return nil
}
