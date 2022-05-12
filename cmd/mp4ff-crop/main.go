/*
mp4ff-crop crops a (progressive) mp4 file to just before a sync frame after specified number of milliseconds.
The intension is that the structure of the file shall be left intact except for cropping of samples and
moving mdat to the end of the file, if not already there.
*/
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/edgeware/mp4ff/mp4"
)

var usg = `Usage of %s:

%s crops a (progressive) mp4 file to just before a sync frame after specified number of milliseconds.
The goal is to leave the file structure intact except for cropping of samples and
moving mdat to the end of the file, if not already there.
`

var opts struct {
	durationMS int
	version    bool
}

func parseOptions() {
	flag.IntVar(&opts.durationMS, "d", 0, "Duration in milliseconds")
	flag.BoolVar(&opts.version, "version", false, "Get mp4ff version")
	flag.Parse()

	flag.Usage = func() {
		parts := strings.Split(os.Args[0], "/")
		name := parts[len(parts)-1]
		fmt.Fprintf(os.Stderr, usg, name, name)
		fmt.Fprintf(os.Stderr, "%s [-d duration] <inFile> <outFile>\n", name)
		flag.PrintDefaults()
	}

	flag.Parse()
}

func main() {
	parseOptions()

	if opts.version {
		fmt.Printf("mp4ff-crop %s\n", mp4.GetVersion())
		os.Exit(0)
	}

	if opts.durationMS <= 0 {
		fmt.Printf("error: duration must be larger than 0\n\n")
		flag.Usage()
		os.Exit(1)
	}

	var inFilePath = flag.Arg(0)
	if inFilePath == "" {
		fmt.Printf("error: no infile path specified\n\n")
		flag.Usage()
		os.Exit(1)
	}

	var outFilePath = flag.Arg(1)
	if outFilePath == "" {
		flag.Usage()
		os.Exit(1)
	}

	ifh, err := os.Open(inFilePath)
	if err != nil {
		log.Fatalln(err)
	}
	defer ifh.Close()
	parsedMp4, err := mp4.DecodeFile(ifh, mp4.WithDecodeMode(mp4.DecModeLazyMdat))
	if err != nil {
		log.Fatal(err)
	}

	ofh, err := os.Create(outFilePath)
	if err != nil {
		log.Fatalln(err)
	}
	defer ofh.Close()

	err = cropMP4(parsedMp4, opts.durationMS, ofh, ifh)
	if err != nil {
		log.Fatal(err)
	}

}

func cropMP4(inMP4 *mp4.File, durationMS int, w io.Writer, ifh io.ReadSeeker) error {
	if inMP4.IsFragmented() {
		return fmt.Errorf("only progressive files are supported")
	}
	inMoov := inMP4.Moov
	inMoovDur := float64(inMoov.Mvhd.Duration) / float64(inMoov.Mvhd.Timescale)
	fmt.Printf("input moov duration = %.3fs\n", inMoovDur)

	endTime, endTimescale, err := findEndTime(inMoov, durationMS)
	if err != nil {
		return err
	}

	err = cropToTime(inMP4, endTime, endTimescale, w, ifh)
	if err != nil {
		return err
	}
	fmt.Printf("wrote output with endTime=%dms\n", (endTime*1000)/endTimescale)
	return nil

}

