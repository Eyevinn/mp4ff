package main

import (
	"encoding/hex"
	"os"

	"github.com/edgeware/gomp4/mp4"
	log "github.com/sirupsen/logrus"
)

const sps1nalu = "67640020accac05005bb0169e0000003002000000c9c4c000432380008647c12401cb1c31380"
const pps1nalu = "68b5df20"

func main() {
	spsNALU, _ := hex.DecodeString(sps1nalu)
	pps, _ := hex.DecodeString(pps1nalu)
	ppsNALUs := [][]byte{pps}

	init := mp4.CreateEmptyMP4Init(180000, "video", "und")
	trak := init.Moov.Trak[0]
	trak.SetAVCDescriptor("avc3", spsNALU, ppsNALUs)
	width := trak.Mdia.Minf.Stbl.Stsd.AvcX.Width
	height := trak.Mdia.Minf.Stbl.Stsd.AvcX.Height
	if width != 1280 || height != 720 {
		log.Fatalf("Did not get right width and height")
	}
	// Next write to a file
	ofd, err := os.Create("out_init.cmfv")
	defer ofd.Close()
	if err != nil {
		log.Fatalf("Error creating file")
	}
	init.Encode(ofd)
}
