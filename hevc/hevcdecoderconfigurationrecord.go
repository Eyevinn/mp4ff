package hevc

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/edgeware/mp4ff/bits"
)

var ErrLengthSize = errors.New("Can only handle 4byte NALU length size")

//HEVCDecConfRec - HEVCDecoderConfigurationRecord
// Specified in ISO/IEC 14496-15 4't ed 2017 Sec. 8.3.3
type HEVCDecConfRec struct {
	ConfigurationVersion             byte
	GeneralProfileSpace              byte
	GeneralTierFlag                  byte
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

type NaluArray struct {
	completeAndType byte
	Nalus           [][]byte
}

func (n *NaluArray) NewNaluArray(complete byte, naluType NaluType, nalus [][]byte) *NaluArray {
	return &NaluArray{
		completeAndType: complete<<7 | byte(naluType),
		Nalus:           nalus,
	}
}

func (n *NaluArray) NaluType() NaluType {
	return NaluType(n.completeAndType & 0x3f)
}

func (n *NaluArray) Complete() byte {
	return n.completeAndType >> 7
}

// DecodeHEVCDecConfRec - decode an HEVCDecConfRec
func DecodeHEVCDecConfRec(r io.Reader) (HEVCDecConfRec, error) {

	data, err := ioutil.ReadAll(r)
	if err != nil {
		return HEVCDecConfRec{}, err
	}
	hdcr := HEVCDecConfRec{}
	sr := bits.NewSliceReader(data)
	configurationVersion := sr.ReadUint8()
	if configurationVersion != 1 {
		return HEVCDecConfRec{}, fmt.Errorf("HEVC decoder configuration record version %d unknown",
			configurationVersion)
	}
	aByte := sr.ReadUint8()
	hdcr.GeneralProfileSpace = (aByte >> 6) & 0x3
	hdcr.GeneralTierFlag = (aByte >> 5) & 0x1
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

func (h *HEVCDecConfRec) Size() uint64 {
	totalSize := 23 // Up to and including numArrays
	for _, array := range h.NaluArrays {
		totalSize += 1 // complete + nalu type
		for _, nalu := range array.Nalus {
			totalSize += len(nalu)
		}
	}
	return uint64(totalSize)
}

// Encode - write an HEVCDecConfRec to w
func (h *HEVCDecConfRec) Encode(w io.Writer) error {
	aw := bits.NewAccErrByteWriter(w)
	aw.WriteUint8(h.ConfigurationVersion)
	aw.WriteUint8(h.GeneralProfileSpace<<6 | h.GeneralTierFlag<<5 | h.GeneralProfileIDC)
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
			aw.WriteSlice(nalu)
		}
	}
	return aw.AccError()
}

// GetNalusForType - get all nalus for a specific naluType
func (h *HEVCDecConfRec) GetNalusForType(naluType NaluType) [][]byte {
	for _, naluArray := range h.NaluArrays {
		if naluArray.NaluType() == naluType {
			return naluArray.Nalus
		}
	}
	return nil
}
