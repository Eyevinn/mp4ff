package mp4

import "encoding/binary"

const (
	AvcNalSEI = 6
	AvcNalSPS = 7
	AvcNalPPS = 8
	AvcNalAUD = 9
)

// AvcNalType -
type AvcNalType uint16

func isVideoNalu(b []byte) bool {
	typ := b[0] & 0x1f
	return 1 <= typ && typ <= 5
}

// FindAvcNalTypes - find list of nal types
func FindAvcNalTypes(b []byte) []AvcNalType {
	var pos uint32 = 0
	nalList := make([]AvcNalType, 0)
	length := len(b)
	if length < 4 {
		return nalList
	}
	for pos < uint32(length-4) {
		nalLength := binary.BigEndian.Uint32(b[pos : pos+4])
		pos += 4
		nalType := AvcNalType(b[pos] & 0x1f)
		nalList = append(nalList, nalType)
		pos += nalLength
	}
	return nalList
}

// HasAvcParameterSets - Check if H.264 SPS and PPS are present
func HasAvcParameterSets(b []byte) bool {
	nalTypeList := FindAvcNalTypes(b)
	hasSPS := false
	hasPPS := false
	for _, nalType := range nalTypeList {
		if nalType == AvcNalSPS {
			hasSPS = true
		}
		if nalType == AvcNalPPS {
			hasPPS = true
		}
		if hasSPS && hasPPS {
			return true
		}
	}
	return false
}
