package mp4

import (
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// LbliSampleGroupEntry - External base layer sample group entry ('lbli')
// (LhvcExternalBaseLayerInfo, ISO/IEC 14496-15 Ed. 7 Sec. 9.6.1). Used for L-HEVC
// tracks that predict from an external base layer carried in a separate track
// (referenced via 'sbas'), i.e. when the active VPS has
// vps_base_layer_internal_flag == 0 and vps_base_layer_available_flag == 1.
type LbliSampleGroupEntry struct {
	BlIrapPicFlag     bool
	BlIrapNalUnitType byte // 6 bits
	SampleOffset      int8
}

// DecodeLbliSampleGroupEntry decodes an lbli sample group entry.
func DecodeLbliSampleGroupEntry(name string, length uint32, sr bits.SliceReader) (SampleGroupEntry, error) {
	b := sr.ReadUint8() // reserved(1) + bl_irap_pic_flag(1) + bl_irap_nal_unit_type(6)
	e := &LbliSampleGroupEntry{
		BlIrapPicFlag:     (b>>6)&0x01 != 0,
		BlIrapNalUnitType: b & 0x3f,
		SampleOffset:      int8(sr.ReadUint8()),
	}
	return e, sr.AccError()
}

// Type - grouping type
func (e *LbliSampleGroupEntry) Type() string {
	return "lbli"
}

// Size - size of the sample group entry payload
func (e *LbliSampleGroupEntry) Size() uint64 {
	return 2 // flags(1) + sample_offset(1)
}

// Encode - encode the sample group entry to a SliceWriter
func (e *LbliSampleGroupEntry) Encode(sw bits.SliceWriter) {
	b := byte(0x80) // reserved bit = '1'
	if e.BlIrapPicFlag {
		b |= 0x40
	}
	b |= e.BlIrapNalUnitType & 0x3f
	sw.WriteUint8(b)
	sw.WriteUint8(byte(e.SampleOffset))
}

// Info - write box-specific information
func (e *LbliSampleGroupEntry) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, e, -2, 0)
	bd.write(" - blIrapPicFlag: %t", e.BlIrapPicFlag)
	bd.write(" - blIrapNalUnitType: %d", e.BlIrapNalUnitType)
	bd.write(" - sampleOffset: %d", e.SampleOffset)
	return bd.err
}
