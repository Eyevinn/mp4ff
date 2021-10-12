package hevc

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/edgeware/mp4ff/bits"
)

// HEVC errors
var (
	ErrLengthSize = errors.New("Can only handle 4byte NALU length size")
)

// DecConfRec - HEVCDecoderConfigurationRecord
// Specified in ISO/IEC 14496-15 4't ed 2017 Sec. 8.3.3
type DecConfRec struct {
	ConfigurationVersion             byte
	GeneralProfileSpace              byte
	GeneralTierFlag                  bool
	GeneralProfileIDC                byte
	GeneralProfileCompatibilityFlags uint32
	GeneralConstraintIndicatorFlags  uint64
	GeneralLevelIDC                  byte
	MinSpatialSegmentationIDC        uint16
	ParallellismType                 byte
	ChromaFormatIDC                  byte
	BitDepthLumaMinus8               byte
	BitDepthChromaMinus8             byte
	AvgFrameRate                     uint16
	ConstantFrameRate                byte
	NumTemporalLayers                byte
	TemporalIDNested                 byte
	LengthSizeMinusOne               byte
	NaluArrays                       []NaluArray
}

// NaluArray - HEVC NALU array including complete bit and type
type NaluArray struct {
	completeAndType byte
	Nalus           [][]byte
}

// NewNaluArray - create an HEVC NaluArray
func NewNaluArray(complete bool, naluType NaluType, nalus [][]byte) *NaluArray {
	var completeBit byte
	if complete {
		completeBit = 0x80
	}
	return &NaluArray{
		completeAndType: completeBit | byte(naluType),
		Nalus:           nalus,
	}
}

// NaluType - return NaluType for NaluArray
func (n *NaluArray) NaluType() NaluType {
	return NaluType(n.completeAndType & 0x3f)
}

// Complete - return 0x1 if complete
func (n *NaluArray) Complete() byte {
	return n.completeAndType >> 7
}

// CreateHEVCDecConfRec - extract information from vps, sps, pps and fill HEVCDecConfRec with that
func CreateHEVCDecConfRec(vpsNalus, spsNalus, ppsNalus [][]byte, vpsComplete, spsComplete, ppsComplete bool) (DecConfRec, error) {
	sps, err := ParseSPSNALUnit(spsNalus[0])
	if err != nil {
		return DecConfRec{}, err
	}
	var naluArrays []NaluArray
	naluArrays = append(naluArrays, *NewNaluArray(vpsComplete, NALU_VPS, vpsNalus))
	naluArrays = append(naluArrays, *NewNaluArray(spsComplete, NALU_SPS, spsNalus))
	naluArrays = append(naluArrays, *NewNaluArray(ppsComplete, NALU_PPS, ppsNalus))
	ptf := sps.ProfileTierLevel
	return DecConfRec{
		ConfigurationVersion:             1,
		GeneralProfileSpace:              ptf.GeneralProfileSpace,
		GeneralTierFlag:                  ptf.GeneralTierFlag,
		GeneralProfileIDC:                ptf.GeneralProfileIDC,
		GeneralProfileCompatibilityFlags: ptf.GeneralProfileCompatibilityFlags,
		GeneralConstraintIndicatorFlags:  ptf.GeneralConstraintIndicatorFlags,
		GeneralLevelIDC:                  ptf.GeneralLevelIDC,
		MinSpatialSegmentationIDC:        0, // Set as default value
		ParallellismType:                 0, // Set as default value
		ChromaFormatIDC:                  sps.ChromaFormatIDC,
		BitDepthLumaMinus8:               sps.BitDepthLumaMinus8,
		BitDepthChromaMinus8:             sps.BitDepthChromaMinus8,
		AvgFrameRate:                     0,          // Set as default value
		ConstantFrameRate:                0,          // Set as default value
		NumTemporalLayers:                0,          // Set as default value
		TemporalIDNested:                 0,          // Set as default value
		LengthSizeMinusOne:               3,          // only support 4-byte length
		NaluArrays:                       naluArrays, // VPS, SPS, PPS nalus with complete flag
	}, nil
}

