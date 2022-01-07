package main

type byteRange struct {
	start uint64
	end   uint64 // Included
}

type byteRanges struct {
	ranges []byteRange
}

func createByteRanges() *byteRanges {
	return &byteRanges{}
}

func (b *byteRanges) addRange(start, end uint64) {
	if len(b.ranges) == 0 || b.ranges[len(b.ranges)-1].end+1 != start {
		b.ranges = append(b.ranges, byteRange{start, end})
		return
	}
	b.ranges[len(b.ranges)-1].end = end
}

func (b *byteRanges) size() uint64 {
	var totSize uint64 = 0
	for _, br := range b.ranges {
		totSize += br.end - br.start + 1
	}
	return uint64(totSize)
}
