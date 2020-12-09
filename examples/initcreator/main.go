package main

import (
	"encoding/hex"
	"errors"
	"log"
	"os"

	"github.com/edgeware/mp4ff/mp4"
)

const sps1nalu = "67640020accac05005bb0169e0000003002000000c9c4c000432380008647c12401cb1c31380"
const pps1nalu = "68b5df20"

func main() {

	err := writeVideoAVCInitSegment()
	if err != nil {
		log.Fatalln(err)
	}
	err = writeAudioAACInitSegment()
	if err != nil {
		log.Fatalln(err)
	}
}

func writeVideoAVCInitSegment() error {
	sps, _ := hex.DecodeString(sps1nalu)
	spsNALUs := [][]byte{sps}
	pps, _ := hex.DecodeString(pps1nalu)
	ppsNALUs := [][]byte{pps}

	videoTimescale := uint32(180000)
	init := mp4.CreateEmptyMP4Init(videoTimescale, "video", "und")
	trak := init.Moov.Trak[0]
	trak.SetAVCDescriptor("avc1", spsNALUs, ppsNALUs)
	width := trak.Mdia.Minf.Stbl.Stsd.AvcX.Width
	height := trak.Mdia.Minf.Stbl.Stsd.AvcX.Height
	if width != 1280 || height != 720 {
		return errors.New("Did not get right width and height")
	}
	err := writeToFile(init, "video_init.cmfv")
	return err
}

func writeAudioAACInitSegment() error {
	audioTimeScale := 48000
	init := mp4.CreateEmptyMP4Init(uint32(audioTimeScale), "audio", "en")
	trak := init.Moov.Trak[0]
	err := trak.SetAACDescriptor(mp4.AAClc, audioTimeScale)
	if err != nil {
		return err
	}
	err = writeToFile(init, "audio_init.cmfv")
	return err
}

func writeToFile(init *mp4.InitSegment, filePath string) error {
	// Next write to a file
	ofd, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer ofd.Close()
	err = init.Encode(ofd)
	return err
}