// findEndTime - find closest video sync frame, or audio frame if no video
func findEndTime(moov *mp4.MoovBox, durationMS int) (endTime, endTimescale uint64, err error) {
	var syncTrak *mp4.TrakBox
	for _, trak := range moov.Traks {
		if trak.Mdia.Hdlr.HandlerType == "vide" {
			syncTrak = trak
			break
		}
	}
	if syncTrak == nil {
		for _, trak := range moov.Traks {
			if trak.Mdia.Hdlr.HandlerType == "soun" {
				syncTrak = trak
				fmt.Printf("Uses audio track %d for endtime\n", trak.Tkhd.TrackID)
				break
			}
		}
	}
	if syncTrak == nil {
		return 0, 0, fmt.Errorf("did not find any video or audio track")
	}

	//trakDur := float64(trak.Tkhd.Duration) / float64(moov.Mvhd.Timescale)
	//fmt.Printf("video trak %d duration = %.3fs\n", trak.Tkhd.TrackID, trakDur)
	endTimescale = uint64(syncTrak.Mdia.Mdhd.Timescale)
	endTime = uint64(durationMS) * endTimescale / 1000

	stbl := syncTrak.Mdia.Minf.Stbl
	stts := stbl.Stts // TimeToSampleBox
	lastSampleNr, err := stts.GetSampleNrAtTime(endTime)
	if err != nil {
		return 0, 0, err
	}
	stss := stbl.Stss
	if stss != nil {
		foundSyncFrame := false
		for sampleNr := lastSampleNr; sampleNr <= stss.SampleNumber[len(stss.SampleNumber)-1]; sampleNr++ {
			if stss.IsSyncSample(sampleNr) {
				lastSampleNr = sampleNr - 1
				foundSyncFrame = true
				break
			}
		}
		if !foundSyncFrame {
			return 0, 0, fmt.Errorf("did not find any syncframe at or after time")
		}
	}
	lastTime, lastDur := stts.GetDecodeTime(lastSampleNr)
	endTime = lastTime + uint64(lastDur)

	return endTime, endTimescale, nil
}

func cropToTime(inMP4 *mp4.File, endTime, endTimescale uint64, w io.Writer, ifh io.ReadSeeker) error {
	traks := inMP4.Moov.Traks
	tos, err := findTrakEnds(traks, endTime, endTimescale)
	if err != nil {
		return err
	}
	byteRanges := createByteRanges()
	firstOffset, err := fillTrakOutsAndByteRanges(traks, tos, byteRanges)
	if err != nil {
		return err
	}

	cropStblChildren(traks, tos)
	updateChunkOffsets(inMP4, firstOffset)

	err = writeUptoMdat(inMP4, endTime, endTimescale, w)
	if err != nil {
		return err
	}

	err = writeMdat(byteRanges, inMP4.Mdat, w, ifh)
	if err != nil {
		return err
	}

	return nil
}

type trakOut struct {
	lastSampleNr  uint32
	endTime       uint64
	lastChunk     mp4.Chunk
	nextInChunkNr uint32
	chunkOffsets  []uint64
}

// findTrakEnds - find where traks end in form of last chunk, lastSampleNr and endTime
func findTrakEnds(traks []*mp4.TrakBox, endTime, endTimescale uint64) (map[uint32]*trakOut, error) {
	tos := make(map[uint32]*trakOut, len(traks))
	for _, trak := range traks {
		trackID := trak.Tkhd.TrackID
		stbl := trak.Mdia.Minf.Stbl
		tos[trackID] = &trakOut{
			nextInChunkNr: 1,
		}
		to := tos[trackID]
		trackTimeScale := trak.Mdia.Mdhd.Timescale
		trackEndTime := endTime
		if trackTimeScale != uint32(endTimescale) {
			trackEndTime = endTime * uint64(trackTimeScale) / endTimescale
		}
		stts := stbl.Stts
		endSampleNr, err := stts.GetSampleNrAtTime(trackEndTime)
		if err != nil {
			return nil, err
		}
		endSampleNr--
		to.lastSampleNr = endSampleNr
		decTime, dur := stts.GetDecodeTime(endSampleNr)
		trackEndTime = decTime + uint64(dur)
		tos[trackID].endTime = trackEndTime
		stsc := stbl.Stsc
		chunkNr, _, err := stsc.ChunkNrFromSampleNr(int(endSampleNr))
		if err != nil {
			return nil, err
		}
		chunk := stsc.GetChunk(uint32(chunkNr))
		to.lastChunk = chunk
	}
	return tos, nil
}

