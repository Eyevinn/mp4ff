package mp4

import (
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// LinfSampleGroupEntry - Layer Information sample group entry (linf)
// ISO/IEC 14496-15 Section 9.6.3
type LinfSampleGroupEntry struct {
	Layers []LinfLayerEntry
}

// LinfLayerEntry describes a single layer in the linf sample group.
type LinfLayerEntry struct {
	LayerID               byte // 6 bits
	MinTemporalID         byte // 3 bits
	MaxTemporalID         byte // 3 bits
	SubLayerPresenceFlags byte // 7 bits
}

// DecodeLinfSampleGroupEntry decodes a linf sample group entry
func DecodeLinfSampleGroupEntry(name string, length uint32, sr bits.SliceReader) (SampleGroupEntry, error) {
	e := &LinfSampleGroupEntry{}
	b0 := sr.ReadUint8()
	numLayers := int(b0 & 0x3f)

	for i := 0; i < numLayers; i++ {
		l := LinfLayerEntry{}
		// 3 bytes per layer: 4 reserved + 6 layer_id + 3 min_tid + 3 max_tid + 1 reserved + 7 sub_layer_flags = 24 bits
		b1 := sr.ReadUint8()
		b2 := sr.ReadUint8()
		b3 := sr.ReadUint8()
		l.LayerID = ((b1 & 0x0f) << 2) | ((b2 >> 6) & 0x03)
		l.MinTemporalID = (b2 >> 3) & 0x07
		l.MaxTemporalID = b2 & 0x07
		l.SubLayerPresenceFlags = b3 & 0x7f
		e.Layers = append(e.Layers, l)
	}
	return e, sr.AccError()
}

// Type - grouping type
func (e *LinfSampleGroupEntry) Type() string {
	return "linf"
}

// Size - size of the entry payload
func (e *LinfSampleGroupEntry) Size() uint64 {
	return uint64(1 + len(e.Layers)*3) // header(1) + 3 bytes per layer
}

// Encode - encode to SliceWriter
func (e *LinfSampleGroupEntry) Encode(sw bits.SliceWriter) {
	sw.WriteUint8(byte(len(e.Layers)) & 0x3f) // 2 reserved + 6 bits count

	for _, l := range e.Layers {
		// Byte 1: 4 reserved + high 4 bits of layer_id
		b1 := (l.LayerID >> 2) & 0x0f
		sw.WriteUint8(b1)
		// Byte 2: low 2 bits of layer_id + 3 min_tid + 3 max_tid
		b2 := ((l.LayerID & 0x03) << 6) | ((l.MinTemporalID & 0x07) << 3) | (l.MaxTemporalID & 0x07)
		sw.WriteUint8(b2)
		// Byte 3: 1 reserved + 7 sub_layer_flags
		sw.WriteUint8(l.SubLayerPresenceFlags & 0x7f)
	}
}

// Info - write linf info
func (e *LinfSampleGroupEntry) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, e, -2, 0)
	bd.write(" - numLayers: %d", len(e.Layers))
	for i, l := range e.Layers {
		bd.write("   Layer[%d]: layerId=%d minTid=%d maxTid=%d subFlags=0x%02x",
			i, l.LayerID, l.MinTemporalID, l.MaxTemporalID, l.SubLayerPresenceFlags)
	}
	return bd.err
}
