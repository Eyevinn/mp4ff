package avc

import (
	"encoding/binary"
)

// NalType - AVC nal type
type NalType uint16

const (
	// NALU_IDR - IDR Random Access Picture NAL Unit
	NALU_IDR = NalType(5)
	// NALU_SEI - Supplementary Enhancement Information NAL Unit
	NALU_SEI = NalType(6)
	// NALU_SSP - SequenceParameterSet NAL Unit
	NALU_SPS = NalType(7)
	// NALU_PPS - PictureParameterSet NAL Unit
	NALU_PPS = NalType(8)
	// NALU_AUD - AccessUnitDelimiter NAL Unit
	NALU_AUD = NalType(9)
	// NALU_FILL - Filler NAL Unit
	NALU_FILL = NalType(12)
	// ExtendedSAR - Extended Sample Aspect Ratio Code
	ExtendedSAR = 255
)

func (a NalType) String() string {
	switch a {
	case NALU_IDR:
		return "IDR"
	case NALU_SEI:
		return "SEI"
	case NALU_SPS:
		return "SPS"
	case NALU_PPS:
		return "PPS"
	case NALU_AUD:
		return "AUD"
	default:
		return "other"
	}
}

// Get NalType from NAL Header byte
func GetNalType(nalHeader byte) NalType {
	return NalType(nalHeader & 0x1f)
}

// FindNalTypes - find list of nal types in sample
func FindNalTypes(sample []byte) []NalType {
	nalList := make([]NalType, 0)
	length := len(sample)
	if length < 4 {
		return nalList
	}
	var pos uint32 = 0
	for pos < uint32(length-4) {
		nalLength := binary.BigEndian.Uint32(sample[pos : pos+4])
		pos += 4
		nalType := NalType(sample[pos] & 0x1f)
		nalList = append(nalList, nalType)
		pos += nalLength
	}
	return nalList
}

// IsIDRSample - does sample contain IDR NALU
func IsIDRSample(sample []byte) bool {
	return ContainsNalType(sample, NALU_IDR)
}

// ContainsNalType - is specificNalType present in sample
func ContainsNalType(sample []byte, specificNalType NalType) bool {
	var pos uint32 = 0
	length := len(sample)
	for pos < uint32(length-4) {
		nalLength := binary.BigEndian.Uint32(sample[pos : pos+4])
		pos += 4
		nalType := NalType(sample[pos] & 0x1f)
		if nalType == specificNalType {
			return true
		}
		pos += nalLength
	}
	return false
}

// HasParameterSets - Check if H.264 SPS and PPS are present
func HasParameterSets(b []byte) bool {
	nalTypeList := FindNalTypes(b)
	hasSPS := false
	hasPPS := false
	for _, nalType := range nalTypeList {
		if nalType == NALU_SPS {
			hasSPS = true
		}
		if nalType == NALU_PPS {
			hasPPS = true
		}
		if hasSPS && hasPPS {
			return true
		}
	}
	return false
}