func fillTrakOutsAndByteRanges(traks []*mp4.TrakBox, tos map[uint32]*trakOut, byteRanges *byteRanges) (firstOffset uint64, err error) {
	var currentOutOffset uint64
	var minChunkOffset uint64
	for {
		var trakIDMin uint32
		var stblMin *mp4.StblBox
		minChunkOffset = 1 << 62
		for _, trak := range traks {
			trakID := trak.Tkhd.TrackID
			stbl := trak.Mdia.Minf.Stbl
			to := tos[trakID]
			nextChunkNr := int(to.nextInChunkNr)
			if nextChunkNr > int(to.lastChunk.ChunkNr) {
				continue
			}
			var chunkOffset uint64
			var err error
			if stbl.Stco != nil {
				chunkOffset, err = stbl.Stco.GetOffset(nextChunkNr)
			} else {
				chunkOffset, err = stbl.Co64.GetOffset(nextChunkNr)
			}
			if err != nil {
				return 0, err
			}
			if chunkOffset < minChunkOffset {
				minChunkOffset = chunkOffset
				trakIDMin = trakID
				stblMin = stbl
			}
		}
		if trakIDMin == 0 {
			break //Done
		}
		if firstOffset == 0 {
			firstOffset = minChunkOffset
			currentOutOffset = firstOffset
		}
		to := tos[trakIDMin]
		chunk := stblMin.Stsc.GetChunk(to.nextInChunkNr)
		lastSampleInChunk := chunk.StartSampleNr + chunk.NrSamples - 1
		sampleNrStart := chunk.StartSampleNr
		sampleNrEnd := minUint32(lastSampleInChunk, to.lastSampleNr)
		inOffset := minChunkOffset
		outChunkSize, _ := stblMin.Stsz.GetTotalSampleSize(sampleNrStart, sampleNrEnd)
		byteRanges.addRange(inOffset, inOffset+outChunkSize-1)
		to.chunkOffsets = append(to.chunkOffsets, currentOutOffset)
		currentOutOffset += outChunkSize
		to.nextInChunkNr++
	}
	return firstOffset, nil
}

// updateChunkOffsets - calculate new moov size, and update stco/co64 (chunk offsets)
func updateChunkOffsets(inMP4 *mp4.File, firstOffset uint64) {
	var sizeWithoutMdat uint64 = 0
	for _, box := range inMP4.Children {
		if box.Type() != "mdat" {
			sizeWithoutMdat += box.Size()
		}
	}
	mdatStart := sizeWithoutMdat
	mdatPayloadStart := mdatStart + 8
	deltaOffset := int64(mdatPayloadStart) - int64(firstOffset)
	for _, trak := range inMP4.Moov.Traks {
		stco := trak.Mdia.Minf.Stbl.Stco
		if stco != nil {
			for i := range stco.ChunkOffset {
				stco.ChunkOffset[i] = uint32(int64(stco.ChunkOffset[i]) + deltaOffset)
			}
		} else {
			co64 := trak.Mdia.Minf.Stbl.Co64
			for i := range co64.ChunkOffset {
				co64.ChunkOffset[i] = uint64(int64(co64.ChunkOffset[i]) + deltaOffset)
			}
		}
	}
}

func writeUptoMdat(inMP4 *mp4.File, endTime, endTimescale uint64, w io.Writer) error {
	pos := uint64(0)
	mvhd := inMP4.Moov.Mvhd
	duration := endTime * uint64(mvhd.Timescale) / endTimescale
	mvhd.Duration = duration
	for _, trak := range inMP4.Moov.Traks {
		trak.Tkhd.Duration = duration
	}
	for _, box := range inMP4.Children {
		if box.Type() != "mdat" {
			pos += box.Size()
			err := box.Encode(w)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func writeMdat(byteRanges *byteRanges, mdatIn *mp4.MdatBox, w io.Writer, ifh io.ReadSeeker) error {
	// write mdat header
	mdatPayloadSize := byteRanges.size()
	if mdatPayloadSize+8 >= 1<<32 {
		return fmt.Errorf("too big mdat size for 32 bits: %d", mdatPayloadSize)
	}
	err := mp4.EncodeHeaderWithSize("mdat", mdatPayloadSize+8, false, w)
	if err != nil {
		return err
	}
	// write mdat body
	nrBytesWritten := int64(0)
	for _, br := range byteRanges.ranges {
		n, err := mdatIn.CopyData(int64(br.start), int64(br.end-br.start+1), ifh, w)
		if err != nil {
			return err
		}
		nrBytesWritten += n
	}
	if nrBytesWritten != int64(mdatPayloadSize) {
		return fmt.Errorf("wrote %d instead of %d in mdat", nrBytesWritten, mdatPayloadSize)
	}
	return nil
}

func minUint32(a, b uint32) uint32 {
	if a < b {
		return a
	}
	return b
}
