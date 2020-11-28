package avc

import (
	"encoding/binary"
	"fmt"
)

// NalType - AVC nal type
type NalType uint16

const (
	// NALU_NON_IDR - Non-IDR Slice NAL unit
	NALU_NON_IDR = NalType(1)
	// NALU_IDR - IDR Random Access Slice NAL Unit
	NALU_IDR = NalType(5)
	// NALU_SEI - Supplementary Enhancement Information NAL Unit
	NALU_SEI = NalType(6)
	// NALU_SSP - SequenceParameterSet NAL Unit
	NALU_SPS = NalType(7)
	// NALU_PPS - PictureParameterSet NAL Unit
	NALU_PPS = NalType(8)
	// NALU_AUD - AccessUnitDelimiter NAL Unit
	NALU_AUD = NalType(9)
	// NALU_EO_SEQ - End of Sequence NAL Unit
	NALU_EO_SEQ = NalType(10)
	// NALU_EO_STREAM - End of Stream NAL Unit
	NALU_EO_STREAM = NalType(11)
	// NALU_FILL - Filler NAL Unit
	NALU_FILL = NalType(12)
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
		return fmt.Sprintf("Other %d", a)
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

// GetParameterSets - get SPS and (multipled) PPS from a sample
func GetParameterSets(sample []byte) (sps []byte, pps [][]byte) {
	sampleLength := uint32(len(sample))
	var pos uint32 = 0
	for {
		if pos >= sampleLength {
			break
		}
		nalLength := binary.BigEndian.Uint32(sample[pos : pos+4])
		pos += 4
		nalHdr := sample[pos]
		switch GetNalType(nalHdr) {
		case NALU_SPS:
			sps = sample[pos : pos+nalLength]
		case NALU_PPS:
			pps = append(pps, sample[pos:pos+nalLength])
		}
		pos += nalLength
	}
	return sps, pps
}
