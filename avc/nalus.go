package avc

import (
	"encoding/binary"
	"fmt"
)

// GetNalusFromSample - get nalus by following 4 byte length fields
func GetNalusFromSample(sample []byte) ([][]byte, error) {
	length := len(sample)
	if length < 4 {
		return nil, fmt.Errorf("less than 4 bytes, No NALUs")
	}
	naluList := make([][]byte, 0, 2)
	var pos uint32 = 0
	for pos+4 < uint32(length) {
		naluLength := binary.BigEndian.Uint32(sample[pos : pos+4])
		pos += 4
		// uint64 arithmetic so that a length field close to 2^32 cannot wrap
		if uint64(pos)+uint64(naluLength) > uint64(length) {
			return nil, fmt.Errorf("NALU length fields are bad. Not video?")
		}
		naluList = append(naluList, sample[pos:pos+naluLength])
		pos += naluLength
	}
	return naluList, nil
}
