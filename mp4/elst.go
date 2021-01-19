package mp4

import (
	"errors"
	"io"
	"io/ioutil"
)

// ElstBox - Edit List Box (elst - optional)
//
// Contained in : Edit Box (edts)
type ElstBox struct {
	Version           byte
	Flags             uint32
	SegmentDuration   []uint64
	MediaTime         []int64
	MediaRateInteger  []int16
	MediaRateFraction []int16
}

// DecodeElst - box-specific decode
func DecodeElst(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	s := NewSliceReader(data)
	versionAndFlags := s.ReadUint32()
	version := byte(versionAndFlags >> 24)
	b := &ElstBox{
		Version:           version,
		Flags:             versionAndFlags & flagsMask,
		SegmentDuration:   []uint64{},
		MediaTime:         []int64{},
		MediaRateInteger:  []int16{},
		MediaRateFraction: []int16{},
	}

	entryCount := s.ReadUint32()
	if version == 1 {
		for i := 0; i < int(entryCount); i++ {
			b.SegmentDuration = append(b.SegmentDuration, s.ReadUint64())
			b.MediaTime = append(b.MediaTime, s.ReadInt64())
			b.MediaRateInteger = append(b.MediaRateInteger, s.ReadInt16())
			b.MediaRateFraction = append(b.MediaRateFraction, s.ReadInt16())
		}
	} else if version == 0 {
		for i := 0; i < int(entryCount); i++ {
			b.SegmentDuration = append(b.SegmentDuration, uint64(s.ReadUint32()))
			b.MediaTime = append(b.MediaTime, int64(s.ReadInt32()))
			b.MediaRateInteger = append(b.MediaRateInteger, s.ReadInt16())
			b.MediaRateFraction = append(b.MediaRateFraction, s.ReadInt16())
		}
	} else {
		return nil, errors.New("Unknown version for elst")
	}
	return b, nil
}

// Type - box type
func (b *ElstBox) Type() string {
	return "elst"
}

// Size - calculated size of box
func (b *ElstBox) Size() uint64 {
	if b.Version == 1 {
		return uint64(boxHeaderSize + 8 + len(b.SegmentDuration)*20)
	}
	return uint64(boxHeaderSize + 8 + len(b.SegmentDuration)*12) // m.Version == 0
}

// Encode - write box to w
func (b *ElstBox) Encode(w io.Writer) error {
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	buf := make([]byte, b.Size()-boxHeaderSize)
	sw := NewSliceWriter(buf)
	versionAndFlags := (uint32(b.Version) << 24) + b.Flags
	sw.WriteUint32(versionAndFlags)
	sw.WriteUint32(uint32(len(b.SegmentDuration)))
	if b.Version == 1 {
		for i := range b.SegmentDuration {
			sw.WriteUint64(b.SegmentDuration[i])
			sw.WriteInt64(b.MediaTime[i])
			sw.WriteInt16(b.MediaRateInteger[i])
			sw.WriteInt16(b.MediaRateFraction[i])
		}
	} else {
		for i := range b.SegmentDuration {
			sw.WriteUint32(uint32(b.SegmentDuration[i]))
			sw.WriteInt32(int32(b.MediaTime[i]))
			sw.WriteInt16(b.MediaRateInteger[i])
			sw.WriteInt16(b.MediaRateFraction[i])
		}
	}

	_, err = w.Write(buf)
	return err
}

func (b *ElstBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, int(b.Version), b.Flags)
	for i := 0; i < len(b.SegmentDuration); i++ {
		bd.write("- entry[%d]: segmentDuration=%d mediaTime=%d, mediaRateInteger=%d "+
			"mediaRateFraction=%d", i+1, b.SegmentDuration[i], b.MediaTime[i],
			b.MediaRateInteger[i], b.MediaRateFraction[i])
	}
	return bd.err
}
