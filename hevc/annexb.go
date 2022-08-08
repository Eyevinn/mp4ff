package hevc

// GetParameterSetsFromByteStream gets SPS and PPS nalus from bytestream
func GetParameterSetsFromByteStream(data []byte) (vpss [][]byte, spss [][]byte, ppss [][]byte) {
	n := len(data)
	currNaluStart := -1
	totSize := 0
	for i := 0; i < n-4; i++ {
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
				switch naluType := GetNaluType(data[currNaluStart]); naluType {
				case NALU_VPS:
					vpss = append(vpss, data[currNaluStart:currNaluEnd])
					totSize += currNaluEnd - currNaluStart
				case NALU_SPS:
					spss = append(spss, data[currNaluStart:currNaluEnd])
					totSize += currNaluEnd - currNaluStart
				case NALU_PPS:
					ppss = append(ppss, data[currNaluStart:currNaluEnd])
					totSize += currNaluEnd - currNaluStart
				}
			}
			currNaluStart = i + 3
			nextNaluType := GetNaluType(data[currNaluStart])
			if nextNaluType < 32 { // Video NALU types are below 32
				break
			}
		}
	}
	psData := make([]byte, totSize)
	pos := 0
	for i := range vpss {
		copy(psData[pos:], vpss[i])
		vpss[i] = psData[pos : pos+len(vpss[i])]
		pos += len(vpss[i])
	}
	for i := range spss {
		copy(psData[pos:], spss[i])
		spss[i] = psData[pos : pos+len(spss[i])]
		pos += len(spss[i])
	}
	for i := range ppss {
		copy(psData[pos:], ppss[i])
		ppss[i] = psData[pos : pos+len(ppss[i])]
		pos += len(ppss[i])
	}
	return vpss, spss, ppss
}
