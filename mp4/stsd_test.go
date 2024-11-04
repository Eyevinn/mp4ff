package mp4

import (
	"bytes"
	"testing"

	"github.com/Eyevinn/mp4ff/aac"
)

func TestStsd(t *testing.T) {
	stsd := StsdBox{}
	samplingFrequency := 48000
	asc := &aac.AudioSpecificConfig{
		ObjectType:           2,
		ChannelConfiguration: 2,
		SamplingFrequency:    samplingFrequency,
		ExtensionFrequency:   0,
		SBRPresentFlag:       false,
		PSPresentFlag:        false,
	}
	buf := &bytes.Buffer{}
	err := asc.Encode(buf)
	if err != nil {
		t.Error(err)
	}
	ascBytes := buf.Bytes()
	esds := CreateEsdsBox(ascBytes)
	mp4a := CreateAudioSampleEntryBox("mp4a",
		uint16(asc.ChannelConfiguration),
		16, uint16(samplingFrequency), esds)
	btrt := BtrtBox{
		BufferSizeDB: 1536,
		MaxBitrate:   96000,
		AvgBitrate:   96000,
	}
	mp4a.AddChild(&btrt)
	stsd.AddChild(mp4a)
	if len(stsd.Children) != 1 {
		t.Error("Expected one child")
	}
	if stsd.Mp4a == nil {
		t.Error("Expected mp4a child")
	}
	gb := stsd.GetBtrt()
	if gb == nil {
		t.Error("Expected btrt")
	} else {
		if *gb != btrt {
			t.Errorf("Got btrt %v, expected %v", *gb, btrt)
		}
	}
}
