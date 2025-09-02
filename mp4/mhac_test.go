package mp4_test

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

func TestEncodeDecode_MhaC(t *testing.T) {
	t.Run("mhaC-basic", func(t *testing.T) {
		mhaC := &mp4.MhaCBox{
			MHADecoderConfigRecord: mp4.MHADecoderConfigurationRecord{
				ConfigVersion:                  1,
				MpegH3DAProfileLevelIndication: 2,
				ReferenceChannelLayout:         3,
				MpegH3DAConfigLength:           4,
				MpegH3DAConfig:                 []byte{0x01, 0x02, 0x03, 0x04},
			},
		}
		boxDiffAfterEncodeAndDecode(t, mhaC)
	})

	t.Run("mhaC-empty-config", func(t *testing.T) {
		mhaC := &mp4.MhaCBox{
			MHADecoderConfigRecord: mp4.MHADecoderConfigurationRecord{
				ConfigVersion:                  0,
				MpegH3DAProfileLevelIndication: 1,
				ReferenceChannelLayout:         2,
				MpegH3DAConfigLength:           0,
				MpegH3DAConfig:                 []byte{},
			},
		}
		boxDiffAfterEncodeAndDecode(t, mhaC)
	})

	t.Run("mhaC-larger-config", func(t *testing.T) {
		configData := make([]byte, 1000)
		for i := 0; i < 1000; i++ {
			configData[i] = byte(i + 1)
		}
		mhaC := &mp4.MhaCBox{
			MHADecoderConfigRecord: mp4.MHADecoderConfigurationRecord{
				ConfigVersion:                  255,
				MpegH3DAProfileLevelIndication: 128,
				ReferenceChannelLayout:         64,
				MpegH3DAConfigLength:           1000,
				MpegH3DAConfig:                 configData,
			},
		}
		boxDiffAfterEncodeAndDecode(t, mhaC)
	})
}

func TestDecodeMhaC_FromHex(t *testing.T) {
	// Example mhaC box with basic configuration
	// Size: 17 bytes, Type: "mhaC", ConfigVersion: 1, ProfileLevel: 2, ChannelLayout: 3, ConfigLength: 4, Config: [1,2,3,4]
	mhaCHex := "00000011" + "6d686143" + "01" + "02" + "03" + "0004" + "01020304"
	mhaCBytes, err := hex.DecodeString(mhaCHex)
	if err != nil {
		t.Fatal(err)
	}

	sr := bits.NewFixedSliceReader(mhaCBytes)
	box, err := mp4.DecodeBoxSR(0, sr)
	if err != nil {
		t.Fatal(err)
	}

	mhaC := box.(*mp4.MhaCBox)
	if mhaC.Type() != "mhaC" {
		t.Errorf("expected box type 'mhaC', got '%s'", mhaC.Type())
	}

	record := mhaC.MHADecoderConfigRecord
	if record.ConfigVersion != 1 {
		t.Errorf("expected ConfigVersion 1, got %d", record.ConfigVersion)
	}
	if record.MpegH3DAProfileLevelIndication != 2 {
		t.Errorf("expected MpegH3DAProfileLevelIndication 2, got %d", record.MpegH3DAProfileLevelIndication)
	}
	if record.ReferenceChannelLayout != 3 {
		t.Errorf("expected ReferenceChannelLayout 3, got %d", record.ReferenceChannelLayout)
	}
	if record.MpegH3DAConfigLength != 4 {
		t.Errorf("expected MpegH3DAConfigLength 4, got %d", record.MpegH3DAConfigLength)
	}
	if len(record.MpegH3DAConfig) != 4 {
		t.Errorf("expected config data length 4, got %d", len(record.MpegH3DAConfig))
	}
	expectedConfig := []byte{1, 2, 3, 4}
	for i, b := range record.MpegH3DAConfig {
		if b != expectedConfig[i] {
			t.Errorf("expected config data[%d] = %d, got %d", i, expectedConfig[i], b)
		}
	}
	buf := bytes.Buffer{}
	err = mhaC.Info(&buf, "mhaC:1", "", "  ")
	if err != nil {
		t.Errorf("unexpected error from Info: %v", err)
	}
	got := buf.String()
	want := ("[mhaC] size=17\n - configVersion=1\n - mpegH3DAProfileLevelIndication=2\n" +
		" - referenceChannelLayout=3\n - mpegH3DAConfigLength=4\n   - mpegH3DAConfig=01020304\n")
	if got != want {
		t.Errorf("expected info %q, got %q", want, got)
	}
}
