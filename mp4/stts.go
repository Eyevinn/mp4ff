package mp4

import (
	"encoding/binary"
	"fmt"
	"io"
	"time"

	"github.com/Eyevinn/mp4ff/bits"
)

// SttsBox -  Decoding Time to Sample Box (stts - mandatory)
//
// This table contains the duration in time units for each sample.
//
//   - SampleCount : the number of consecutive samples having the same duration
//   - SampleTimeDelta : duration in time units
type SttsBox struct {
	Version         byte
	Flags           uint32
	SampleCount     []uint32
	SampleTimeDelta []uint32
}

// DecodeStts - box-specific decode
func DecodeStts(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	sr := bits.NewFixedSliceReader(data)
	return DecodeSttsSR(hdr, startPos, sr)
}

// DecodeSttsSR - box-specific decode
func DecodeSttsSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	versionAndFlags := sr.ReadUint32()
	entryCount := sr.ReadUint32()

	b := SttsBox{
		Version: byte(versionAndFlags >> 24),
		Flags:   versionAndFlags & flagsMask,
	}

	if hdr.Size != b.expectedSize(entryCount) {
		return nil, fmt.Errorf("stts: expected size %d, got %d", b.expectedSize(entryCount), hdr.Size)
	}

	// Bulk-read the table: one slice-reader call for all entries instead
	// of one per entry, then decode in a tight loop.
	data := sr.ReadBytes(8 * int(entryCount))
	if sr.AccError() != nil {
		return nil, sr.AccError()
	}
	b.SampleCount = make([]uint32, entryCount)
	b.SampleTimeDelta = make([]uint32, entryCount)
	for i := 0; i < int(entryCount); i++ {
		b.SampleCount[i] = binary.BigEndian.Uint32(data[8*i:])
		b.SampleTimeDelta[i] = binary.BigEndian.Uint32(data[8*i+4:])
	}
	return &b, nil
}

// Type - return box type
func (b *SttsBox) Type() string {
	return "stts"
}

// Size - return calculated size
func (b *SttsBox) Size() uint64 {
	return b.expectedSize(uint32(len(b.SampleCount)))
}

// expectedSize - calculate size for a given entry count
func (b *SttsBox) expectedSize(entryCount uint32) uint64 {
	// 8 = version + flags(4) + entryCount(4)
	// 8 = sampleCount(4) + sampleTimeDelta(4)
	return uint64(boxHeaderSize + 8 + uint64(entryCount)*8)
}

// GetTimeCode - return the timecode (duration since the beginning of the media)
// of the beginning of a sample
func (b *SttsBox) GetTimeCode(sample, timescale uint32) time.Duration {
	sample--
	var units uint32
	i := 0
	for sample > 0 && i < len(b.SampleCount) {
		if sample >= b.SampleCount[i] {
			units += b.SampleCount[i] * b.SampleTimeDelta[i]
			sample -= b.SampleCount[i]
		} else {
			units += sample * b.SampleTimeDelta[i]
			sample = 0
		}
		i++
	}
	return time.Second * time.Duration(units) / time.Duration(timescale)
}

// GetDecodeTime - decode time and duration for (one-based) sampleNr in track timescale.
// An error is returned if sampleNr is zero, or if the stts entries cover fewer samples
// than sampleNr (a file whose stts and stsz disagree on the sample count).
func (b *SttsBox) GetDecodeTime(sampleNr uint32) (decTime uint64, dur uint32, err error) {
	if sampleNr == 0 {
		return 0, 0, fmt.Errorf("sampleNr is 0, but sample numbers are one-based")
	}
	samplesRemaining := sampleNr - 1
	decTime = 0
	i := 0
	for {
		if i >= len(b.SampleTimeDelta) {
			return 0, 0, fmt.Errorf("stts entries cover fewer samples than sampleNr %d", sampleNr)
		}
		dur = b.SampleTimeDelta[i]
		if samplesRemaining >= b.SampleCount[i] {
			decTime += uint64(b.SampleCount[i]) * uint64(dur)
			samplesRemaining -= b.SampleCount[i]
		} else {
			if samplesRemaining > 0 {
				decTime += uint64(samplesRemaining) * uint64(dur)
			}
			break
		}
		i++
	}
	return decTime, dur, nil
}

