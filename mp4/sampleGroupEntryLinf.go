package mp4

import (
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// LinfSampleGroupEntry - Layer Information sample group entry ('linf')
// ISO/IEC 14496-15 Ed. 7 Sec. 4.15 (LayerInfoGroupEntry). For L-HEVC (Sec. 9.6.3)
// IrapGdrPicsInLayerOnlyFlag and CompletenessFlag are required to be 0, but they
// are kept as fields here so the box round-trips for any layered codec.
type LinfSampleGroupEntry struct {
	Layers []LinfLayerEntry
}

// LinfLayerEntry describes a single layer in the linf sample group.
type LinfLayerEntry struct {
	IrapGdrPicsInLayerOnlyFlag bool
	CompletenessFlag           bool
	LayerID                    byte // 6 bits
	MinTemporalID              byte // 3 bits
	MaxTemporalID              byte // 3 bits
	SubLayerPresenceFlags      byte // 7 bits
}

// DecodeLinfSampleGroupEntry decodes a linf sample group entry.
func DecodeLinfSampleGroupEntry(name string, length uint32, sr bits.SliceReader) (SampleGroupEntry, error) {
	e := &LinfSampleGroupEntry{}
	numLayers := int(sr.ReadUint8() & 0x3f) // 2 reserved bits + 6 bits count

	for i := 0; i < numLayers; i++ {
		// 3 bytes per layer:
		//  b1: reserved(2) irap_gdr_pics_in_layer_only_flag(1) completeness_flag(1) layer_id[5:2](4)
		//  b2: layer_id[1:0](2) min_TemporalId(3) max_TemporalId(3)
		//  b3: reserved(1) sub_layer_presence_flags(7)
		b1 := sr.ReadUint8()
		b2 := sr.ReadUint8()
		b3 := sr.ReadUint8()
		l := LinfLayerEntry{
			IrapGdrPicsInLayerOnlyFlag: (b1>>5)&0x01 != 0,
			CompletenessFlag:           (b1>>4)&0x01 != 0,
			LayerID:                    ((b1 & 0x0f) << 2) | ((b2 >> 6) & 0x03),
			MinTemporalID:              (b2 >> 3) & 0x07,
			MaxTemporalID:              b2 & 0x07,
			SubLayerPresenceFlags:      b3 & 0x7f,
		}
		e.Layers = append(e.Layers, l)
	}
	return e, sr.AccError()
}

// Type - grouping type
func (e *LinfSampleGroupEntry) Type() string {
	return "linf"
}

// Size - size of the sample group entry payload
func (e *LinfSampleGroupEntry) Size() uint64 {
	return uint64(1 + len(e.Layers)*3) // num_layers(1) + 3 bytes per layer
}

// Encode - encode the sample group entry to a SliceWriter
func (e *LinfSampleGroupEntry) Encode(sw bits.SliceWriter) {
	sw.WriteUint8(byte(len(e.Layers)) & 0x3f) // 2 reserved bits + 6 bits count

	for _, l := range e.Layers {
		var b1 byte
		if l.IrapGdrPicsInLayerOnlyFlag {
			b1 |= 0x20
		}
		if l.CompletenessFlag {
			b1 |= 0x10
		}
		b1 |= (l.LayerID >> 2) & 0x0f
		sw.WriteUint8(b1)
		b2 := ((l.LayerID & 0x03) << 6) | ((l.MinTemporalID & 0x07) << 3) | (l.MaxTemporalID & 0x07)
		sw.WriteUint8(b2)
		sw.WriteUint8(l.SubLayerPresenceFlags & 0x7f)
	}
}

// Info - write box-specific information
func (e *LinfSampleGroupEntry) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, e, -2, 0)
	bd.write(" - numLayers: %d", len(e.Layers))
	for i, l := range e.Layers {
		bd.write("   Layer[%d]: layerId=%d minTid=%d maxTid=%d subFlags=0x%02x irapGdrOnly=%t complete=%t",
			i, l.LayerID, l.MinTemporalID, l.MaxTemporalID, l.SubLayerPresenceFlags,
			l.IrapGdrPicsInLayerOnlyFlag, l.CompletenessFlag)
	}
	return bd.err
}
