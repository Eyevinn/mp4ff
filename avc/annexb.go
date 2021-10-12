package avc

//Functions to handle AnnexB Byte stream format"

import "encoding/binary"

// ExtractNalusFromByteStream - extract NALUs without startcode from ByteStream
func ExtractNalusFromByteStream(data []byte) [][]byte {
	currNaluStart := -1
	n := len(data)
	var nalus [][]byte
	for i := 0; i < n-3; i++ {
		if data[i] == 0 && data[i+1] == 0 && data[i+2] == 1 {
			if currNaluStart > 0 {
				currNaluEnd := i
				for j := i - 1; j > currNaluStart; j-- {
					// Remove zeros from end of NAL unit
					if data[j] == 0 {
						currNaluEnd = j
					} else {
						break
					}
				}
				nalus = append(nalus, extractSlice(data, currNaluStart, currNaluEnd))
			}
			currNaluStart = i + 3
		}
	}
	if currNaluStart < 0 {
		return nil
	}
	nalus = append(nalus, extractSlice(data, currNaluStart, n))
	return nalus
}

func extractSlice(data []byte, start, stop int) []byte {
	sl := make([]byte, stop-start)
	_ = copy(sl, data[start:stop])
	return sl
}

type scNalu struct {
	startCodeLength int
	startPos        int
}

// ConvertByteStreamToNaluSample - Change start codes to 4-byte length fields
func ConvertByteStreamToNaluSample(stream []byte) []byte {
	streamLen := len(stream)
	var scNalus []scNalu
	minStartCodeLength := 4
	for i := 0; i < streamLen-3; i++ {
		if stream[i] == 0 && stream[i+1] == 0 && stream[i+2] == 1 {
			startCodeLength := 3
			startPos := i + 3
			if i >= 1 && stream[i-1] == 0 {
				startCodeLength++
			}
			if startCodeLength < minStartCodeLength {
				minStartCodeLength = startCodeLength
			}
			scNalus = append(scNalus, scNalu{startCodeLength, startPos})
		}
	}
	lengthField := make([]byte, 4)
	var naluLength int
	if minStartCodeLength == 4 {
		// In-place replacement of startcodes for length fields
		for i, s := range scNalus {

			if i+1 < len(scNalus) {
				naluLength = scNalus[i+1].startPos - s.startPos - 4
			} else {
				naluLength = len(stream) - scNalus[i].startPos
			}
			binary.BigEndian.PutUint32(lengthField, uint32(naluLength))
			copy(stream[s.startPos-4:s.startPos], lengthField)
		}
		return stream
	}
	// Build new output slice with one extra byte per NALU
	out := make([]byte, 0, streamLen+len(scNalus))
	for i, s := range scNalus {
		if i+1 < len(scNalus) {
			naluLength = scNalus[i+1].startPos - s.startPos - scNalus[i+1].startCodeLength
		} else {
			naluLength = len(stream) - scNalus[i].startPos
		}
		binary.BigEndian.PutUint32(lengthField, uint32(naluLength))
		out = append(out, lengthField...)
		out = append(out, stream[s.startPos:s.startPos+naluLength]...)
	}
	return out
}

// ConvertSampleToByteStream - Replace 4-byte NALU lengths with start codes
func ConvertSampleToByteStream(sample []byte) []byte {
	sampleLength := uint32(len(sample))
	var pos uint32 = 0
	for {
		if pos >= sampleLength {
			break
		}
		naluLength := binary.BigEndian.Uint32(sample[pos : pos+4])
		startCode := []byte{0, 0, 0, 1}
		copy(sample[pos:pos+4], startCode)
		pos += naluLength + 4
	}
	return sample
}
