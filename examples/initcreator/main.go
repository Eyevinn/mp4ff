// initcreator - create example init segments for video, audio, and subtitles
package main

import (
	"encoding/hex"
	"fmt"
	"log"
	"os"

	"github.com/edgeware/mp4ff/aac"
	"github.com/edgeware/mp4ff/bits"
	"github.com/edgeware/mp4ff/mp4"
)

const (
	avcSPSnalu  = "67640020accac05005bb0169e0000003002000000c9c4c000432380008647c12401cb1c31380"
	avcPPSnalu  = "68b5df20"
	hevcVPSnalu = "40010c01ffff022000000300b0000003000003007b18b024"
	hevcSPSnalu = "420101022000000300b0000003000003007ba0078200887db6718b92448053888892cf24a69272c9124922dc91aa48fca223ff000100016a02020201"
	hevcPPSnalu = "4401c0252f053240"
)

func main() {

	err := writeVideoAVCInitSegment()
	if err != nil {
		log.Fatalln(err)
	}
	err = writeVideoHEVCInitSegment()
	if err != nil {
		log.Fatalln(err)
	}
	err = writeAudioAACInitSegment()
	if err != nil {
		log.Fatalln(err)
	}
	err = writeAudioAC3InitSegment()
	if err != nil {
		log.Fatalln(err)
	}
	err = writeAudioEC3InitSegment()
	if err != nil {
		log.Fatalln(err)
	}
	err = writeSubtitlesWvttInitSegment()
	if err != nil {
		log.Fatalln(err)
	}
	err = writeSubtitlesStppInitSegment()
	if err != nil {
		log.Fatalln(err)
	}
}

func writeVideoAVCInitSegment() error {
	sps, _ := hex.DecodeString(avcSPSnalu)
	spsNALUs := [][]byte{sps}
	pps, _ := hex.DecodeString(avcPPSnalu)
	ppsNALUs := [][]byte{pps}

	videoTimescale := uint32(180000)
	init := mp4.CreateEmptyInit()
	init.AddEmptyTrack(videoTimescale, "video", "und")
	trak := init.Moov.Trak
	err := trak.SetAVCDescriptor("avc1", spsNALUs, ppsNALUs)
	if err != nil {
		return err
	}
	width := trak.Mdia.Minf.Stbl.Stsd.AvcX.Width
	height := trak.Mdia.Minf.Stbl.Stsd.AvcX.Height
	if width != 1280 || height != 720 {
		return fmt.Errorf("Did get %dx%d instead of 1280x720", width, height)
	}
	err = writeToFile(init, "video_avc_init.cmfv")
	return err
}

func writeVideoHEVCInitSegment() error {
	vps, _ := hex.DecodeString(hevcVPSnalu)
	vpsNALUs := [][]byte{vps}
	sps, _ := hex.DecodeString(hevcSPSnalu)
	spsNALUs := [][]byte{sps}
	pps, _ := hex.DecodeString(hevcPPSnalu)
	ppsNALUs := [][]byte{pps}

	videoTimescale := uint32(180000)
	init := mp4.CreateEmptyInit()
	init.AddEmptyTrack(videoTimescale, "video", "und")
	trak := init.Moov.Trak
	err := trak.SetHEVCDescriptor("hvc1", vpsNALUs, spsNALUs, ppsNALUs)
	if err != nil {
		return err
	}
	width := trak.Mdia.Minf.Stbl.Stsd.HvcX.Width
	height := trak.Mdia.Minf.Stbl.Stsd.HvcX.Height
	if width != 960 || height != 540 {
		return fmt.Errorf("Did get %dx%d instead of 960x540", width, height)
	}
	err = writeToFile(init, "video_hevc_init.cmfv")
	return err
}

func writeAudioAACInitSegment() error {
	audioTimeScale := 48000
	init := mp4.CreateEmptyInit()
	init.AddEmptyTrack(uint32(audioTimeScale), "audio", "en")
	trak := init.Moov.Trak
	err := trak.SetAACDescriptor(aac.AAClc, audioTimeScale)
	if err != nil {
		return err
	}
	err = writeToFile(init, "audio_aac_init.cmfa")
	return err
}

func writeAudioAC3InitSegment() error {
	dac3Hex := "0000000b646163330c3dc0"
	dac3Bytes, err := hex.DecodeString(dac3Hex)
	if err != nil {
		return err
	}
	sr := bits.NewFixedSliceReader(dac3Bytes)
	box, err := mp4.DecodeBoxSR(0, sr)
	if err != nil {
		return err
	}
	dac3 := box.(*mp4.Dac3Box)
	init := mp4.CreateEmptyInit()
	samplingRate := mp4.AC3SampleRates[dac3.FSCod]
	init.AddEmptyTrack(uint32(samplingRate), "audio", "en")
	trak := init.Moov.Trak
	err = trak.SetAC3Descriptor(dac3)
	if err != nil {
		return err
	}
	err = writeToFile(init, "audio_ac3_init.cmfa")
	return err
}

func writeAudioEC3InitSegment() error {
	dec3Hex := "0000000e646563330c00200f0202"
	dec3Bytes, err := hex.DecodeString(dec3Hex)
	if err != nil {
		return err
	}
	sr := bits.NewFixedSliceReader(dec3Bytes)
	box, err := mp4.DecodeBoxSR(0, sr)
	if err != nil {
		return err
	}
	dec3 := box.(*mp4.Dec3Box)
	init := mp4.CreateEmptyInit()
	samplingRate := mp4.AC3SampleRates[dec3.EC3Subs[0].FSCod]
	init.AddEmptyTrack(uint32(samplingRate), "audio", "en")
	trak := init.Moov.Trak
	err = trak.SetEC3Descriptor(dec3)
	if err != nil {
		return err
	}
	err = writeToFile(init, "audio_ec3_init.cmfa")
	return err
}

func writeSubtitlesWvttInitSegment() error {
	subtitleTimescale := 1000
	init := mp4.CreateEmptyInit()
	init.AddEmptyTrack(uint32(subtitleTimescale), "wvtt", "en")
	trak := init.Moov.Trak
	err := trak.SetWvttDescriptor("WEBVTT")
	if err != nil {
		return err
	}
	err = writeToFile(init, "subtitles_wvtt_init.cmft")
	return err
}

func writeSubtitlesStppInitSegment() error {
	subtitleTimescale := 1000
	init := mp4.CreateEmptyInit()
	init.AddEmptyTrack(uint32(subtitleTimescale), "stpp", "en")
	trak := init.Moov.Trak
	schemaLocation := ""
	auxiliaryMimeType := ""
	err := trak.SetStppDescriptor("http://www.w3.org/ns/ttml", schemaLocation, auxiliaryMimeType)
	if err != nil {
		return err
	}
	err = writeToFile(init, "subtitles_stpp_init.cmft")
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
