package mp4_test

import (
	"bytes"
	"encoding/hex"
	"os"
	"testing"

	"github.com/Eyevinn/mp4ff/aac"
	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

func TestStsd(t *testing.T) {
	stsd := mp4.StsdBox{}
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
	esds := mp4.CreateEsdsBox(ascBytes)
	mp4a := mp4.CreateAudioSampleEntryBox("mp4a",
		uint16(asc.ChannelConfiguration),
		16, uint16(samplingFrequency), esds)
	btrt := mp4.BtrtBox{
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

func TestStsdEncodeDecode(t *testing.T) {
	stsd := &mp4.StsdBox{}
	evte := &mp4.EvteBox{}
	stsd.AddChild(evte)
	boxDiffAfterEncodeAndDecode(t, stsd)
	b, err := stsd.GetSampleDescription(0)
	if err != nil {
		t.Error(err)
	}
	if b != evte {
		t.Errorf("Expected %v, got %v", evte, b)
	}
	b, err = stsd.GetSampleDescription(1)
	if err == nil {
		t.Errorf("Expected error, got %v", b)
	}
	btrt := stsd.GetBtrt()
	if btrt != nil {
		t.Errorf("Expected nil, got %v", btrt)
	}
}

func TestStsdVP9(t *testing.T) {
	hexData := "" +
		"000000a87374736400000000000000010000009876703039000000000000" +
		"000100000000000000000000000000000000050002d00048000000480000" +
		"000000000001184c61766336312e31392e313030206c69627670782d7670" +
		"39000000000000000018ffff000000147670634301000000001f80020202" +
		"00000000000a6669656c0100000000107061737000000001000000010000" +
		"001462747274000000000010152200101522"

	binData, err := hex.DecodeString(hexData)
	if err != nil {
		t.Error(err)
	}

	cmpAfterDecodeEncodeBox(t, binData)
	// Check that VP9 pointer is set
	stsd := decodeStsdBox(t, binData)
	if stsd.VpXX == nil {
		t.Errorf("Expected VP9 box pointer, got nil")
	}
	if stsd.VpXX.Type() != "vp09" {
		t.Errorf("VpXX type is %s, expected vp09", stsd.VpXX.Type())
	}
}

func TestStsdAC4(t *testing.T) {
	data, err := os.ReadFile("testdata/stsd_ac4.bin")
	if err != nil {
		t.Fatal(err)
	}
	cmpAfterDecodeEncodeBox(t, data)
	// Check that AC4 pointer is set
	stsd := decodeStsdBox(t, data)
	if stsd.AC4 == nil {
		t.Errorf("Expected AC4 box pointer, got nil")
	}
	if stsd.AC4.Type() != "ac-4" {
		t.Errorf("AC4 type is %s, expected ac-4", stsd.AC4.Type())
	}
}

func decodeStsdBox(t *testing.T, data []byte) *mp4.StsdBox {
	t.Helper()
	sr := bits.NewFixedSliceReader(data)
	box, err := mp4.DecodeBoxSR(0, sr)
	if err != nil {
		t.Fatal(err)
	}
	stsd, ok := box.(*mp4.StsdBox)
	if !ok {
		t.Fatalf("Expected StsdBox, got %T", box)
	}
	return stsd
}

// TestSdsdMha1Decode checks that mha1 is recognized as AudioSampleEntry
func TestStsdMha1Decode(t *testing.T) {
	data, err := os.ReadFile("testdata/stsd_mha1.bin")
	if err != nil {
		t.Fatal(err)
	}
	stsd := decodeStsdBox(t, data)
	if len(stsd.Children) != 1 {
		t.Errorf("Expected one child, got %d", len(stsd.Children))
	}
	_, ok := stsd.Children[0].(*mp4.AudioSampleEntryBox)
	if !ok {
		t.Errorf("Expected AudioSampleEntryBox, got %T", stsd.Children[0])
	}
	if stsd.MhXX == nil {
		t.Errorf("Expected MHA1 box pointer, got nil")
	}
	mha1 := stsd.MhXX
	if len(mha1.Children) != 2 {
		t.Errorf("Expected two children, got %d", len(mha1.Children))
	}
	if mha1.Children[0].Type() != "mhaC" {
		t.Errorf("Expected MHA1 first child to be mhaC, got %s", mha1.Children[0].Type())
	}
	// Validate that the mhaC box is properly decoded as MhaCBox
	mhaC, ok := mha1.Children[0].(*mp4.MhaCBox)
	if !ok {
		t.Errorf("Expected MhaCBox, got %T", mha1.Children[0])
	} else {
		// Basic validation of MHA decoder config record
		if mhaC.MHADecoderConfigRecord.ConfigVersion != 1 ||
			mhaC.MHADecoderConfigRecord.MpegH3DAProfileLevelIndication != 12 ||
			mhaC.MHADecoderConfigRecord.ReferenceChannelLayout != 6 ||
			mhaC.MHADecoderConfigRecord.MpegH3DAConfigLength != 63 {
			t.Errorf("MHA decoder config value does not match. Got: %+v", mhaC.MHADecoderConfigRecord)
		}
	}
	if mha1.MhaC == nil {
		t.Errorf("Expected MHA1 box pointer, got nil")
	}
	if mha1.Children[1].Type() != "btrt" {
		t.Errorf("Expected MHA1 second child to be btrt, got %s", mha1.Children[1].Type())
	}

	cmpAfterDecodeEncodeBox(t, data)
}

func TestStsdAVS3Decode(t *testing.T) {
	data, err := os.ReadFile("testdata/stsd_avs3.bin")
	if err != nil {
		t.Fatal(err)
	}
	stsd := decodeStsdBox(t, data)
	if len(stsd.Children) != 1 {
		t.Errorf("Expected one child, got %d", len(stsd.Children))
	}
	_, ok := stsd.Children[0].(*mp4.VisualSampleEntryBox)
	if !ok {
		t.Errorf("Expected VisualSampleEntryBox, got %T", stsd.Children[0])
	}
	if stsd.Avs3 == nil {
		t.Errorf("Expected AVS3 visual sample entry box pointer, got nil")
	}
	avs3 := stsd.Avs3
	if avs3.Type() != "avs3" {
		t.Errorf("Expected avs3 type, got %s", avs3.Type())
	}
	if len(avs3.Children) != 3 { // Should have av3c, btrt, and colr boxes
		t.Errorf("Expected three children (av3c, btrt, colr), got %d", len(avs3.Children))
	}

	// Check that av3c box is properly parsed
	if avs3.Av3c == nil {
		t.Errorf("Expected av3c box pointer, got nil")
	} else {
		// Basic validation of AVS3 decoder config record
		if avs3.Av3c.Avs3Config.ConfigurationVersion != 1 {
			t.Errorf("Expected configuration version 1, got %d", avs3.Av3c.Avs3Config.ConfigurationVersion)
		}
		if avs3.Av3c.Avs3Config.SequenceHeaderLength != 221 { // 0xdd from hex dump
			t.Errorf("Expected sequence header length 221, got %d", avs3.Av3c.Avs3Config.SequenceHeaderLength)
		}
		if len(avs3.Av3c.Avs3Config.SequenceHeader) != 221 {
			t.Errorf("Expected sequence header length 221, got %d", len(avs3.Av3c.Avs3Config.SequenceHeader))
		}
		// LibraryDependencyIDC should be extracted from the last byte (which should be 0xFC | value)
		if avs3.Av3c.Avs3Config.LibraryDependencyIDC > 3 {
			t.Errorf("Expected LibraryDependencyIDC 0-3, got %d", avs3.Av3c.Avs3Config.LibraryDependencyIDC)
		}
	}

	// Check that btrt box is properly parsed
	if avs3.Btrt == nil {
		t.Errorf("Expected btrt box pointer, got nil")
	}

	cmpAfterDecodeEncodeBox(t, data)
}