// DecodeHEVCDecConfRec - decode an HEVCDecConfRec
func DecodeHEVCDecConfRec(r io.Reader) (DecConfRec, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return DecConfRec{}, err
	}
	hdcr := DecConfRec{}
	sr := bits.NewSliceReader(data)
	hdcr.ConfigurationVersion = sr.ReadUint8()
	if hdcr.ConfigurationVersion != 1 {
		return DecConfRec{}, fmt.Errorf("HEVC decoder configuration record version %d unknown",
			hdcr.ConfigurationVersion)
	}
	aByte := sr.ReadUint8()
	hdcr.GeneralProfileSpace = (aByte >> 6) & 0x3
	hdcr.GeneralTierFlag = (aByte>>5)&0x1 == 0x1
	hdcr.GeneralProfileIDC = aByte & 0x1f
	hdcr.GeneralProfileCompatibilityFlags = sr.ReadUint32()
	hdcr.GeneralConstraintIndicatorFlags = (uint64(sr.ReadUint32()) << 16) | uint64(sr.ReadUint16())
	hdcr.GeneralLevelIDC = sr.ReadUint8()
	hdcr.MinSpatialSegmentationIDC = sr.ReadUint16() & 0x0fff
	hdcr.ParallellismType = sr.ReadUint8() & 0x3
	hdcr.ChromaFormatIDC = sr.ReadUint8() & 0x3
	hdcr.BitDepthLumaMinus8 = sr.ReadUint8() & 0x7
	hdcr.BitDepthChromaMinus8 = sr.ReadUint8() & 0x7
	hdcr.AvgFrameRate = sr.ReadUint16()
	aByte = sr.ReadUint8()
	hdcr.ConstantFrameRate = (aByte >> 6) & 0x3
	hdcr.NumTemporalLayers = (aByte >> 3) & 0x7
	hdcr.TemporalIDNested = (aByte >> 2) & 0x1
	hdcr.LengthSizeMinusOne = aByte & 0x3
	if hdcr.LengthSizeMinusOne != 3 {
		return hdcr, ErrLengthSize
	}
	numArrays := sr.ReadUint8()
	for j := 0; j < int(numArrays); j++ {
		array := NaluArray{
			completeAndType: sr.ReadUint8(),
			Nalus:           nil,
		}
		numNalus := int(sr.ReadUint16())
		for i := 0; i < numNalus; i++ {
			naluLength := int(sr.ReadUint16())
			array.Nalus = append(array.Nalus, sr.ReadBytes(naluLength))
		}
		hdcr.NaluArrays = append(hdcr.NaluArrays, array)
	}
	return hdcr, sr.AccError()
}

// Size - total size in bytes
func (h *DecConfRec) Size() uint64 {
	totalSize := 23 // Up to and including numArrays
	for _, array := range h.NaluArrays {
		totalSize += 3 // complete + nalu type + num nalus
		for _, nalu := range array.Nalus {
			totalSize += 2 // nal unit length
			totalSize += len(nalu)
		}
	}
	return uint64(totalSize)
}

// Encode - write an HEVCDecConfRec to w
func (h *DecConfRec) Encode(w io.Writer) error {
	aw := bits.NewAccErrByteWriter(w)
	aw.WriteUint8(h.ConfigurationVersion)
	var generalTierFlagBit byte
	if h.GeneralTierFlag {
		generalTierFlagBit = 1 << 5
	}
	aw.WriteUint8(h.GeneralProfileSpace<<6 | generalTierFlagBit | h.GeneralProfileIDC)
	aw.WriteUint32(h.GeneralProfileCompatibilityFlags)
	aw.WriteUint48(h.GeneralConstraintIndicatorFlags)
	aw.WriteUint8(h.GeneralLevelIDC)
	aw.WriteUint16(0xf000 | h.MinSpatialSegmentationIDC)
	aw.WriteUint8(0xfc | h.ParallellismType)
	aw.WriteUint8(0xfc | h.ChromaFormatIDC)
	aw.WriteUint8(0xf8 | h.BitDepthLumaMinus8)
	aw.WriteUint8(0xf8 | h.BitDepthChromaMinus8)
	aw.WriteUint16(h.AvgFrameRate)
	aw.WriteUint8(h.ConstantFrameRate<<6 | h.NumTemporalLayers<<3 | h.TemporalIDNested<<2 | h.LengthSizeMinusOne)
	aw.WriteUint8(byte(len(h.NaluArrays)))
	for _, array := range h.NaluArrays {
		aw.WriteUint8(array.completeAndType)
		aw.WriteUint16(uint16(len(array.Nalus)))
		for _, nalu := range array.Nalus {
			aw.WriteUint16(uint16(len(nalu)))
			aw.WriteSlice(nalu)
		}
	}
	return aw.AccError()
}

// GetNalusForType - get all nalus for a specific naluType
func (h *DecConfRec) GetNalusForType(naluType NaluType) [][]byte {
	for _, naluArray := range h.NaluArrays {
		if naluArray.NaluType() == naluType {
			return naluArray.Nalus
		}
	}
	return nil
}
