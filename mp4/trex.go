package mp4

import (
	"io"
	"io/ioutil"
)

// TrexBox - Track Extends Box
//
// Contained in : Mvex Box (mvex)
type TrexBox struct {
	Version                       byte
	Flags                         uint32
	TrackID                       uint32
	DefaultSampleDescriptionIndex uint32
	DefaultSampleDuration         uint32
	DefaultSampleSize             uint32
	DefaultSampleFlags            uint32
}

// CreateTrex - create trex box with trackID
func CreateTrex(trackID uint32) *TrexBox {
	return &TrexBox{
		TrackID:                       trackID,
		DefaultSampleDescriptionIndex: 1,
	}
}

// DecodeTrex - box-specific decode
func DecodeTrex(hdr *boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	s := NewSliceReader(data)
	versionAndFlags := s.ReadUint32()

	b := &TrexBox{
		Version:                       byte(versionAndFlags >> 24),
		Flags:                         versionAndFlags & flagsMask,
		TrackID:                       s.ReadUint32(),
		DefaultSampleDescriptionIndex: s.ReadUint32(),
		DefaultSampleDuration:         s.ReadUint32(),
		DefaultSampleSize:             s.ReadUint32(),
		DefaultSampleFlags:            s.ReadUint32(), // interpreted as SampleFlags
	}
	return b, nil
}

// Type - return box type
func (t *TrexBox) Type() string {
	return "trex"
}

// Size - return calculated size
func (t *TrexBox) Size() uint64 {
	return uint64(boxHeaderSize + 4 + 20)
}

// Encode - write box to w
func (t *TrexBox) Encode(w io.Writer) error {
	err := EncodeHeader(t, w)
	if err != nil {
		return err
	}
	buf := makebuf(t)
	sw := NewSliceWriter(buf)
	versionAndFlags := (uint32(t.Version) << 24) + t.Flags
	sw.WriteUint32(versionAndFlags)
	sw.WriteUint32(t.TrackID)
	sw.WriteUint32(t.DefaultSampleDescriptionIndex)
	sw.WriteUint32(t.DefaultSampleDuration)
	sw.WriteUint32(t.DefaultSampleSize)
	sw.WriteUint32(t.DefaultSampleFlags)
	_, err = w.Write(buf)
	return err
}

func (t *TrexBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, t, int(t.Version), t.Flags)
	bd.write(" - trackID: %d", t.TrackID)
	bd.write(" - defaultSampleDescriptionIndex: %d", t.DefaultSampleDescriptionIndex)
	bd.write(" - defaultSampleDuration: %d", t.DefaultSampleDuration)
	bd.write(" - defaultSampleSize: %d", t.DefaultSampleSize)
	bd.write(" - defaultSampleFlags: %08x (%s)", t.DefaultSampleFlags, DecodeSampleFlags(t.DefaultSampleFlags))
	return bd.err
}
