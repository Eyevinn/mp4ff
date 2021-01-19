package mp4

import (
	"encoding/binary"
	"io"
	"io/ioutil"
	"log"
	"time"
)

// SttsBox -  Decoding Time to Sample Box (stts - mandatory)
//
// This table contains the duration in time units for each sample.
//
//   * SampleCount : the number of consecutive samples having the same duration
//   * SampleTimeDelta : duration in time units
type SttsBox struct {
	Version         byte
	Flags           uint32
	SampleCount     []uint32
	SampleTimeDelta []uint32
}

// DecodeStts - box-specific decode
func DecodeStts(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	versionAndFlags := binary.BigEndian.Uint32(data[0:4])
	b := &SttsBox{
		Version:         byte(versionAndFlags >> 24),
		Flags:           versionAndFlags & flagsMask,
		SampleCount:     []uint32{},
		SampleTimeDelta: []uint32{},
	}
	ec := binary.BigEndian.Uint32(data[4:8])
	for i := 0; i < int(ec); i++ {
		sCount := binary.BigEndian.Uint32(data[(8 + 8*i):(12 + 8*i)])
		sDelta := binary.BigEndian.Uint32(data[(12 + 8*i):(16 + 8*i)])
		b.SampleCount = append(b.SampleCount, sCount)
		b.SampleTimeDelta = append(b.SampleTimeDelta, sDelta)
	}
	return b, nil
}

// Type - return box type
func (b *SttsBox) Type() string {
	return "stts"
}

// Size - return calculated size
func (b *SttsBox) Size() uint64 {
	return uint64(boxHeaderSize + 8 + len(b.SampleCount)*8)
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

// GetDecodeTime - decode time and duration for (one-based) sampleNr in track timescale
func (b *SttsBox) GetDecodeTime(sampleNr uint32) (decTime uint64, dur uint32) {
	if sampleNr == 0 {
		// This is bad index input. Should not happen
		log.Print("ERROR: SttsBox.GetDecodeTime called with sampleNr == 0, although one-based")
		return 0, 1
	}
	sampleNr-- // one-based
	decTime = 0
	i := 0
	for sampleNr > 0 && i < len(b.SampleCount) {
		dur = b.SampleTimeDelta[i]

		if sampleNr >= b.SampleCount[i] {
			decTime += uint64(b.SampleCount[i] * dur)
			sampleNr -= b.SampleCount[i]
		} else {
			decTime += uint64(sampleNr * dur)
			sampleNr = 0
		}
		i++
	}
	return decTime, dur
}

// Encode - write box to w
func (b *SttsBox) Encode(w io.Writer) error {
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	buf := makebuf(b)
	sw := NewSliceWriter(buf)
	versionAndFlags := (uint32(b.Version) << 24) + b.Flags
	sw.WriteUint32(versionAndFlags)
	sw.WriteUint32(uint32(len(b.SampleCount)))
	for i := range b.SampleCount {
		sw.WriteUint32(b.SampleCount[i])
		sw.WriteUint32(b.SampleTimeDelta[i])
	}
	_, err = w.Write(buf)
	return err
}
func (b *SttsBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, int(b.Version), b.Flags)
	level := getInfoLevel(b, specificBoxLevels)
	if level >= 1 {
		for i := range b.SampleCount {
			bd.write(" - entry[%d]: sampleCount=%d sampleDelta=%d",
				i+1, b.SampleCount[i], b.SampleTimeDelta[i])
		}
	}
	return bd.err
}
