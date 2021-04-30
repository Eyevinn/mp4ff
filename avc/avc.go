package avc

import (
	"encoding/binary"
	"fmt"
)

// NaluType - AVC NAL unit type
type NaluType uint16

const (
	// NALU_NON_IDR - Non-IDR Slice NAL unit
	NALU_NON_IDR = NaluType(1)
	// NALU_IDR - IDR Random Access Slice NAL Unit
	NALU_IDR = NaluType(5)
	// NALU_SEI - Supplementary Enhancement Information NAL Unit
	NALU_SEI = NaluType(6)
	// NALU_SSP - SequenceParameterSet NAL Unit
	NALU_SPS = NaluType(7)
	// NALU_PPS - PictureParameterSet NAL Unit
	NALU_PPS = NaluType(8)
	// NALU_AUD - AccessUnitDelimiter NAL Unit
	NALU_AUD = NaluType(9)
	// NALU_EO_SEQ - End of Sequence NAL Unit
	NALU_EO_SEQ = NaluType(10)
	// NALU_EO_STREAM - End of Stream NAL Unit
	NALU_EO_STREAM = NaluType(11)
	// NALU_FILL - Filler NAL Unit
	NALU_FILL = NaluType(12)
)

func (a NaluType) String() string {
	switch a {
	case NALU_NON_IDR:
		return "NonIDR_1"
	case NALU_IDR:
		return "IDR_5"
	case NALU_SEI:
		return "SEI_6"
	case NALU_SPS:
		return "SPS_7"
	case NALU_PPS:
		return "PPS_8"
	case NALU_AUD:
		return "AUD_9"
	case NALU_EO_SEQ:
		return "EndOfSequence_10"
	case NALU_EO_STREAM:
		return "EndOfStream_11"
	case NALU_FILL:
		return "FILL_12"
	default:
		return fmt.Sprintf("Other_%d", a)
	}
}

// Get NaluType from NAL Unit Header byte
func GetNaluType(naluHeader byte) NaluType {
	return NaluType(naluHeader & 0x1f)
}

// FindNaluTypes - find list of NAL unit types in sample
func FindNaluTypes(sample []byte) []NaluType {
	naluList := make([]NaluType, 0)
	length := len(sample)
	if length < 4 {
		return naluList
	}
	var naluLength uint32 = 0
	for naluLength < uint32(length-4) {
		nalLength := binary.BigEndian.Uint32(sample[naluLength : naluLength+4])
		naluLength += 4
		nalType := NaluType(sample[naluLength] & 0x1f)
		naluList = append(naluList, nalType)
		naluLength += nalLength
	}
	return naluList
}

// IsIDRSample - does sample contain IDR NALU
func IsIDRSample(sample []byte) bool {
	return ContainsNaluType(sample, NALU_IDR)
}

// ContainsNaluType - is specific NaluType present in sample
func ContainsNaluType(sample []byte, specificNalType NaluType) bool {
	var pos uint32 = 0
	length := len(sample)
	for pos < uint32(length-4) {
		naluLength := binary.BigEndian.Uint32(sample[pos : pos+4])
		pos += 4
		naluType := NaluType(sample[pos] & 0x1f)
		if naluType == specificNalType {
			return true
		}
		pos += naluLength
	}
	return false
}

// HasParameterSets - Check if H.264 SPS and PPS are present
func HasParameterSets(b []byte) bool {
	naluTypeList := FindNaluTypes(b)
	hasSPS := false
	hasPPS := false
	for _, naluType := range naluTypeList {
		if naluType == NALU_SPS {
			hasSPS = true
		}
		if naluType == NALU_PPS {
			hasPPS = true
		}
		if hasSPS && hasPPS {
			return true
		}
	}
	return false
}

// GetParameterSets - get (multiple) SPS and PPS from a sample
func GetParameterSets(sample []byte) (sps [][]byte, pps [][]byte) {
	sampleLength := uint32(len(sample))
	var pos uint32 = 0
	for {
		if pos >= sampleLength {
			break
		}
		naluLength := binary.BigEndian.Uint32(sample[pos : pos+4])
		pos += 4
		naluHdr := sample[pos]
		switch GetNaluType(naluHdr) {
		case NALU_SPS:
			sps = append(sps, sample[pos:pos+naluLength])
		case NALU_PPS:
			pps = append(pps, sample[pos:pos+naluLength])
		}
		pos += naluLength
	}
	return sps, pps
}
