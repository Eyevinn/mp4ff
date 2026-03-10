package mp4

import (
	"io"

	"github.com/Eyevinn/mp4ff/bits"
)

// OinfSampleGroupEntry - Operating Points Information sample group entry (oinf)
// ISO/IEC 14496-15 Section 9.6.2
type OinfSampleGroupEntry struct {
	ScalabilityMask   uint16
	ProfileTierLevels []OinfPTL
	OperatingPoints   []OinfOperatingPoint
	DependencyLayers  []OinfDependencyLayer
}

// OinfPTL - Profile/Tier/Level for an operating point layer
type OinfPTL struct {
	GeneralProfileSpace              byte
	GeneralTierFlag                  bool
	GeneralProfileIDC                byte
	GeneralProfileCompatibilityFlags uint32
	GeneralConstraintIndicatorFlags  uint64 // 48 bits
	GeneralLevelIDC                  byte
}

// OinfOperatingPoint - Operating point description
type OinfOperatingPoint struct {
	OutputLayerSetIdx uint16
	MaxTemporalID     byte
	Layers            []OinfOPLayer
	MinPicWidth       uint16
	MinPicHeight      uint16
	MaxPicWidth       uint16
	MaxPicHeight      uint16
	MaxChromaFormat   byte // 2 bits
	MaxBitDepthMinus8 byte // 3 bits (stored as value, actual = value + 8)
	FrameRateInfoFlag bool
	BitRateInfoFlag   bool
	AvgFrameRate      uint16 // if FrameRateInfoFlag
	ConstantFrameRate byte   // 2 bits, if FrameRateInfoFlag
	MaxBitRate        uint32 // if BitRateInfoFlag
	AvgBitRate        uint32 // if BitRateInfoFlag
}

// OinfOPLayer - Per-layer info within an operating point
type OinfOPLayer struct {
	PtlIdx              byte
	LayerID             byte // 6 bits
	IsOutputLayer       bool
	IsAlternateOutLayer bool
}

// OinfDependencyLayer - Layer dependency description
type OinfDependencyLayer struct {
	LayerID          byte
	DependsOnLayers  []byte
	DimensionIds     []byte // one per set bit in scalability_mask
}

