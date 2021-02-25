package mp4

import (
	"io"
	"io/ioutil"
)

// TfraBox - Track Fragment Random Access Box (tfra)
// Contained it MfraBox (mfra)
type TfraBox struct {
	Version               byte
	Flags                 uint32
	TrackID               uint32
	LengthSizeOfTrafNum   byte
	LengthSizeOfTrunNum   byte
	LengthSizeOfSampleNum byte
	Entries               []TrafEntry
}

// Tfrabox - reference as used inside SidxBox
type TrafEntry struct {
	Time        int64
	MoofOffset  int64
	TrafNumber  uint32
	TrunNumber  uint32
	SampleDelta uint32
}

// DecodeTfra - box-specific decode
func DecodeTfra(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	s := NewSliceReader(data)
	versionAndFlags := s.ReadUint32()
	version := byte(versionAndFlags >> 24)

	b := &TfraBox{
		Version: version,
		Flags:   versionAndFlags & flagsMask,
	}
	b.TrackID = s.ReadUint32()
	sizesBlock := s.ReadUint32()
	b.LengthSizeOfTrafNum = byte((sizesBlock >> 4) & 0x3)
	b.LengthSizeOfTrunNum = byte((sizesBlock >> 2) & 0x3)
	b.LengthSizeOfSampleNum = byte(sizesBlock & 0x3)
	nrEntries := s.ReadUint32()
	for i := uint32(0); i < nrEntries; i++ {
		te := TrafEntry{}
		if b.Version == 1 {
			te.Time = s.ReadInt64()
			te.MoofOffset = s.ReadInt64()
		} else {
			te.Time = int64(s.ReadInt32())
			te.MoofOffset = int64(s.ReadInt32())
		}
		switch b.LengthSizeOfTrafNum {
		case 0:
			te.TrafNumber = uint32(s.ReadUint8())
		case 1:
			te.TrafNumber = uint32(s.ReadUint16())
		case 2:
			te.TrafNumber = uint32(s.ReadUint24())
		case 3:
			te.TrafNumber = s.ReadUint32()
		}
		switch b.LengthSizeOfTrunNum {
		case 0:
			te.TrunNumber = uint32(s.ReadUint8())
		case 1:
			te.TrunNumber = uint32(s.ReadUint16())
		case 2:
			te.TrunNumber = uint32(s.ReadUint24())
		case 3:
			te.TrunNumber = s.ReadUint32()
		}
		switch b.LengthSizeOfSampleNum {
		case 0:
			te.SampleDelta = uint32(s.ReadUint8())
		case 1:
			te.SampleDelta = uint32(s.ReadUint16())
		case 2:
			te.SampleDelta = uint32(s.ReadUint24())
		case 3:
			te.SampleDelta = s.ReadUint32()
		}
		b.Entries = append(b.Entries, te)
	}
	return b, nil
}

// Type - return box type
func (b *TfraBox) Type() string {
	return "tfra"
}

// Size - return calculated size
func (b *TfraBox) Size() uint64 {
	// Add up all fields depending on version
	nrEntries := len(b.Entries)
	size := (boxHeaderSize + 4 + 12 + // Up to number_of_entry
		8*(1+int(b.Version))*nrEntries +
		int(1+b.LengthSizeOfTrafNum+
			1+b.LengthSizeOfTrunNum+
			1+b.LengthSizeOfSampleNum)*nrEntries)
	return uint64(size)

}

// Encode - write box to w
func (b *TfraBox) Encode(w io.Writer) error {
	err := EncodeHeader(b, w)
	if err != nil {
		return err
	}
	buf := makebuf(b)
	sw := NewSliceWriter(buf)
	versionAndFlags := (uint32(b.Version) << 24) + b.Flags
	sw.WriteUint32(versionAndFlags)
	sw.WriteUint32(b.TrackID)
	sizesBlock := uint32(b.LengthSizeOfTrafNum<<4 + b.LengthSizeOfTrunNum<<2 + b.LengthSizeOfSampleNum)
	sw.WriteUint32(sizesBlock)
	sw.WriteUint32(uint32(len(b.Entries)))
	for _, e := range b.Entries {

		if b.Version == 1 {
			sw.WriteInt64(e.Time)
			sw.WriteInt64(e.MoofOffset)
		} else {
			sw.WriteInt32(int32(e.Time))
			sw.WriteInt32(int32(e.MoofOffset))
		}
		switch b.LengthSizeOfTrafNum {
		case 0:
			sw.WriteUint8(byte(e.TrafNumber))
		case 1:
			sw.WriteUint16(uint16(e.TrafNumber))
		case 2:
			sw.WriteUint24(uint32(e.TrafNumber))
		case 3:
			sw.WriteUint32(uint32(e.TrafNumber))
		}
		switch b.LengthSizeOfTrunNum {
		case 0:
			sw.WriteUint8(byte(e.TrunNumber))
		case 1:
			sw.WriteUint16(uint16(e.TrunNumber))
		case 2:
			sw.WriteUint24(uint32(e.TrunNumber))
		case 3:
			sw.WriteUint32(uint32(e.TrunNumber))
		}
		switch b.LengthSizeOfSampleNum {
		case 0:
			sw.WriteUint8(byte(e.SampleDelta))
		case 1:
			sw.WriteUint16(uint16(e.SampleDelta))
		case 2:
			sw.WriteUint24(uint32(e.SampleDelta))
		case 3:
			sw.WriteUint32(uint32(e.SampleDelta))
		}
	}
	_, err = w.Write(buf)
	return err
}

//Info - more info for level 1
func (b *TfraBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, int(b.Version), b.Flags)
	bd.write(" - trackID: %d", b.TrackID)
	bd.write(" - nrEntries: %d", len(b.Entries))
	level := getInfoLevel(b, specificBoxLevels)
	if level >= 1 {
		for i, e := range b.Entries {
			bd.write(" - %d: time=%d moofOffset=%d trafNr=%d trunNr=%d sampleDelta=%d",
				i+1, e.Time, e.MoofOffset, e.TrafNumber, e.TrunNumber, e.SampleDelta)
		}
	}
	return bd.err
}
