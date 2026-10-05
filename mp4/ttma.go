package mp4

import (
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// DefaultMaxActivationSeconds is the maximum period of activation, T_MPA, that a
// TtmaBox should give, as in ETSI EN 303 560 for TTML in MPEG-2 TS.
const DefaultMaxActivationSeconds = 5

// TtmaBox - TimedTextMaxActivationBox (ttma), in an stpc or wvtc sample entry.
// MaxActivationPeriod is T_MPA in the timescale of the media header box. When no
// later sample arrives, the active document or cues end T_MPA after the composition
// time of the latest sample, or at the end of that sample if it lasts longer.
// Experimental: the 4CC is not registered with MP4RA.
//
// Contained in : XMLSubtitleSampleEntry (stpc) or WVTTSampleEntry (wvtc)
type TtmaBox struct {
	Version             byte
	Flags               uint32
	MaxActivationPeriod uint32
}

// CreateTtma - create a TtmaBox with T_MPA given in the media timescale
func CreateTtma(maxActivationPeriod uint32) *TtmaBox {
	return &TtmaBox{MaxActivationPeriod: maxActivationPeriod}
}

// DecodeTtma - box-specific decode
func DecodeTtma(hdr BoxHeader, startPos uint64, r io.Reader) (Box, error) {
	data, err := readBoxBody(r, hdr)
	if err != nil {
		return nil, err
	}
	sr := bits.NewFixedSliceReader(data)
	return DecodeTtmaSR(hdr, startPos, sr)
}

// DecodeTtmaSR - box-specific decode
func DecodeTtmaSR(hdr BoxHeader, startPos uint64, sr bits.SliceReader) (Box, error) {
	versionAndFlags := sr.ReadUint32()
	return &TtmaBox{
		Version:             byte(versionAndFlags >> 24),
		Flags:               versionAndFlags & flagsMask,
		MaxActivationPeriod: sr.ReadUint32(),
	}, sr.AccError()
}

// Type - box type
func (b *TtmaBox) Type() string {
	return "ttma"
}

// Size - calculated size of box
func (b *TtmaBox) Size() uint64 {
	return boxHeaderSize + 8
}

// Encode - write box to w
func (b *TtmaBox) Encode(w io.Writer) error {
	sw := bits.NewFixedSliceWriter(int(b.Size()))
	err := b.EncodeSW(sw)
	if err != nil {
		return err
	}
	_, err = w.Write(sw.Bytes())
	return err
}

// EncodeSW - box-specific encode to slicewriter
func (b *TtmaBox) EncodeSW(sw bits.SliceWriter) error {
	err := EncodeHeaderSW(b, sw)
	if err != nil {
		return err
	}
	versionAndFlags := (uint32(b.Version) << 24) + b.Flags
	sw.WriteUint32(versionAndFlags)
	sw.WriteUint32(b.MaxActivationPeriod)
	return sw.AccError()
}

// Info - write box-specific information
func (b *TtmaBox) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, b, int(b.Version), b.Flags)
	bd.write(" - maxActivationPeriod: %d", b.MaxActivationPeriod)
	return bd.err
}
