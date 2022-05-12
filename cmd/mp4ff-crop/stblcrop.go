package main

import (
	"github.com/edgeware/mp4ff/mp4"
)

func cropStblChildren(traks []*mp4.TrakBox, trakOuts map[uint32]*trakOut) {
	for _, trak := range traks {
		trakID := trak.Tkhd.TrackID
		stbl := trak.Mdia.Minf.Stbl
		to := trakOuts[trakID]
		for _, ch := range stbl.Children {
			switch ch.Type() {
			case "stts":
				cropStts(ch.(*mp4.SttsBox), to.lastSampleNr)
			case "stss":
				cropStss(ch.(*mp4.StssBox), to.lastSampleNr)
			case "ctts":
				cropCtts(ch.(*mp4.CttsBox), to.lastSampleNr)
			case "stsc":
				cropStsc(ch.(*mp4.StscBox), to.lastSampleNr)
			case "stsz":
				cropStsz(ch.(*mp4.StszBox), to.lastSampleNr)
			case "sdtp":
				cropSdtp(ch.(*mp4.SdtpBox), to.lastSampleNr)
			case "stco":
				updateStco(ch.(*mp4.StcoBox), to.chunkOffsets)
			case "co64":
				updateCo64(ch.(*mp4.Co64Box), to.chunkOffsets)
			}
		}
	}
}

func cropStts(b *mp4.SttsBox, lastSampleNr uint32) {
	var countedSamples uint32 = 0
	var nrEntries int
	for i := 0; i < len(b.SampleCount); i++ {
		if countedSamples+b.SampleCount[i] >= lastSampleNr {
			countedSamples += b.SampleCount[i]
			nrEntries = i + 1
			continue
		}
		// Now 0 or more remains in a last entry
		nrEntries = i
		remaining := lastSampleNr - countedSamples
		if remaining > 0 {
			b.SampleCount[i] = remaining
			nrEntries = i + 1
		}
		break
	}
	b.SampleCount = b.SampleCount[:nrEntries]
	b.SampleTimeDelta = b.SampleTimeDelta[:nrEntries]
}

func cropStss(b *mp4.StssBox, lastSampleNr uint32) {
	nrEntries := b.EntryCount()
	nrEntriesToKeep := 0
	for i := uint32(0); i < nrEntries; i++ {
		if b.SampleNumber[i] > lastSampleNr {
			break
		}
		nrEntriesToKeep++
	}
	b.SampleNumber = b.SampleNumber[:nrEntriesToKeep]
}

func cropCtts(b *mp4.CttsBox, lastSampleNr uint32) {
	var countedSamples uint32 = 0
	var nrEntries int
	for i := 0; i < len(b.SampleCount); i++ {
		if countedSamples+b.SampleCount[i] <= lastSampleNr {
			countedSamples += b.SampleCount[i]
			nrEntries = i + 1
			continue
		}
		// Now 0 or more remains in a last entry
		remaining := lastSampleNr - countedSamples
		if remaining > 0 {
			b.SampleCount[i] = remaining
			nrEntries = i + 1
		}
		break
	}
	b.SampleCount = b.SampleCount[:nrEntries]
	b.SampleOffset = b.SampleOffset[:nrEntries]
}

func cropStsc(b *mp4.StscBox, lastSampleNr uint32) {
	var countedSamples uint32 = 0
	nrEntries := len(b.FirstChunk)
	nrFullEntries := 0
	nextChunkNr := 1
	for i := 0; i < nrEntries-1; i++ {
		nrChunksOfLength := b.FirstChunk[i+1] - b.FirstChunk[i]
		nrSamplesInEntry := nrChunksOfLength * b.SamplesPerChunk[i]
		if countedSamples+nrSamplesInEntry < lastSampleNr {
			countedSamples += nrSamplesInEntry
			nrFullEntries++
			nextChunkNr += int(nrChunksOfLength)
			continue
		}
		break
	}
	samplesLeft := lastSampleNr - countedSamples
	if samplesLeft == 0 {
		if nrFullEntries < nrEntries-1 {
			b.FirstChunk = b.FirstChunk[:nrFullEntries]
			b.SamplesPerChunk = b.SamplesPerChunk[:nrFullEntries]
		}
	} else {
		partialSizeLeft := samplesLeft % b.SamplesPerChunk[nrFullEntries]
		nrChunksOfLastSize := samplesLeft / b.SamplesPerChunk[nrFullEntries]
		if nrChunksOfLastSize >= 1 {
			b.FirstChunk = b.FirstChunk[:nrFullEntries+1]
			b.SamplesPerChunk = b.SamplesPerChunk[:nrFullEntries+1]
		} else {
			b.FirstChunk = b.FirstChunk[:nrFullEntries]
			b.SamplesPerChunk = b.SamplesPerChunk[:nrFullEntries]
		}
		nextChunkNr += int(nrChunksOfLastSize)
		if partialSizeLeft > 0 {
			b.FirstChunk = append(b.FirstChunk, uint32(nextChunkNr))
			b.SamplesPerChunk = append(b.SamplesPerChunk, partialSizeLeft)
		}
	}
	if len(b.SampleDescriptionID) > 0 {
		b.SampleDescriptionID = b.SampleDescriptionID[:len(b.FirstChunk)]
	}
}

func cropStsz(b *mp4.StszBox, lastSampleNr uint32) {
	if b.SampleUniformSize == 0 {
		b.SampleSize = b.SampleSize[:lastSampleNr]
	}
	b.SampleNumber = lastSampleNr
}

func cropSdtp(b *mp4.SdtpBox, lastSampleNr uint32) {
	if len(b.Entries) > int(lastSampleNr) {
		b.Entries = b.Entries[:lastSampleNr]
	}
}

func updateStco(b *mp4.StcoBox, offsets []uint64) {
	b.ChunkOffset = make([]uint32, len(offsets))
	for i := range offsets {
		b.ChunkOffset[i] = uint32(offsets[i])
	}
}

func updateCo64(b *mp4.Co64Box, offsets []uint64) {
	b.ChunkOffset = make([]uint64, len(offsets))
	_ = copy(b.ChunkOffset, offsets)
}