// DecodeOinfSampleGroupEntry decodes an oinf sample group entry
func DecodeOinfSampleGroupEntry(name string, length uint32, sr bits.SliceReader) (SampleGroupEntry, error) {
	e := &OinfSampleGroupEntry{}
	e.ScalabilityMask = sr.ReadUint16()
	reserved6 := sr.ReadUint8()
	numPTLs := int(reserved6 & 0x3f)

	for i := 0; i < numPTLs; i++ {
		ptl := OinfPTL{}
		b := sr.ReadUint8()
		ptl.GeneralProfileSpace = (b >> 6) & 0x03
		ptl.GeneralTierFlag = (b>>5)&0x01 != 0
		ptl.GeneralProfileIDC = b & 0x1f
		ptl.GeneralProfileCompatibilityFlags = sr.ReadUint32()
		hi := uint64(sr.ReadUint16()) << 32
		lo := uint64(sr.ReadUint32())
		ptl.GeneralConstraintIndicatorFlags = hi | lo
		ptl.GeneralLevelIDC = sr.ReadUint8()
		e.ProfileTierLevels = append(e.ProfileTierLevels, ptl)
	}

	numOPs := int(sr.ReadUint16())
	for i := 0; i < numOPs; i++ {
		op := OinfOperatingPoint{}
		op.OutputLayerSetIdx = sr.ReadUint16()
		op.MaxTemporalID = sr.ReadUint8()
		layerCount := int(sr.ReadUint8())
		for j := 0; j < layerCount; j++ {
			l := OinfOPLayer{}
			l.PtlIdx = sr.ReadUint8()
			b := sr.ReadUint8()
			l.LayerID = (b >> 2) & 0x3f
			l.IsOutputLayer = (b>>1)&0x01 != 0
			l.IsAlternateOutLayer = b&0x01 != 0
			op.Layers = append(op.Layers, l)
		}
		op.MinPicWidth = sr.ReadUint16()
		op.MinPicHeight = sr.ReadUint16()
		op.MaxPicWidth = sr.ReadUint16()
		op.MaxPicHeight = sr.ReadUint16()
		flagsByte := sr.ReadUint8()
		op.MaxChromaFormat = (flagsByte >> 6) & 0x03
		op.MaxBitDepthMinus8 = (flagsByte >> 3) & 0x07
		op.FrameRateInfoFlag = (flagsByte>>1)&0x01 != 0
		op.BitRateInfoFlag = flagsByte&0x01 != 0
		if op.FrameRateInfoFlag {
			op.AvgFrameRate = sr.ReadUint16()
			b := sr.ReadUint8()
			op.ConstantFrameRate = (b) & 0x03
		}
		if op.BitRateInfoFlag {
			op.MaxBitRate = sr.ReadUint32()
			op.AvgBitRate = sr.ReadUint32()
		}
		e.OperatingPoints = append(e.OperatingPoints, op)
	}

	numDeps := int(sr.ReadUint8())
	numDims := popcount16(e.ScalabilityMask)
	for i := 0; i < numDeps; i++ {
		dep := OinfDependencyLayer{}
		dep.LayerID = sr.ReadUint8()
		numDepsOn := int(sr.ReadUint8())
		for j := 0; j < numDepsOn; j++ {
			dep.DependsOnLayers = append(dep.DependsOnLayers, sr.ReadUint8())
		}
		for j := 0; j < numDims; j++ {
			dep.DimensionIds = append(dep.DimensionIds, sr.ReadUint8())
		}
		e.DependencyLayers = append(e.DependencyLayers, dep)
	}

	return e, sr.AccError()
}

// Type - grouping type
func (e *OinfSampleGroupEntry) Type() string {
	return "oinf"
}

// Size - size of the entry payload (not including any box header)
func (e *OinfSampleGroupEntry) Size() uint64 {
	size := uint64(3) // scalability_mask(2) + reserved+ptl_count(1)
	size += uint64(len(e.ProfileTierLevels)) * 12
	size += 2 // num_operating_points
	for _, op := range e.OperatingPoints {
		size += 4 // ols_idx(2) + max_tid(1) + layer_count(1)
		size += uint64(len(op.Layers)) * 2
		size += 9 // dimensions(8) + flags(1)
		if op.FrameRateInfoFlag {
			size += 3 // avgFrameRate(2) + reserved+constantFrameRate(1)
		}
		if op.BitRateInfoFlag {
			size += 8 // maxBitRate(4) + avgBitRate(4)
		}
	}
	numDims := popcount16(e.ScalabilityMask)
	size += 1 // num_dependency_layers
	for _, dep := range e.DependencyLayers {
		size += 2 // layer_id(1) + num_deps_on(1)
		size += uint64(len(dep.DependsOnLayers))
		size += uint64(numDims)
	}
	return size
}

