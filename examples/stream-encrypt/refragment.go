package main

import (
	"fmt"

	"github.com/Eyevinn/mp4ff/mp4"
)

type RefragmentConfig struct {
	SamplesPerFrag uint32
}

func processFragment(
	inputFrag *mp4.Fragment,
	sa mp4.SampleAccessor,
	config RefragmentConfig,
	writeFunc func(*mp4.Fragment) error,
) error {
	trackID := inputFrag.Moof.Traf.Tfhd.TrackID
	totalSamples := getTotalSampleCount(inputFrag.Moof.Traf.Trun)

	if config.SamplesPerFrag == 0 || totalSamples <= config.SamplesPerFrag {
		samples, err := sa.GetSamples(trackID)
		if err != nil {
			return err
		}

		inputFrag.Mdat.Data = nil
		inputFrag.Mdat.SetLazyDataSize(0)
		for _, s := range samples {
			inputFrag.Mdat.AddSampleData(s.Data)
		}

		return writeFunc(inputFrag)
	}

	isFirstSubFrag := true
	for startNr := uint32(1); startNr <= totalSamples; {
		endNr := min(startNr+config.SamplesPerFrag-1, totalSamples)

		samples, err := sa.GetSampleRange(trackID, startNr, endNr)
		if err != nil {
			return fmt.Errorf("GetSampleRange(%d, %d): %w", startNr, endNr, err)
		}

		outFrag, err := createFragmentFromSamples(inputFrag, samples, startNr, endNr, isFirstSubFrag)
		if err != nil {
			return fmt.Errorf("createFragmentFromSamples: %w", err)
		}

		if err := writeFunc(outFrag); err != nil {
			return err
		}

		startNr = endNr + 1
		isFirstSubFrag = false
	}

	return nil
}

func getTotalSampleCount(trun *mp4.TrunBox) uint32 {
	return trun.SampleCount()
}

func createFragmentFromSamples(
	inputFrag *mp4.Fragment,
	samples []mp4.FullSample,
	_startNr, _endNr uint32,
	isFirstSubFrag bool,
) (*mp4.Fragment, error) {
	if len(samples) == 0 {
		return nil, fmt.Errorf("no samples provided")
	}

	newFrag := mp4.NewFragment()

	for _, child := range inputFrag.Children {
		switch child.Type() {
		case "styp":
			if isFirstSubFrag {
				newFrag.AddChild(child)
			}
		case "sidx", "emsg", "prft":
			newFrag.AddChild(child)
		}
	}

	seqNum := inputFrag.Moof.Mfhd.SequenceNumber
	tfhd := inputFrag.Moof.Traf.Tfhd
	trackID := tfhd.TrackID

	moof := &mp4.MoofBox{}
	mfhd := mp4.CreateMfhd(seqNum)
	_ = moof.AddChild(mfhd)

	traf := &mp4.TrafBox{}
	_ = moof.AddChild(traf)

	newTfhd := mp4.CreateTfhd(trackID)
	if tfhd.HasDefaultSampleDuration() {
		newTfhd.DefaultSampleDuration = tfhd.DefaultSampleDuration
	}
	if tfhd.HasDefaultSampleSize() {
		newTfhd.DefaultSampleSize = tfhd.DefaultSampleSize
	}
	if tfhd.HasDefaultSampleFlags() {
		newTfhd.DefaultSampleFlags = tfhd.DefaultSampleFlags
	}
	_ = traf.AddChild(newTfhd)

	tfdt := mp4.CreateTfdt(samples[0].DecodeTime)
	_ = traf.AddChild(tfdt)

	trun := mp4.CreateTrun(0)

	mdat := &mp4.MdatBox{}

	for _, fullSample := range samples {
		trun.AddSample(fullSample.Sample)
		mdat.AddSampleData(fullSample.Data)
	}

	_ = traf.AddChild(trun)

	newFrag.AddChild(moof)
	newFrag.AddChild(mdat)

	return newFrag, nil
}

func min(a, b uint32) uint32 {
	if a < b {
		return a
	}
	return b
}
