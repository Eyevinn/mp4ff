package mp4

import (
	"io"

	"github.com/edgeware/mp4ff/bits"
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
func DecodeTrex(hdr boxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
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
func (b *TrexBox) Type() string {
	return "trex"
}

// Size - return calculated size
func (b *TrexBox) Size() uint64 {
	return uint64(boxHeaderSize + 4 + 20)
}

// Encode - write box to w
func (b *TrexBox) Encode(w io.Writer) error {
	sw := bits.NewSliceWriterWithSize(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - box-specific encode to slicewriter
func (b *TrexBox) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	versionAndFlags := (uint32(b.Version) << 24) + b.Flags
	sw.WriteUint32(versionAndFlags)
	sw.WriteUint32(b.TrackID)
	sw.WriteUint32(b.DefaultSampleDescriptionIndex)
	sw.WriteUint32(b.DefaultSampleDuration)
	sw.WriteUint32(b.DefaultSampleSize)
	sw.WriteUint32(b.DefaultSampleFlags)
	return sw.AccError()
}

// Info - write box-specific information
func (b *TrexBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, int(b.Version), b.Flags)
	bd.write(" - trackID: %d", b.TrackID)
	bd.write(" - defaultSampleDescriptionIndex: %d", b.DefaultSampleDescriptionIndex)
	bd.write(" - defaultSampleDuration: %d", b.DefaultSampleDuration)
	bd.write(" - defaultSampleSize: %d", b.DefaultSampleSize)
	bd.write(" - defaultSampleFlags: %08x (%s)", b.DefaultSampleFlags, DecodeSampleFlags(b.DefaultSampleFlags))
	return bd.err
}