// Encode - encode to SliceWriter
func (e *OinfSampleGroupEntry) Encode(sw bits.SliceWriter) {
	sw.WriteUint16(e.ScalabilityMask)
	sw.WriteUint8(byte(len(e.ProfileTierLevels)) & 0x3f)

	for _, ptl := range e.ProfileTierLevels {
		b := (ptl.GeneralProfileSpace << 6)
		if ptl.GeneralTierFlag {
			b |= 0x20
		}
		b |= ptl.GeneralProfileIDC & 0x1f
		sw.WriteUint8(b)
		sw.WriteUint32(ptl.GeneralProfileCompatibilityFlags)
		sw.WriteUint16(uint16(ptl.GeneralConstraintIndicatorFlags >> 32))
		sw.WriteUint32(uint32(ptl.GeneralConstraintIndicatorFlags & 0xFFFFFFFF))
		sw.WriteUint8(ptl.GeneralLevelIDC)
	}

	sw.WriteUint16(uint16(len(e.OperatingPoints)))
	for _, op := range e.OperatingPoints {
		sw.WriteUint16(op.OutputLayerSetIdx)
		sw.WriteUint8(op.MaxTemporalID)
		sw.WriteUint8(byte(len(op.Layers)))
		for _, l := range op.Layers {
			sw.WriteUint8(l.PtlIdx)
			b := (l.LayerID & 0x3f) << 2
			if l.IsOutputLayer {
				b |= 0x02
			}
			if l.IsAlternateOutLayer {
				b |= 0x01
			}
			sw.WriteUint8(b)
		}
		sw.WriteUint16(op.MinPicWidth)
		sw.WriteUint16(op.MinPicHeight)
		sw.WriteUint16(op.MaxPicWidth)
		sw.WriteUint16(op.MaxPicHeight)
		flagsByte := (op.MaxChromaFormat & 0x03) << 6
		flagsByte |= (op.MaxBitDepthMinus8 & 0x07) << 3
		if op.FrameRateInfoFlag {
			flagsByte |= 0x02
		}
		if op.BitRateInfoFlag {
			flagsByte |= 0x01
		}
		sw.WriteUint8(flagsByte)
		if op.FrameRateInfoFlag {
			sw.WriteUint16(op.AvgFrameRate)
			sw.WriteUint8(op.ConstantFrameRate & 0x03)
		}
		if op.BitRateInfoFlag {
			sw.WriteUint32(op.MaxBitRate)
			sw.WriteUint32(op.AvgBitRate)
		}
	}

	sw.WriteUint8(byte(len(e.DependencyLayers)))
	for _, dep := range e.DependencyLayers {
		sw.WriteUint8(dep.LayerID)
		sw.WriteUint8(byte(len(dep.DependsOnLayers)))
		for _, id := range dep.DependsOnLayers {
			sw.WriteUint8(id)
		}
		for _, dim := range dep.DimensionIds {
			sw.WriteUint8(dim)
		}
	}
}

// Info - write oinf info
func (e *OinfSampleGroupEntry) Info(w io.Writer, specificBoxLevels, indent, indentStep string) error {
	bd := newInfoDumper(w, indent, e, -2, 0)
	bd.write(" - scalabilityMask: 0x%04x", e.ScalabilityMask)
	bd.write(" - numProfileTierLevels: %d", len(e.ProfileTierLevels))
	for i, ptl := range e.ProfileTierLevels {
		bd.write("   PTL[%d]: space=%d tier=%t profile=%d level=%d",
			i, ptl.GeneralProfileSpace, ptl.GeneralTierFlag, ptl.GeneralProfileIDC, ptl.GeneralLevelIDC)
	}
	bd.write(" - numOperatingPoints: %d", len(e.OperatingPoints))
	for i, op := range e.OperatingPoints {
		bd.write("   OP[%d]: olsIdx=%d maxTid=%d layers=%d dims=%dx%d-%dx%d",
			i, op.OutputLayerSetIdx, op.MaxTemporalID, len(op.Layers),
			op.MinPicWidth, op.MinPicHeight, op.MaxPicWidth, op.MaxPicHeight)
		for j, l := range op.Layers {
			bd.write("     layer[%d]: ptlIdx=%d layerId=%d output=%t", j, l.PtlIdx, l.LayerID, l.IsOutputLayer)
		}
	}
	bd.write(" - numDependencyLayers: %d", len(e.DependencyLayers))
	for i, dep := range e.DependencyLayers {
		bd.write("   Dep[%d]: layerId=%d dependsOn=%v dimIds=%v", i, dep.LayerID, dep.DependsOnLayers, dep.DimensionIds)
	}
	return bd.err
}

func popcount16(v uint16) int {
	count := 0
	for v != 0 {
		count += int(v & 1)
		v >>= 1
	}
	return count
}