// SampleDurations returns the duration of each of nrSamples samples,
// validating that the entries cover exactly that many samples.
func (b *SttsBox) SampleDurations(nrSamples uint32) ([]uint32, error) {
	durs := make([]uint32, 0, nrSamples)
	for i := range b.SampleCount {
		if uint64(len(durs))+uint64(b.SampleCount[i]) > uint64(nrSamples) {
			return nil, fmt.Errorf("stts covers more than the %d declared samples", nrSamples)
		}
		for j := uint32(0); j < b.SampleCount[i]; j++ {
			durs = append(durs, b.SampleTimeDelta[i])
		}
	}
	if uint32(len(durs)) != nrSamples {
		return nil, fmt.Errorf("stts covers %d samples, not the %d declared", len(durs), nrSamples)
	}
	return durs, nil
}

// GetDur - get dur for a specific sample
func (b *SttsBox) GetDur(sampleNr uint32) (dur uint32) {
	if sampleNr == 0 {
		// This is bad index input. Should never happen
		panic("SttsBox.GetDur called with sampleNr == 0, although one-based")
	}
	sampleNr-- // one-based -> zero-based
	i := 0
	for i < len(b.SampleCount) {
		dur = b.SampleTimeDelta[i]

		if sampleNr >= b.SampleCount[i] {
			sampleNr -= b.SampleCount[i]
		} else {
			return dur
		}
		i++
	}
	return dur
}

// Encode - write box to w
func (b *SttsBox) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - box-specific encode to slicewriter
func (b *SttsBox) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	versionAndFlags := (uint32(b.Version) << 24) + b.Flags
	sw.WriteUint32(versionAndFlags)
	sw.WriteUint32(uint32(len(b.SampleCount)))
	for i := range b.SampleCount {
		sw.WriteUint32(b.SampleCount[i])
		sw.WriteUint32(b.SampleTimeDelta[i])
	}
	return sw.AccError()
}

// Info - write box-specific information
func (b *SttsBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, int(b.Version), b.Flags)
	if len(b.SampleCount) > 0 {
		bd.write(" - sampleCount: %d", len(b.SampleCount))
	}
	level := getInfoLevel(b, specificBoxLevels)
	if level >= 1 {
		for i := range b.SampleCount {
			bd.write(" - entry[%d]: sampleCount=%d sampleDelta=%d",
				i+1, b.SampleCount[i], b.SampleTimeDelta[i])
		}
	}
	return bd.err
}

// GetSampleNrAtTime returns the 1-based sample number at or as soon as possible after time.
// Match a final single zero duration if present.
// If time is too big to reach, an error is returned.
// Time is calculated by summing up durations of previous samples
func (b *SttsBox) GetSampleNrAtTime(sampleStartTime uint64) (sampleNr uint32, err error) {
	accTime := uint64(0)
	accNr := uint32(0)
	nrEntries := len(b.SampleCount)
	if nrEntries == 0 {
		return 0, fmt.Errorf("stts box has no entries")
	}
	for i := 0; i < nrEntries; i++ {
		timeDelta := uint64(b.SampleTimeDelta[i])
		if sampleStartTime < accTime+uint64(b.SampleCount[i])*timeDelta {
			relTime := (sampleStartTime - accTime)
			nrInInterval := relTime / timeDelta
			if relTime%timeDelta != 0 { // If not exact, increase number to next sample
				nrInInterval++
			}
			return accNr + uint32(nrInInterval) + 1, nil
		}
		accNr += b.SampleCount[i]
		accTime += timeDelta * uint64(b.SampleCount[i])
	}
	// Check if there is a final single zero duration and time matches.
	if b.SampleTimeDelta[nrEntries-1] == 0 && b.SampleCount[nrEntries-1] == 1 &&
		sampleStartTime == accTime {
		return accNr, nil
	}
	return 0, fmt.Errorf("no matching sample found for time=%d", sampleStartTime)
}
